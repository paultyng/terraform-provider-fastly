package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SumologicServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceSumologic(sa ServiceMetadata) ServiceAttributeDefinition {
	return &SumologicServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "sumologic",
			serviceMetadata: sa,
		},
	}
}

func (h *SumologicServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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
		opts := gofastly.DeleteSumologicInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Fastly Sumologic removal opts: %#v", opts)
		err := conn.DeleteSumologic(&opts)
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
		opts := gofastly.CreateSumologicInput{
			ServiceID:         d.Id(),
			ServiceVersion:    latestVersion,
			Name:              resource["name"].(string),
			URL:               resource["url"].(string),
			MessageType:       resource["message_type"].(string),
			Format:            vla.format,
			FormatVersion:     int(uintOrDefault(vla.formatVersion)),
			ResponseCondition: vla.responseCondition,
			Placement:         vla.placement,
		}

		log.Printf("[DEBUG] Create Sumologic Opts: %#v", opts)
		_, err := conn.CreateSumologic(&opts)
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

		opts := gofastly.UpdateSumologicInput{
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
		if v, ok := modified["url"]; ok {
			opts.URL = gofastly.String(v.(string))
		}
		if v, ok := modified["format"]; ok {
			opts.Format = gofastly.String(v.(string))
		}
		if v, ok := modified["response_condition"]; ok {
			opts.ResponseCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["message_type"]; ok {
			opts.MessageType = gofastly.String(v.(string))
		}
		if v, ok := modified["format_version"]; ok {
			opts.FormatVersion = gofastly.Int(v.(int))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Sumologic Opts: %#v", opts)
		_, err := conn.UpdateSumologic(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *SumologicServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Sumologic for (%s)", d.Id())
	sumologicList, err := conn.ListSumologics(&gofastly.ListSumologicsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Sumologic for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	sul := flattenSumologics(sumologicList)

	for _, element := range sul {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), sul); err != nil {
		log.Printf("[WARN] Error setting Sumologic for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *SumologicServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify this Sumologic endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"url": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The URL to Sumologic collector endpoint",
		},
		// Optional fields
		"message_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Default:          "classic",
			Description:      "How the message should be formatted; one of: `classic`, `loggly`, `logplex` or `blank`. Default `classic`. See [Fastly's Documentation on Sumologic](https://developer.fastly.com/reference/api/logging/sumologic/)",
			ValidateDiagFunc: validateLoggingMessageType(),
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

func flattenSumologics(sumologicList []*gofastly.Sumologic) []map[string]interface{} {
	var l []map[string]interface{}
	for _, p := range sumologicList {
		// Convert Sumologic to a map for saving to state.
		ns := map[string]interface{}{
			"name":               p.Name,
			"url":                p.URL,
			"format":             p.Format,
			"response_condition": p.ResponseCondition,
			"message_type":       p.MessageType,
			"format_version":     int(p.FormatVersion),
			"placement":          p.Placement,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range ns {
			if v == "" {
				delete(ns, k)
			}
		}

		l = append(l, ns)
	}

	return l
}
