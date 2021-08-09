package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type BlobStorageLoggingServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceBlobStorageLogging(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&BlobStorageLoggingServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "blobstoragelogging",
			serviceMetadata: sa,
		},
	})
}

func (h *BlobStorageLoggingServiceAttributeHandler) Key() string { return h.key }

func (h *BlobStorageLoggingServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify the Azure Blob Storage endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"account_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique Azure Blob Storage namespace in which your data objects are stored",
		},
		"container": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of the Azure Blob Storage container in which to store logs",
		},
		"sas_token": {
			Type:        schema.TypeString,
			Required:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_AZURE_SHARED_ACCESS_SIGNATURE", ""),
			Description: "The Azure shared access signature providing write access to the blob service objects. Be sure to update your token before it expires or the logging functionality will not work",
			Sensitive:   true,
		},
		// Optional fields
		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The path to upload logs to. Must end with a trailing slash. If this field is left empty, the files will be saved in the container's root path",
		},
		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently the logs should be transferred in seconds. Default `3600`",
		},
		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: "`strftime` specified timestamp formatting. Default `%Y-%m-%dT%H:%M:%S.000`",
		},
		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "Level of Gzip compression from `0-9`. `0` means no compression. `1` is the fastest and the least compressed version, `9` is the slowest and the most compressed version. Default `0`",
		},
		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "A PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted. Can be either `classic`, `loggly`, `logplex` or `blank`. Default `classic`",
			ValidateDiagFunc: validateLoggingMessageType(),
		},
		"file_max_bytes": {
			Type:        schema.TypeInt,
			Optional:    true,
			Description: "Maximum size of an uploaded log file, if non-zero.",
		},
		"compression_codec": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      `The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.`,
			ValidateDiagFunc: validateLoggingCompressionCodec(),
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%h %l %u %t \"%r\" %>s %b",
			Description: "Apache-style string or VCL variables to use for log formatting (default: `%h %l %u %t \"%r\" %>s %b`)",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          2,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2)",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of the condition to apply",
		}
	}

	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
}

func (h *BlobStorageLoggingServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	var vla = h.getVCLLoggingAttributes(resource)
	opts := gofastly.CreateBlobStorageInput{
		ServiceID:         d.Id(),
		ServiceVersion:    serviceVersion,
		Name:              resource["name"].(string),
		Path:              resource["path"].(string),
		AccountName:       resource["account_name"].(string),
		Container:         resource["container"].(string),
		SASToken:          resource["sas_token"].(string),
		Period:            uint(resource["period"].(int)),
		TimestampFormat:   resource["timestamp_format"].(string),
		GzipLevel:         uint(resource["gzip_level"].(int)),
		PublicKey:         resource["public_key"].(string),
		MessageType:       resource["message_type"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
		FileMaxBytes:      uint(resource["file_max_bytes"].(int)),
		CompressionCodec:  resource["compression_codec"].(string),
	}

	log.Printf("[DEBUG] Blob Storage logging create opts: %#v", opts)
	_, err := conn.CreateBlobStorage(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *BlobStorageLoggingServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Blob Storages for (%s)", d.Id())
	blobStorageList, err := conn.ListBlobStorages(&gofastly.ListBlobStoragesInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Blob Storages for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	bsl := flattenBlobStorages(blobStorageList)

	for _, element := range bsl {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), bsl); err != nil {
		log.Printf("[WARN] Error setting Blob Storages for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *BlobStorageLoggingServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateBlobStorageInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["path"]; ok {
		opts.Path = gofastly.String(v.(string))
	}
	if v, ok := modified["account_name"]; ok {
		opts.AccountName = gofastly.String(v.(string))
	}
	if v, ok := modified["container"]; ok {
		opts.Container = gofastly.String(v.(string))
	}
	if v, ok := modified["sas_token"]; ok {
		opts.SASToken = gofastly.String(v.(string))
	}
	if v, ok := modified["period"]; ok {
		opts.Period = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["timestamp_format"]; ok {
		opts.TimestampFormat = gofastly.String(v.(string))
	}
	if v, ok := modified["compression_codec"]; ok {
		opts.CompressionCodec = gofastly.String(v.(string))
	}
	if v, ok := modified["gzip_level"]; ok {
		opts.GzipLevel = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["public_key"]; ok {
		opts.PublicKey = gofastly.String(v.(string))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["message_type"]; ok {
		opts.MessageType = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["file_max_bytes"]; ok {
		opts.FileMaxBytes = gofastly.Uint(uint(v.(int)))
	}

	log.Printf("[DEBUG] Update Blob Storage Opts: %#v", opts)
	_, err := conn.UpdateBlobStorage(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *BlobStorageLoggingServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteBlobStorageInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Blob Storage logging removal opts: %#v", opts)
	err := conn.DeleteBlobStorage(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenBlobStorages(blobStorageList []*gofastly.BlobStorage) []map[string]interface{} {
	var bsl []map[string]interface{}
	for _, bs := range blobStorageList {
		// Convert Blob Storages to a map for saving to state.
		nbs := map[string]interface{}{
			"name":               bs.Name,
			"path":               bs.Path,
			"account_name":       bs.AccountName,
			"container":          bs.Container,
			"sas_token":          bs.SASToken,
			"period":             bs.Period,
			"timestamp_format":   bs.TimestampFormat,
			"gzip_level":         bs.GzipLevel,
			"public_key":         bs.PublicKey,
			"format":             bs.Format,
			"format_version":     bs.FormatVersion,
			"message_type":       bs.MessageType,
			"placement":          bs.Placement,
			"response_condition": bs.ResponseCondition,
			"file_max_bytes":     bs.FileMaxBytes,
			"compression_codec":  bs.CompressionCodec,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nbs {
			if v == "" {
				delete(nbs, k)
			}
		}

		bsl = append(bsl, nbs)
	}

	return bsl
}
