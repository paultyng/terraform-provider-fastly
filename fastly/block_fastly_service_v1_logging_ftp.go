package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type FTPServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingFTP(sa ServiceMetadata) ServiceAttributeDefinition {
	return &FTPServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_ftp",
			serviceMetadata: sa,
		},
	}
}

func (h *FTPServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	of, nf := d.GetChange(h.GetKey())

	if of == nil {
		of = new(schema.Set)
	}
	if nf == nil {
		nf = new(schema.Set)
	}

	oldSet := of.(*schema.Set)
	newSet := nf.(*schema.Set)

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

		log.Printf("[DEBUG] Fastly FTP logging endpoint removal opts: %#v", opts)

		if err := deleteFTP(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly FTP logging addition opts: %#v", opts)

		if err := createFTP(conn, opts); err != nil {
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

		opts := gofastly.UpdateFTPInput{
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
		if v, ok := modified["address"]; ok {
			opts.Address = gofastly.String(v.(string))
		}
		if v, ok := modified["port"]; ok {
			opts.Port = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["public_key"]; ok {
			opts.PublicKey = gofastly.String(v.(string))
		}
		if v, ok := modified["user"]; ok {
			opts.Username = gofastly.String(v.(string))
		}
		if v, ok := modified["password"]; ok {
			opts.Password = gofastly.String(v.(string))
		}
		if v, ok := modified["path"]; ok {
			opts.Path = gofastly.String(v.(string))
		}
		if v, ok := modified["period"]; ok {
			opts.Period = gofastly.Uint(uint(v.(int)))
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
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}
		if v, ok := modified["gzip_level"]; ok {
			opts.GzipLevel = gofastly.Uint8(uint8(v.(int)))
		}
		if v, ok := modified["compression_codec"]; ok {
			opts.CompressionCodec = gofastly.String(v.(string))
		}
		if v, ok := modified["message_type"]; ok {
			opts.MessageType = gofastly.String(v.(string))
		}
		if v, ok := modified["timestamp_format"]; ok {
			opts.TimestampFormat = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update FTP Opts: %#v", opts)
		_, err := conn.UpdateFTP(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *FTPServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh FTP.
	log.Printf("[DEBUG] Refreshing FTP logging endpoints for (%s)", d.Id())
	ftpList, err := conn.ListFTPs(&gofastly.ListFTPsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up FTP logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenFTP(ftpList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting FTP logging endpoints for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *FTPServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the FTP logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"address": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The FTP address to stream logs to",
		},

		"user": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The username for the server (can be `anonymous`)",
		},

		"password": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The password for the server (for anonymous use an email address)",
			Sensitive:   true,
		},

		"path": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The path to upload log files to. If the path ends in `/` then it is treated as a directory",
		},

		// Optional fields
		"port": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     21,
			Description: "The port number. Default: `21`",
		},

		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently the logs should be transferred, in seconds (Default `3600`)",
		},

		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "The PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},

		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "Gzip Compression level. Default `0`",
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
			Description:      "How the message should be formatted (default: `classic`)",
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
			Description: "Apache-style string or VCL variables to use for log formatting.",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          2,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed.",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of the condition to apply.",
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

func createFTP(conn *gofastly.Client, i *gofastly.CreateFTPInput) error {
	_, err := conn.CreateFTP(i)
	return err
}

func deleteFTP(conn *gofastly.Client, i *gofastly.DeleteFTPInput) error {
	err := conn.DeleteFTP(i)
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

func flattenFTP(ftpList []*gofastly.FTP) []map[string]interface{} {
	var fsl []map[string]interface{}
	for _, fl := range ftpList {
		// Convert FTP logging to a map for saving to state.
		nfl := map[string]interface{}{
			"name":               fl.Name,
			"address":            fl.Address,
			"user":               fl.Username,
			"password":           fl.Password,
			"path":               fl.Path,
			"port":               fl.Port,
			"period":             fl.Period,
			"public_key":         fl.PublicKey,
			"gzip_level":         fl.GzipLevel,
			"timestamp_format":   fl.TimestampFormat,
			"format":             fl.Format,
			"format_version":     fl.FormatVersion,
			"message_type":       fl.MessageType,
			"placement":          fl.Placement,
			"response_condition": fl.ResponseCondition,
			"compression_codec":  fl.CompressionCodec,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range nfl {
			if v == "" {
				delete(nfl, k)
			}
		}

		fsl = append(fsl, nfl)
	}

	return fsl
}

func (h *FTPServiceAttributeHandler) buildCreate(ftpMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateFTPInput {
	df := ftpMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateFTPInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Address:           df["address"].(string),
		Username:          df["user"].(string),
		Password:          df["password"].(string),
		Path:              df["path"].(string),
		Port:              uint(df["port"].(int)),
		Period:            uint(df["period"].(int)),
		PublicKey:         df["public_key"].(string),
		GzipLevel:         uint8(df["gzip_level"].(int)),
		TimestampFormat:   df["timestamp_format"].(string),
		MessageType:       df["message_type"].(string),
		CompressionCodec:  df["compression_codec"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *FTPServiceAttributeHandler) buildDelete(ftpMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteFTPInput {
	df := ftpMap.(map[string]interface{})

	return &gofastly.DeleteFTPInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
