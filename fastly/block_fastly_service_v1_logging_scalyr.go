package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ScalyrServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingScalyr(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&ScalyrServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_scalyr",
			serviceMetadata: sa,
		},
	})
}

func (h *ScalyrServiceAttributeHandler) Key() string { return h.key }

func (h *ScalyrServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Scalyr logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The token to use for authentication (https://www.scalyr.com/keys)",
			Sensitive:   true,
		},

		// Optional
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
			Description: "Apache style log formatting.",
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
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
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

func (h *ScalyrServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildCreate(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Scalyr logging addition opts: %#v", opts)

	if err := createScalyr(conn, opts); err != nil {
		return err
	}
	return nil
}

func (h *ScalyrServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// Refresh Scalyr.
	log.Printf("[DEBUG] Refreshing Scalyr logging endpoints for (%s)", d.Id())
	scalyrList, err := conn.ListScalyrs(&gofastly.ListScalyrsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Scalyr logging endpoints for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	scalyrLogList := flattenScalyr(scalyrList)

	for _, element := range scalyrLogList {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), scalyrLogList); err != nil {
		log.Printf("[WARN] Error setting Scalyr logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *ScalyrServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateScalyrInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

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
	if v, ok := modified["token"]; ok {
		opts.Token = gofastly.String(v.(string))
	}
	if v, ok := modified["region"]; ok {
		opts.Region = gofastly.String(v.(string))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Scalyr Opts: %#v", opts)
	_, err := conn.UpdateScalyr(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ScalyrServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Scalyr logging endpoint removal opts: %#v", opts)

	if err := deleteScalyr(conn, opts); err != nil {
		return err
	}
	return nil
}

func createScalyr(conn *gofastly.Client, i *gofastly.CreateScalyrInput) error {
	_, err := conn.CreateScalyr(i)
	return err
}

func deleteScalyr(conn *gofastly.Client, i *gofastly.DeleteScalyrInput) error {
	err := conn.DeleteScalyr(i)

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

func flattenScalyr(scalyrList []*gofastly.Scalyr) []map[string]interface{} {
	var flattened []map[string]interface{}
	for _, s := range scalyrList {
		// Convert logging to a map for saving to state.
		flatScalyr := map[string]interface{}{
			"name":               s.Name,
			"region":             s.Region,
			"token":              s.Token,
			"response_condition": s.ResponseCondition,
			"format":             s.Format,
			"placement":          s.Placement,
			"format_version":     s.FormatVersion,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range flatScalyr {
			if v == "" {
				delete(flatScalyr, k)
			}
		}

		flattened = append(flattened, flatScalyr)
	}

	return flattened
}

func (h *ScalyrServiceAttributeHandler) buildCreate(scalyrMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateScalyrInput {
	df := scalyrMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateScalyrInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Region:            df["region"].(string),
		Token:             df["token"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *ScalyrServiceAttributeHandler) buildDelete(scalyrMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteScalyrInput {
	df := scalyrMap.(map[string]interface{})

	return &gofastly.DeleteScalyrInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
