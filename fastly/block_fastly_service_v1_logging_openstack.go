package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type OpenstackServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingOpenstack(sa ServiceMetadata) ServiceAttributeDefinition {
	return &OpenstackServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_openstack",
			serviceMetadata: sa,
		},
	}
}

func (h *OpenstackServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	ol, nl := d.GetChange(h.GetKey())

	if ol == nil {
		ol = new(schema.Set)
	}
	if nl == nil {
		nl = new(schema.Set)
	}

	oldSet := ol.(*schema.Set)
	newSet := nl.(*schema.Set)

	setDiff := NewSetDiff(func(resource interface{}) (interface{}, error) {
		t, ok := resource.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("resource failed to be type asserted: %+v", resource)
		}
		return t["name"], nil
	})

	diffResult, err := setDiff.Diff(oldSet, newSet)
	if err != nil {
		return err
	}

	// DELETE removed resources
	for _, resource := range diffResult.Deleted {
		resource := resource.(map[string]interface{})
		opts := h.buildDelete(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly OpenStack logging endpoint removal opts: %#v", opts)

		if err := deleteOpenstack(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly OpenStack logging addition opts: %#v", opts)

		if err := createOpenstack(conn, opts); err != nil {
			return err
		}
	}

	// UPDATE modified resources
	//
	// NOTE: although the go-fastly API client enables updating of a resource by
	// its 'name' attribute, this isn't possible within terraform due to
	// constraints in the data model/schema of the resources not having a uid.
	for _, resource := range diffResult.Modified {
		resource := resource.(map[string]interface{})

		opts := gofastly.UpdateOpenstackInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		// only attempt to update attributes that have changed
		modified := setDiff.Filter(resource, oldSet)

		// NOTE: where we transition between interface{} we lose the ability to
		// infer the underlying type being either a uint vs an int. This
		// materializes as a panic (yay) and so it's only at runtime we discover
		// this and so we've updated the below code to convert the type asserted
		// int into a uint before passing the value to gofastly.Uint().
		if v, ok := modified["access_key"]; ok {
			opts.AccessKey = gofastly.String(v.(string))
		}
		if v, ok := modified["bucket_name"]; ok {
			opts.BucketName = gofastly.String(v.(string))
		}
		if v, ok := modified["url"]; ok {
			opts.URL = gofastly.String(v.(string))
		}
		if v, ok := modified["user"]; ok {
			opts.User = gofastly.String(v.(string))
		}
		if v, ok := modified["path"]; ok {
			opts.Path = gofastly.String(v.(string))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}
		if v, ok := modified["period"]; ok {
			opts.Period = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["compression_codec"]; ok {
			opts.CompressionCodec = gofastly.String(v.(string))
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
		if v, ok := modified["public_key"]; ok {
			opts.PublicKey = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update OpenStack Opts: %#v", opts)
		_, err := conn.UpdateOpenstack(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *OpenstackServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh OpenStack.
	log.Printf("[DEBUG] Refreshing OpenStack logging endpoints for (%s)", d.Id())
	openstackList, err := conn.ListOpenstack(&gofastly.ListOpenstackInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up OpenStack logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenOpenstack(openstackList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting OpenStack logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createOpenstack(conn *gofastly.Client, i *gofastly.CreateOpenstackInput) error {
	_, err := conn.CreateOpenstack(i)
	return err
}

func deleteOpenstack(conn *gofastly.Client, i *gofastly.DeleteOpenstackInput) error {
	err := conn.DeleteOpenstack(i)

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

func flattenOpenstack(openstackList []*gofastly.Openstack) []map[string]interface{} {
	var lsl []map[string]interface{}
	for _, ll := range openstackList {
		// Convert OpenStack logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"url":                ll.URL,
			"user":               ll.User,
			"bucket_name":        ll.BucketName,
			"access_key":         ll.AccessKey,
			"public_key":         ll.PublicKey,
			"gzip_level":         ll.GzipLevel,
			"message_type":       ll.MessageType,
			"path":               ll.Path,
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

func (h *OpenstackServiceAttributeHandler) buildCreate(openstackMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateOpenstackInput {
	df := openstackMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateOpenstackInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		URL:               df["url"].(string),
		User:              df["user"].(string),
		BucketName:        df["bucket_name"].(string),
		AccessKey:         df["access_key"].(string),
		PublicKey:         df["public_key"].(string),
		GzipLevel:         uint(df["gzip_level"].(int)),
		MessageType:       df["message_type"].(string),
		Path:              df["path"].(string),
		Period:            uint(df["period"].(int)),
		TimestampFormat:   df["timestamp_format"].(string),
		CompressionCodec:  df["compression_codec"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *OpenstackServiceAttributeHandler) buildDelete(openstackMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteOpenstackInput {
	df := openstackMap.(map[string]interface{})

	return &gofastly.DeleteOpenstackInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}

func (h *OpenstackServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the OpenStack logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"url": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Your OpenStack auth url",
		},

		"user": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The username for your OpenStack account",
		},

		"bucket_name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name of your OpenStack container",
		},

		"access_key": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "Your OpenStack account access key",
		},

		// Optional fields
		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "A PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},

		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "What level of Gzip encoding to have when dumping logs (default `0`, no compression)",
		},

		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently the logs should be transferred, in seconds. Default `3600`",
		},

		"path": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path",
		},

		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. [Fastly Documentation](https://developer.fastly.com/reference/api/logging/gcs/)",
			ValidateDiagFunc: validateLoggingMessageType(),
		},

		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: "specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
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
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
		}
	}

	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
	return nil
}
