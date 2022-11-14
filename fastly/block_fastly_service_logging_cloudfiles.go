package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v7/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// CloudfilesServiceAttributeHandler provides a base implementation for ServiceAttributeDefinition.
type CloudfilesServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

// NewServiceLoggingCloudfiles returns a new resource.
func NewServiceLoggingCloudfiles(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&CloudfilesServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_cloudfiles",
			serviceMetadata: sa,
		},
	})
}

// Key returns the resource key.
func (h *CloudfilesServiceAttributeHandler) Key() string {
	return h.key
}

// GetSchema returns the resource schema.
func (h *CloudfilesServiceAttributeHandler) GetSchema() *schema.Schema {
	blockAttributes := map[string]*schema.Schema{
		"access_key": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "Your Cloud File account access key",
		},
		"bucket_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of your Cloud Files container",
		},
		"compression_codec": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      `The codec used for compression of your logs. Valid values are zstd, snappy, and gzip. If the specified codec is "gzip", gzip_level will default to 3. To specify a different level, leave compression_codec blank and explicitly set the level using gzip_level. Specifying both compression_codec and gzip_level in the same API request will result in an error.`,
			ValidateDiagFunc: validateLoggingCompressionCodec(),
		},
		"gzip_level": {
			Type:     schema.TypeInt,
			Optional: true,
			// NOTE: The default represents an unset value
			// We use this instead of zero because the zero value for an int type is
			// actually a valid value for the API. The API will attempt to default to
			// zero if nothing is set by the user in their TF configuration.
			Default:     -1,
			Description: GzipLevelDescription,
		},
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      MessageTypeDescription,
			ValidateDiagFunc: validateLoggingMessageType(),
		},
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Rackspace Cloud Files logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The path to upload logs to",
		},
		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)",
		},
		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "The PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},
		"region": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The region to stream logs to. One of: DFW (Dallas), ORD (Chicago), IAD (Northern Virginia), LON (London), SYD (Sydney), HKG (Hong Kong)",
		},
		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: TimestampFormatDescription,
		},
		"user": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The username for your Cloud Files account",
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

// Create creates the resource.
func (h *CloudfilesServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]any, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildCreate(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Cloud Files logging addition opts: %#v", opts)

	return createCloudfiles(conn, opts)
}

// Read refreshes the resource.
func (h *CloudfilesServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]any, serviceVersion int, conn *gofastly.Client) error {
	resources := d.Get(h.GetKey()).(*schema.Set).List()

	if len(resources) > 0 || d.Get("imported").(bool) {
		// Refresh Cloud Files.
		log.Printf("[DEBUG] Refreshing Cloud Files logging endpoints for (%s)", d.Id())
		cloudfilesList, err := conn.ListCloudfiles(&gofastly.ListCloudfilesInput{
			ServiceID:      d.Id(),
			ServiceVersion: serviceVersion,
		})
		if err != nil {
			return fmt.Errorf("error looking up Cloud Files logging endpoints for (%s), version (%v): %s", d.Id(), serviceVersion, err)
		}

		ell := flattenCloudfiles(cloudfilesList, resources)

		for _, element := range ell {
			h.pruneVCLLoggingAttributes(element)
		}

		if err := d.Set(h.GetKey(), ell); err != nil {
			log.Printf("[WARN] Error setting Cloud Files logging endpoints for (%s): %s", d.Id(), err)
		}
	}

	return nil
}

