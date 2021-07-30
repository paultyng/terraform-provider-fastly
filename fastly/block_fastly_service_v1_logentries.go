package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type LogentriesServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLogentries(sa ServiceMetadata) ServiceAttributeDefinition {
	return &LogentriesServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logentries",
			serviceMetadata: sa,
		},
	}
}

func (h *LogentriesServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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
		opts := gofastly.DeleteLogentriesInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Fastly Logentries removal opts: %#v", opts)
		err := conn.DeleteLogentries(&opts)
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
		opts := gofastly.CreateLogentriesInput{
			ServiceID:         d.Id(),
			ServiceVersion:    latestVersion,
			Name:              resource["name"].(string),
			Port:              uint(resource["port"].(int)),
			UseTLS:            gofastly.Compatibool(resource["use_tls"].(bool)),
			Token:             resource["token"].(string),
			Format:            vla.format,
			FormatVersion:     uintOrDefault(vla.formatVersion),
			Placement:         vla.placement,
			ResponseCondition: vla.responseCondition,
		}

		log.Printf("[DEBUG] Create Logentries Opts: %#v", opts)
		_, err := conn.CreateLogentries(&opts)
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

		opts := gofastly.UpdateLogentriesInput{
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
		if v, ok := modified["port"]; ok {
			opts.Port = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["use_tls"]; ok {
			opts.UseTLS = gofastly.CBool(v.(bool))
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
		if v, ok := modified["response_condition"]; ok {
			opts.ResponseCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["region"]; ok {
			opts.Region = gofastly.String(v.(string))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Logentries Opts: %#v", opts)
		_, err := conn.UpdateLogentries(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *LogentriesServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Logentries for (%s)", d.Id())
	logentriesList, err := conn.ListLogentries(&gofastly.ListLogentriesInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Logentries for (%s), version (%d): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	lel := flattenLogentries(logentriesList)

	for _, element := range lel {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), lel); err != nil {
		log.Printf("[WARN] Error setting Logentries for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *LogentriesServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Logentries logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Use token based authentication (https://logentries.com/doc/input-token/)",
		},
		// Optional
		"port": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     20000,
			Description: "The port number configured in Logentries",
		},
		"use_tls": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Whether to use TLS for secure logging",
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%h %l %u %t %r %>s",
			Description: "Apache-style string or VCL variables to use for log formatting",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          1,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (Default: 1)",
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

	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
	return nil
}

func flattenLogentries(logentriesList []*gofastly.Logentries) []map[string]interface{} {
	var LEList []map[string]interface{}
	for _, currentLE := range logentriesList {
		// Convert Logentries to a map for saving to state.
		LEMapString := map[string]interface{}{
			"name":               currentLE.Name,
			"port":               currentLE.Port,
			"use_tls":            currentLE.UseTLS,
			"token":              currentLE.Token,
			"format":             currentLE.Format,
			"format_version":     currentLE.FormatVersion,
			"response_condition": currentLE.ResponseCondition,
			"placement":          currentLE.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range LEMapString {
			if v == "" {
				delete(LEMapString, k)
			}
		}

		LEList = append(LEList, LEMapString)
	}

	return LEList
}
