package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type NewRelicServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingNewRelic(sa ServiceMetadata) ServiceAttributeDefinition {
	return &NewRelicServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_newrelic",
			serviceMetadata: sa,
		},
	}
}

func (h *NewRelicServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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

		log.Printf("[DEBUG] Fastly New Relic logging endpoint removal opts: %#v", opts)

		if err := deleteNewRelic(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly New Relic logging addition opts: %#v", opts)

		if err := createNewRelic(conn, opts); err != nil {
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

		opts := gofastly.UpdateNewRelicInput{
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

		log.Printf("[DEBUG] Update New Relic Opts: %#v", opts)
		_, err := conn.UpdateNewRelic(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *NewRelicServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh NewRelic.
	log.Printf("[DEBUG] Refreshing New Relic logging endpoints for (%s)", d.Id())
	newrelicList, err := conn.ListNewRelic(&gofastly.ListNewRelicInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up New Relic logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	dll := flattenNewRelic(newrelicList)

	for _, element := range dll {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), dll); err != nil {
		log.Printf("[WARN] Error setting New Relic logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createNewRelic(conn *gofastly.Client, i *gofastly.CreateNewRelicInput) error {
	_, err := conn.CreateNewRelic(i)
	return err
}

func deleteNewRelic(conn *gofastly.Client, i *gofastly.DeleteNewRelicInput) error {
	err := conn.DeleteNewRelic(i)

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

func flattenNewRelic(newrelicList []*gofastly.NewRelic) []map[string]interface{} {
	var dsl []map[string]interface{}
	for _, dl := range newrelicList {
		// Convert NewRelic logging to a map for saving to state.
		ndl := map[string]interface{}{
			"name":               dl.Name,
			"token":              dl.Token,
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

func (h *NewRelicServiceAttributeHandler) buildCreate(newrelicMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateNewRelicInput {
	df := newrelicMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateNewRelicInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Token:             df["token"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *NewRelicServiceAttributeHandler) buildDelete(newrelicMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteNewRelicInput {
	df := newrelicMap.(map[string]interface{})

	return &gofastly.DeleteNewRelicInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}

func (h *NewRelicServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the New Relic logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "The Insert API key from the Account page of your New Relic account",
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Apache style log formatting. Your log must produce valid JSON that New Relic Logs can ingest.",
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
