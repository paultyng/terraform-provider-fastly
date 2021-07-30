package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SplunkServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceSplunk(sa ServiceMetadata) ServiceAttributeDefinition {
	return &SplunkServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "splunk",
			serviceMetadata: sa,
		},
	}
}

func (h *SplunkServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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
		opts := gofastly.DeleteSplunkInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Splunk removal opts: %#v", opts)
		err := conn.DeleteSplunk(&opts)
		if errRes, ok := err.(*gofastly.HTTPError); ok {
			if errRes.StatusCode != 404 {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		var vla = h.getVCLLoggingAttributes(resource)
		opts := gofastly.CreateSplunkInput{
			ServiceID:         d.Id(),
			ServiceVersion:    latestVersion,
			Name:              resource["name"].(string),
			URL:               resource["url"].(string),
			Token:             resource["token"].(string),
			TLSHostname:       resource["tls_hostname"].(string),
			TLSCACert:         resource["tls_ca_cert"].(string),
			TLSClientCert:     resource["tls_client_cert"].(string),
			TLSClientKey:      resource["tls_client_key"].(string),
			Format:            vla.format,
			FormatVersion:     uintOrDefault(vla.formatVersion),
			ResponseCondition: vla.responseCondition,
			Placement:         vla.placement,
		}

		log.Printf("[DEBUG] Splunk create opts: %#v", opts)
		_, err := conn.CreateSplunk(&opts)
		if err != nil {
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

		opts := gofastly.UpdateSplunkInput{
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
		if v, ok := modified["url"]; ok {
			opts.URL = gofastly.String(v.(string))
		}
		if v, ok := modified["request_max_entries"]; ok {
			opts.RequestMaxEntries = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["request_max_bytes"]; ok {
			opts.RequestMaxBytes = gofastly.Uint(uint(v.(int)))
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
		if v, ok := modified["token"]; ok {
			opts.Token = gofastly.String(v.(string))
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

		log.Printf("[DEBUG] Update Splunk Opts: %#v", opts)
		_, err := conn.UpdateSplunk(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *SplunkServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Splunks for (%s)", d.Id())
	splunkList, err := conn.ListSplunks(&gofastly.ListSplunksInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Splunks for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	spl := flattenSplunks(splunkList)

	for _, element := range spl {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), spl); err != nil {
		log.Printf("[WARN] Error setting Splunks for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *SplunkServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify the Splunk endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"url": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The Splunk URL to stream logs to",
		},
		"token": {
			Type:        schema.TypeString,
			Required:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SPLUNK_TOKEN", ""),
			Description: "The Splunk token to be used for authentication",
			Sensitive:   true,
		},
		// Optional fields
		"tls_hostname": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The hostname used to verify the server's certificate. It can either be the Common Name or a Subject Alternative Name (SAN)",
		},
		"tls_ca_cert": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SPLUNK_CA_CERT", ""),
			Description: "A secure certificate to authenticate the server with. Must be in PEM format. You can provide this certificate via an environment variable, `FASTLY_SPLUNK_CA_CERT`",
		},
		"tls_client_cert": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SPLUNK_CLIENT_CERT", ""),
			Description: "The client certificate used to make authenticated requests. Must be in PEM format.",
		},
		"tls_client_key": {
			Type:        schema.TypeString,
			Optional:    true,
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_SPLUNK_CLIENT_KEY", ""),
			Description: "The client private key used to make authenticated requests. Must be in PEM format.",
			Sensitive:   true,
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

	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
	return nil
}

func flattenSplunks(splunkList []*gofastly.Splunk) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, s := range splunkList {
		// Convert Splunk to a map for saving to state.
		nbs := map[string]interface{}{
			"name":               s.Name,
			"url":                s.URL,
			"format":             s.Format,
			"format_version":     s.FormatVersion,
			"response_condition": s.ResponseCondition,
			"placement":          s.Placement,
			"token":              s.Token,
			"tls_hostname":       s.TLSHostname,
			"tls_ca_cert":        s.TLSCACert,
			"tls_client_cert":    s.TLSClientCert,
			"tls_client_key":     s.TLSClientKey,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nbs {
			if v == "" {
				delete(nbs, k)
			}
		}

		sl = append(sl, nbs)
	}

	return sl
}