// Update updates the resource.
func (h *CloudfilesServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]any, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateCloudfilesInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: When converting from an interface{} we lose the underlying type.
	// Converting to the wrong type will result in a runtime panic.
	if v, ok := modified["user"]; ok {
		opts.User = gofastly.String(v.(string))
	}
	if v, ok := modified["access_key"]; ok {
		opts.AccessKey = gofastly.String(v.(string))
	}
	if v, ok := modified["bucket_name"]; ok {
		opts.BucketName = gofastly.String(v.(string))
	}
	if v, ok := modified["path"]; ok {
		opts.Path = gofastly.String(v.(string))
	}
	if v, ok := modified["region"]; ok {
		opts.Region = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}
	if v, ok := modified["period"]; ok {
		opts.Period = gofastly.Int(v.(int))
	}
	if v, ok := modified["gzip_level"]; ok {
		opts.GzipLevel = gofastly.Int(v.(int))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Int(v.(int))
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
	if v, ok := modified["public_key"]; ok {
		opts.PublicKey = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Cloud Files Opts: %#v", opts)
	_, err := conn.UpdateCloudfiles(&opts)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes the resource.
func (h *CloudfilesServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]any, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Cloud Files logging endpoint removal opts: %#v", opts)

	return deleteCloudfiles(conn, opts)
}

func createCloudfiles(conn *gofastly.Client, i *gofastly.CreateCloudfilesInput) error {
	_, err := conn.CreateCloudfiles(i)
	return err
}

func deleteCloudfiles(conn *gofastly.Client, i *gofastly.DeleteCloudfilesInput) error {
	err := conn.DeleteCloudfiles(i)

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

func flattenCloudfiles(cloudfilesList []*gofastly.Cloudfiles, state []any) []map[string]any {
	var lsl []map[string]any
	for _, ll := range cloudfilesList {
		// Avoid setting gzip_level to the API default of zero if originally unset.
		// This avoids an unnecessary diff where the local state would have been
		// updated to zero and so would be different from the -1 default set.
		// As the user never set the attribute we don't want to show a diff to say
		// it should be zero according to the API.
		//
		// NOTE: Ideally the local state would be updated when .Create() is called.
		// e.g. we'd check if the value is -1 for gzip_level and set it in state as
		// zero instead. This way we could avoid having to do this check here.
		// The reason that's not possible (or not ideal at least) is because Create
		// is called multiple times (once for each block defined in configuration)
		// while the setting of the state must be done holistically, and so what
		// that means is, if we did the above suggestion we would be resetting the
		// entire state object multiple times, where as here we're only ever setting
		// it once.
		for _, s := range state {
			v := s.(map[string]any)
			if v["name"].(string) == ll.Name && v["gzip_level"].(int) == -1 {
				ll.GzipLevel = v["gzip_level"].(int)
				break
			}
		}

		// Convert Cloud Files logging to a map for saving to state.
		nll := map[string]any{
			"name":               ll.Name,
			"bucket_name":        ll.BucketName,
			"user":               ll.User,
			"access_key":         ll.AccessKey,
			"public_key":         ll.PublicKey,
			"gzip_level":         ll.GzipLevel,
			"message_type":       ll.MessageType,
			"path":               ll.Path,
			"region":             ll.Region,
			"period":             ll.Period,
			"timestamp_format":   ll.TimestampFormat,
			"format":             ll.Format,
			"format_version":     ll.FormatVersion,
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

func (h *CloudfilesServiceAttributeHandler) buildCreate(cloudfilesMap any, serviceID string, serviceVersion int) *gofastly.CreateCloudfilesInput {
	df := cloudfilesMap.(map[string]any)

	vla := h.getVCLLoggingAttributes(df)
	opts := &gofastly.CreateCloudfilesInput{
		AccessKey:        gofastly.String(df["access_key"].(string)),
		BucketName:       gofastly.String(df["bucket_name"].(string)),
		CompressionCodec: gofastly.String(df["compression_codec"].(string)),
		Format:           gofastly.String(vla.format),
		FormatVersion:    vla.formatVersion,
		MessageType:      gofastly.String(df["message_type"].(string)),
		Name:             gofastly.String(df["name"].(string)),
		Path:             gofastly.String(df["path"].(string)),
		Period:           gofastly.Int(df["period"].(int)),
		Placement:        gofastly.String(vla.placement),
		PublicKey:        gofastly.String(df["public_key"].(string)),
		Region:           gofastly.String(df["region"].(string)),
		ServiceID:        serviceID,
		ServiceVersion:   serviceVersion,
		TimestampFormat:  gofastly.String(df["timestamp_format"].(string)),
		User:             gofastly.String(df["user"].(string)),
	}

	// NOTE: go-fastly v7+ expects a pointer, so TF can't set the zero type value.
	// If we set a default value for an attribute, then it will be sent to the API.
	// In some scenarios this can cause the API to reject the request.
	// For example, configuring compression_codec + gzip_level is invalid.
	if gl, ok := df["gzip_level"].(int); ok && gl != -1 {
		opts.GzipLevel = gofastly.Int(gl)
	}

	// WARNING: The following fields shouldn't have an emptry string passed.
	// As it will cause the Fastly API to return an error.
	// This is because go-fastly v7+ will not 'omitempty' due to pointer type.
	if vla.responseCondition != "" {
		opts.ResponseCondition = gofastly.String(vla.responseCondition)
	}

	return opts
}

func (h *CloudfilesServiceAttributeHandler) buildDelete(cloudfilesMap any, serviceID string, serviceVersion int) *gofastly.DeleteCloudfilesInput {
	df := cloudfilesMap.(map[string]any)

	return &gofastly.DeleteCloudfilesInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
