package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DatadogServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingDatadog(sa ServiceMetadata) ServiceAttributeDefinition {
	return &DatadogServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_datadog",
			serviceMetadata: sa,
		},
	}
}

func (h *DatadogServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	od, nd := d.GetChange(h.GetKey())

	if od == nil {
		od = new(schema.Set)
	}
	if nd == nil {
		nd = new(schema.Set)
	}

	oldSet := od.(*schema.Set)
	newSet := nd.(*schema.Set)

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

		log.Printf("[DEBUG] Fastly Datadog logging endpoint removal opts: %#v", opts)

		if err := deleteDatadog(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Datadog logging addition opts: %#v", opts)

		if err := createDatadog(conn, opts); err != nil {
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

		opts := gofastly.UpdateDatadogInput{
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
		if v, ok := modified["token"]; ok {
			opts.Token = gofastly.String(v.(string))
		}
		if v, ok := modified["region"]; ok {
			opts.Region = gofastly.String(v.(string))
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

		log.Printf("[DEBUG] Update Datadog Opts: %#v", opts)
		_, err := conn.UpdateDatadog(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *DatadogServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh Datadog.
	log.Printf("[DEBUG] Refreshing Datadog logging endpoints for (%s)", d.Id())
	datadogList, err := conn.ListDatadog(&gofastly.ListDatadogInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Datadog logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	dll := flattenDatadog(datadogList)

	for _, element := range dll {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), dll); err != nil {
		log.Printf("[WARN] Error setting Datadog logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createDatadog(conn *gofastly.Client, i *gofastly.CreateDatadogInput) error {
	_, err := conn.CreateDatadog(i)
	return err
}

func deleteDatadog(conn *gofastly.Client, i *gofastly.DeleteDatadogInput) error {
	err := conn.DeleteDatadog(i)

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

func (h *DatadogServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Datadog logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "The API key from your Datadog account",
		},

		// Optional fields
		"region": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "US",
			Description: "The region that log data will be sent to. One of `US` or `EU`. Defaults to `US` if undefined",
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
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of the condition to apply.",
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

func flattenDatadog(datadogList []*gofastly.Datadog) []map[string]interface{} {
	var dsl []map[string]interface{}
	for _, dl := range datadogList {
		// Convert Datadog logging to a map for saving to state.
		ndl := map[string]interface{}{
			"name":               dl.Name,
			"token":              dl.Token,
			"region":             dl.Region,
			"format":             dl.Format,
			"format_version":     dl.FormatVersion,
			"placement":          dl.Placement,
			"response_condition": dl.ResponseCondition,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range ndl {
			if v == "" {
				delete(ndl, k)
			}
		}

		dsl = append(dsl, ndl)
	}

	return dsl
}

func (h *DatadogServiceAttributeHandler) buildCreate(datadogMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateDatadogInput {
	df := datadogMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateDatadogInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Token:             df["token"].(string),
		Region:            df["region"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *DatadogServiceAttributeHandler) buildDelete(datadogMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteDatadogInput {
	df := datadogMap.(map[string]interface{})

	return &gofastly.DeleteDatadogInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
