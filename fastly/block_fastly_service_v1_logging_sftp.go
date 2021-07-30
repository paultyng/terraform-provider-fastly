package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SFTPServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingSFTP(sa ServiceMetadata) ServiceAttributeDefinition {
	return &SFTPServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_sftp",
			serviceMetadata: sa,
		},
	}
}

func (h *SFTPServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the SFTP logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"address": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The SFTP address to stream logs to",
		},

		"user": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The username for the server",
		},

		"path": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The path to upload log files to. If the path ends in `/` then it is treated as a directory",
		},

		"ssh_known_hosts": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A list of host keys for all hosts we can connect to over SFTP",
		},

		// Optional fields
		"port": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     22,
			Description: "The port the SFTP service listens on. (Default: `22`)",
		},

		"password": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The password for the server. If both `password` and `secret_key` are passed, `secret_key` will be preferred",
			Sensitive:   true,
		},

		"secret_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "The SSH private key for the server. If both `password` and `secret_key` are passed, `secret_key` will be preferred",
			Sensitive:        true,
			ValidateDiagFunc: validateStringTrimmed,
		},

		"public_key": {
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "A PGP public key that Fastly will use to encrypt your log files before writing them to disk",
			ValidateDiagFunc: validateStringTrimmed,
		},

		"period": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     3600,
			Description: "How frequently log files are finalized so they can be available for reading (in seconds, default `3600`)",
		},

		"gzip_level": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "What level of Gzip encoding to have when dumping logs (default `0`, no compression)",
		},

		"timestamp_format": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%Y-%m-%dT%H:%M:%S.000",
			Description: "The `strftime` specified timestamp formatting (default `%Y-%m-%dT%H:%M:%S.000`)",
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
			Default:     "%h %l %u %t \"%r\" %>s %b",
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

func (h *SFTPServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	os, ns := d.GetChange(h.GetKey())

	if os == nil {
		os = new(schema.Set)
	}
	if ns == nil {
		ns = new(schema.Set)
	}

	oldSet := os.(*schema.Set)
	newSet := ns.(*schema.Set)

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

		log.Printf("[DEBUG] Fastly SFTP logging endpoint removal opts: %#v", opts)

		if err := deleteSFTP(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		if opts.Password == "" && opts.SecretKey == "" {
			return fmt.Errorf("[ERR] Either password or secret_key must be set")
		}

		log.Printf("[DEBUG] Fastly SFTP logging addition opts: %#v", opts)

		if err := createSFTP(conn, opts); err != nil {
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

		opts := gofastly.UpdateSFTPInput{
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
		if v, ok := modified["secret_key"]; ok {
			opts.SecretKey = gofastly.String(v.(string))
		}
		if v, ok := modified["ssh_known_hosts"]; ok {
			opts.SSHKnownHosts = gofastly.String(v.(string))
		}
		if v, ok := modified["user"]; ok {
			opts.User = gofastly.String(v.(string))
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
		if v, ok := modified["format_version"]; ok {
			opts.FormatVersion = gofastly.Uint(uint(v.(int)))
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
		if v, ok := modified["response_condition"]; ok {
			opts.ResponseCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["timestamp_format"]; ok {
			opts.TimestampFormat = gofastly.String(v.(string))
		}
		if v, ok := modified["message_type"]; ok {
			opts.MessageType = gofastly.String(v.(string))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update SFTP Opts: %#v", opts)
		_, err := conn.UpdateSFTP(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *SFTPServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh SFTP.
	log.Printf("[DEBUG] Refreshing SFTP logging endpoints for (%s)", d.Id())
	sftpList, err := conn.ListSFTPs(&gofastly.ListSFTPsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up SFTP logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenSFTP(sftpList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting SFTP logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createSFTP(conn *gofastly.Client, i *gofastly.CreateSFTPInput) error {
	_, err := conn.CreateSFTP(i)
	return err
}

func deleteSFTP(conn *gofastly.Client, i *gofastly.DeleteSFTPInput) error {
	err := conn.DeleteSFTP(i)

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

func flattenSFTP(sftpList []*gofastly.SFTP) []map[string]interface{} {
	var ssl []map[string]interface{}
	for _, sl := range sftpList {
		// Convert SFTP logging to a map for saving to state.
		nsl := map[string]interface{}{
			"name":               sl.Name,
			"address":            sl.Address,
			"user":               sl.User,
			"path":               sl.Path,
			"ssh_known_hosts":    sl.SSHKnownHosts,
			"port":               sl.Port,
			"password":           sl.Password,
			"secret_key":         sl.SecretKey,
			"public_key":         sl.PublicKey,
			"period":             sl.Period,
			"gzip_level":         sl.GzipLevel,
			"timestamp_format":   sl.TimestampFormat,
			"message_type":       sl.MessageType,
			"format":             sl.Format,
			"format_version":     sl.FormatVersion,
			"response_condition": sl.ResponseCondition,
			"placement":          sl.Placement,
			"compression_codec":  sl.CompressionCodec,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nsl {
			if v == "" {
				delete(nsl, k)
			}
		}

		ssl = append(ssl, nsl)
	}

	return ssl
}

func (h *SFTPServiceAttributeHandler) buildCreate(sftpMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateSFTPInput {
	df := sftpMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateSFTPInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Address:           df["address"].(string),
		Name:              df["name"].(string),
		User:              df["user"].(string),
		Path:              df["path"].(string),
		PublicKey:         df["public_key"].(string),
		SecretKey:         df["secret_key"].(string),
		SSHKnownHosts:     df["ssh_known_hosts"].(string),
		Port:              uint(df["port"].(int)),
		Password:          df["password"].(string),
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

func (h *SFTPServiceAttributeHandler) buildDelete(sftpMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteSFTPInput {
	df := sftpMap.(map[string]interface{})

	return &gofastly.DeleteSFTPInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
