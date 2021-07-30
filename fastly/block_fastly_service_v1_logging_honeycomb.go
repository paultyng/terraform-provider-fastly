package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type HoneycombServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingHoneycomb(sa ServiceMetadata) ServiceAttributeDefinition {
	return &HoneycombServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_honeycomb",
			serviceMetadata: sa,
		},
	}
}

func (h *HoneycombServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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

		log.Printf("[DEBUG] Fastly Honeycomb logging endpoint removal opts: %#v", opts)

		if err := deleteHoneycomb(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Honeycomb logging addition opts: %#v", opts)

		if err := createHoneycomb(conn, opts); err != nil {
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

		opts := gofastly.UpdateHoneycombInput{
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
		if v, ok := modified["format"]; ok {
			opts.Format = gofastly.String(v.(string))
		}
		if v, ok := modified["format_version"]; ok {
			opts.FormatVersion = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["dataset"]; ok {
			opts.Dataset = gofastly.String(v.(string))
		}
		if v, ok := modified["token"]; ok {
			opts.Token = gofastly.String(v.(string))
		}
		if v, ok := modified["response_condition"]; ok {
			opts.ResponseCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Honeycomb Opts: %#v", opts)
		_, err := conn.UpdateHoneycomb(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *HoneycombServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh Honeycomb.
	log.Printf("[DEBUG] Refreshing Honeycomb logging endpoints for (%s)", d.Id())
	honeycombList, err := conn.ListHoneycombs(&gofastly.ListHoneycombsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Honeycomb logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	ell := flattenHoneycomb(honeycombList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting Honeycomb logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createHoneycomb(conn *gofastly.Client, i *gofastly.CreateHoneycombInput) error {
	_, err := conn.CreateHoneycomb(i)
	return err
}

func deleteHoneycomb(conn *gofastly.Client, i *gofastly.DeleteHoneycombInput) error {
	err := conn.DeleteHoneycomb(i)

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

func flattenHoneycomb(honeycombList []*gofastly.Honeycomb) []map[string]interface{} {
	var lsl []map[string]interface{}
	for _, ll := range honeycombList {
		// Convert Honeycomb logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"token":              ll.Token,
			"dataset":            ll.Dataset,
			"format":             ll.Format,
			"format_version":     ll.FormatVersion,
			"placement":          ll.Placement,
			"response_condition": ll.ResponseCondition,
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

func (h *HoneycombServiceAttributeHandler) buildCreate(honeycombMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateHoneycombInput {
	df := honeycombMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateHoneycombInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Token:             df["token"].(string),
		Dataset:           df["dataset"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *HoneycombServiceAttributeHandler) buildDelete(honeycombMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteHoneycombInput {
	df := honeycombMap.(map[string]interface{})

	return &gofastly.DeleteHoneycombInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}

func (h *HoneycombServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Honeycomb logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "The Write Key from the Account page of your Honeycomb account",
		},

		"dataset": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The Honeycomb Dataset you want to log to",
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Apache style log formatting. Your log must produce valid JSON that Honeycomb can ingest.",
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
