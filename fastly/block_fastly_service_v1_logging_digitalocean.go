package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DigitalOceanServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingDigitalOcean(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&DigitalOceanServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_digitalocean",
			serviceMetadata: sa,
		},
	})
}

func (h *DigitalOceanServiceAttributeHandler) Key() string { return h.key }

func (h *DigitalOceanServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the DigitalOcean Spaces logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"bucket_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of the DigitalOcean Space",
		},

		"access_key": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "Your DigitalOcean Spaces account access key",
		},

		"secret_key": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "Your DigitalOcean Spaces account secret key",
		},

		// Optional fields
		"domain": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The domain of the DigitalOcean Spaces endpoint (default `nyc3.digitaloceanspaces.com`)",
			Default:     "nyc3.digitaloceanspaces.com",
		},

		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "A PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},

		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The path to upload logs to",
		},

		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Description: "How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)",
		},

		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "`strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
		},

		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Description: "What level of Gzip encoding to have when dumping logs (default `0`, no compression)",
		},

		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted. One of: `classic` (default), `loggly`, `logplex` or `blank`",
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
			Description: "Apache style log formatting.",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          2,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.",
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

func (h *DigitalOceanServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildCreate(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly DigitalOcean Spaces logging addition opts: %#v", opts)

	if err := createDigitalOcean(conn, opts); err != nil {
		return err
	}
	return nil
}

func (h *DigitalOceanServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// Refresh DigitalOcean Spaces.
	log.Printf("[DEBUG] Refreshing DigitalOcean Spaces logging endpoints for (%s)", d.Id())
	digitaloceanList, err := conn.ListDigitalOceans(&gofastly.ListDigitalOceansInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up DigitalOcean Spaces logging endpoints for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	ell := flattenDigitalOcean(digitaloceanList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting DigitalOcean Spaces logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *DigitalOceanServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateDigitalOceanInput{
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
		opts.BucketName = gofastly.String(v.(string))
	}
	if v, ok := modified["domain"]; ok {
		opts.Domain = gofastly.String(v.(string))
	}
	if v, ok := modified["access_key"]; ok {
		opts.AccessKey = gofastly.String(v.(string))
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
	if v, ok := modified["gzip_level"]; ok {
		opts.GzipLevel = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["message_type"]; ok {
		opts.MessageType = gofastly.String(v.(string))
	}
	if v, ok := modified["timestamp_format"]; ok {
		opts.TimestampFormat = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}
	if v, ok := modified["public_key"]; ok {
		opts.PublicKey = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update DigitalOcean Opts: %#v", opts)
	_, err := conn.UpdateDigitalOcean(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *DigitalOceanServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly DigitalOcean Spaces logging endpoint removal opts: %#v", opts)

	if err := deleteDigitalOcean(conn, opts); err != nil {
		return err
	}
	return nil
}

func createDigitalOcean(conn *gofastly.Client, i *gofastly.CreateDigitalOceanInput) error {
	_, err := conn.CreateDigitalOcean(i)
	return err
}

func deleteDigitalOcean(conn *gofastly.Client, i *gofastly.DeleteDigitalOceanInput) error {
	err := conn.DeleteDigitalOcean(i)

	errRes, ok := err.(*gofastly.HTTPError)
	if !ok {
		return err
	}

	// 404 response codes don't result in an error propagating because a 404 could
	// indicate that a resource was deleted elsewhere.
	if !errRes.IsNotFound() {
		return err
	}

	return nil
}

func flattenDigitalOcean(digitaloceanList []*gofastly.DigitalOcean) []map[string]interface{} {
	var lsl []map[string]interface{}
	for _, ll := range digitaloceanList {
		// Convert DigitalOcean Spaces logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"bucket_name":        ll.BucketName,
			"domain":             ll.Domain,
			"access_key":         ll.AccessKey,
			"secret_key":         ll.SecretKey,
			"public_key":         ll.PublicKey,
			"path":               ll.Path,
			"period":             ll.Period,
			"timestamp_format":   ll.TimestampFormat,
			"gzip_level":         ll.GzipLevel,
			"format":             ll.Format,
			"format_version":     ll.FormatVersion,
			"message_type":       ll.MessageType,
			"placement":          ll.Placement,
			"response_condition": ll.ResponseCondition,
			"compression_codec":  ll.CompressionCodec,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range nll {
			if v == "" {
				delete(nll, k)
			}
		}

		lsl = append(lsl, nll)
	}

	return lsl
}

func (h *DigitalOceanServiceAttributeHandler) buildCreate(digitaloceanMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateDigitalOceanInput {
	df := digitaloceanMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateDigitalOceanInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		BucketName:        df["bucket_name"].(string),
		Domain:            df["domain"].(string),
		AccessKey:         df["access_key"].(string),
		SecretKey:         df["secret_key"].(string),
		PublicKey:         df["public_key"].(string),
		Path:              df["path"].(string),
		Period:            uint(df["period"].(int)),
		GzipLevel:         uint(df["gzip_level"].(int)),
		TimestampFormat:   df["timestamp_format"].(string),
		MessageType:       df["message_type"].(string),
		CompressionCodec:  df["compression_codec"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *DigitalOceanServiceAttributeHandler) buildDelete(digitaloceanMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteDigitalOceanInput {
	df := digitaloceanMap.(map[string]interface{})

	return &gofastly.DeleteDigitalOceanInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
