package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SyslogServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceSyslog(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&SyslogServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "syslog",
			serviceMetadata: sa,
		},
	})
}

func (h *SyslogServiceAttributeHandler) Key() string { return h.key }

func (h *SyslogServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify this Syslog endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"address": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A hostname or IPv4 address of the Syslog endpoint",
		},
		// Optional
		"port": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     514,
			Description: "The port associated with the address where the Syslog endpoint can be accessed. Default `514`",
		},
		"token": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Whether to prepend each message with a specific token",
		},
		"use_tls": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
			Description: "Whether to use TLS for secure logging. Default `false`",
		},
		"tls_hostname": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Used during the TLS handshake to validate the certificate",
		},
		"tls_ca_cert": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SYSLOG_CA_CERT", ""),
			Description: "A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CA_CERT`",
		},
		"tls_client_cert": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SYSLOG_CLIENT_CERT", ""),
			Description: "The client certificate used to make authenticated requests. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SYSLOG_CLIENT_CERT`",
		},
		"tls_client_key": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SYSLOG_CLIENT_KEY", ""),
			Description: "The client private key used to make authenticated requests. Must be in PEM format. You can provide this key via an environment variable, `FASTLY_SYSLOG_CLIENT_KEY`",
			Sensitive:   true,
		},
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`",
			ValidateDiagFunc: validateLoggingMessageType(),
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%h %l %u %t \"%r\" %>s %b",
			Description: "Apache-style string or VCL variables to use for log formatting",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          1,
			Description:      "The version of the custom logging format. Can be either 1 or 2. (Default: 1)",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "Name of blockAttributes condition to apply this logging.",
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

func (h *SyslogServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	var vla = h.getVCLLoggingAttributes(resource)
	opts := gofastly.CreateSyslogInput{
		ServiceID:         d.Id(),
		ServiceVersion:    serviceVersion,
		Name:              resource["name"].(string),
		Address:           resource["address"].(string),
		Port:              uint(resource["port"].(int)),
		Token:             resource["token"].(string),
		UseTLS:            gofastly.Compatibool(resource["use_tls"].(bool)),
		TLSHostname:       resource["tls_hostname"].(string),
		TLSCACert:         resource["tls_ca_cert"].(string),
		TLSClientCert:     resource["tls_client_cert"].(string),
		TLSClientKey:      resource["tls_client_key"].(string),
		MessageType:       resource["message_type"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		ResponseCondition: vla.responseCondition,
		Placement:         vla.placement,
	}

	log.Printf("[DEBUG] Create Syslog Opts: %#v", opts)
	_, err := conn.CreateSyslog(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *SyslogServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Syslog for (%s)", d.Id())
	syslogList, err := conn.ListSyslogs(&gofastly.ListSyslogsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Syslog for (%s), version (%d): %s", d.Id(), serviceVersion, err)
	}

	sll := flattenSyslogs(syslogList)

	for _, element := range sll {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), sll); err != nil {
		log.Printf("[WARN] Error setting Syslog for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *SyslogServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateSyslogInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["address"]; ok {
		opts.Address = gofastly.String(v.(string))
	}
	if v, ok := modified["hostname"]; ok {
		opts.Hostname = gofastly.String(v.(string))
	}
	if v, ok := modified["port"]; ok {
		opts.Port = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["use_tls"]; ok {
		opts.UseTLS = gofastly.CBool(v.(bool))
	}
	if v, ok := modified["ipv4"]; ok {
		opts.IPV4 = gofastly.String(v.(string))
	}
	if v, ok := modified["tls_ca_cert"]; ok {
		opts.TLSCACert = gofastly.String(v.(string))
	}
	if v, ok := modified["tls_hostname"]; ok {
		opts.TLSHostname = gofastly.String(v.(string))
	}
	if v, ok := modified["tls_client_cert"]; ok {
		opts.TLSClientCert = gofastly.String(v.(string))
	}
	if v, ok := modified["tls_client_key"]; ok {
		opts.TLSClientKey = gofastly.String(v.(string))
	}
	if v, ok := modified["token"]; ok {
		opts.Token = gofastly.String(v.(string))
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
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Syslog Opts: %#v", opts)
	_, err := conn.UpdateSyslog(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *SyslogServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteSyslogInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Syslog removal opts: %#v", opts)
	err := conn.DeleteSyslog(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenSyslogs(syslogList []*gofastly.Syslog) []map[string]interface{} {
	var pl []map[string]interface{}
	for _, p := range syslogList {
		// Convert Syslog to a map for saving to state.
		ns := map[string]interface{}{
			"name":               p.Name,
			"address":            p.Address,
			"port":               p.Port,
			"format":             p.Format,
			"format_version":     p.FormatVersion,
			"token":              p.Token,
			"use_tls":            p.UseTLS,
			"tls_hostname":       p.TLSHostname,
			"tls_ca_cert":        p.TLSCACert,
			"tls_client_cert":    p.TLSClientCert,
			"tls_client_key":     p.TLSClientKey,
			"response_condition": p.ResponseCondition,
			"message_type":       p.MessageType,
			"placement":          p.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range ns {
			if v == "" {
				delete(ns, k)
			}
		}

		pl = append(pl, ns)
	}

	return pl
}
