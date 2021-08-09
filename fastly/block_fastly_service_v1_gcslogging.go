package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type GCSLoggingServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceGCSLogging(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&GCSLoggingServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "gcslogging",
			serviceMetadata: sa,
		},
	})
}

func (h *GCSLoggingServiceAttributeHandler) Key() string { return h.key }

func (h *GCSLoggingServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify this GCS endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"email": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_GCS_EMAIL", ""),
			Description: "The email address associated with the target GCS bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_EMAIL`",
		},
		"bucket_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of the bucket in which to store the logs",
		},
		"secret_key": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_GCS_SECRET_KEY", ""),
			Description: "The secret key associated with the target gcs bucket on your account. You may optionally provide this secret via an environment variable, `FASTLY_GCS_SECRET_KEY`. A typical format for the key is PEM format, containing actual newline characters where required",
			Sensitive:   true,
		},
		// Optional fields
		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path",
		},
		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "Level of Gzip compression, from `0-9`. `0` is no compression. `1` is fastest and least compressed, `9` is slowest and most compressed. Default `0`",
		},
		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently the logs should be transferred, in seconds (Default 3600)",
		},
		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: "specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
		},
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. [Fastly Documentation](https://developer.fastly.com/reference/api/logging/gcs/)",
			ValidateDiagFunc: validateLoggingMessageType(),
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
			Default:     "%h %l %u %t %r %>s",
			Description: "Apache-style string or VCL variables to use for log formatting",
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Name of a condition to apply this logging.",
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed.",
			ValidateDiagFunc: validateLoggingPlacement(),
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

func (h *GCSLoggingServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	var vla = h.getVCLLoggingAttributes(resource)
	opts := gofastly.CreateGCSInput{
		ServiceID:         d.Id(),
		ServiceVersion:    serviceVersion,
		Name:              resource["name"].(string),
		User:              resource["email"].(string),
		Bucket:            resource["bucket_name"].(string),
		SecretKey:         resource["secret_key"].(string),
		Path:              resource["path"].(string),
		Period:            uint(resource["period"].(int)),
		GzipLevel:         uint8(resource["gzip_level"].(int)),
		TimestampFormat:   resource["timestamp_format"].(string),
		MessageType:       resource["message_type"].(string),
		CompressionCodec:  resource["compression_codec"].(string),
		Format:            vla.format,
		ResponseCondition: vla.responseCondition,
		Placement:         vla.placement,
	}

	log.Printf("[DEBUG] Create GCS Opts: %#v", opts)
	_, err := conn.CreateGCS(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *GCSLoggingServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing GCS for (%s)", d.Id())
	GCSList, err := conn.ListGCSs(&gofastly.ListGCSsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up GCS for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	gcsl := flattenGCS(GCSList)

	for _, element := range gcsl {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), gcsl); err != nil {
		log.Printf("[WARN] Error setting gcs for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *GCSLoggingServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateGCSInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["bucket_name"]; ok {
		opts.Bucket = gofastly.String(v.(string))
	}
	if v, ok := modified["user"]; ok {
		opts.User = gofastly.String(v.(string))
	}
	if v, ok := modified["secret_key"]; ok {
		opts.SecretKey = gofastly.String(v.(string))
	}
	if v, ok := modified["path"]; ok {
		opts.Path = gofastly.String(v.(string))
	}
	if v, ok := modified["period"]; ok {
		opts.Period = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["compression_codec"]; ok {
		opts.CompressionCodec = gofastly.String(v.(string))
	}
	if v, ok := modified["gzip_level"]; ok {
		opts.GzipLevel = gofastly.Uint8(uint8(v.(int)))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["message_type"]; ok {
		opts.MessageType = gofastly.String(v.(string))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["timestamp_format"]; ok {
		opts.TimestampFormat = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update GCS Opts: %#v", opts)
	_, err := conn.UpdateGCS(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *GCSLoggingServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteGCSInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly gcslogging removal opts: %#v", opts)
	err := conn.DeleteGCS(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenGCS(gcsList []*gofastly.GCS) []map[string]interface{} {
	var GCSList []map[string]interface{}
	for _, currentGCS := range gcsList {
		// Convert gcs to a map for saving to state.
		GCSMapString := map[string]interface{}{
			"name":               currentGCS.Name,
			"email":              currentGCS.User,
			"bucket_name":        currentGCS.Bucket,
			"secret_key":         currentGCS.SecretKey,
			"path":               currentGCS.Path,
			"period":             int(currentGCS.Period),
			"gzip_level":         int(currentGCS.GzipLevel),
			"response_condition": currentGCS.ResponseCondition,
			"message_type":       currentGCS.MessageType,
			"format":             currentGCS.Format,
			"timestamp_format":   currentGCS.TimestampFormat,
			"placement":          currentGCS.Placement,
			"compression_codec":  currentGCS.CompressionCodec,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range GCSMapString {
			if v == "" {
				delete(GCSMapString, k)
			}
		}

		GCSList = append(GCSList, GCSMapString)
	}

	return GCSList
}
