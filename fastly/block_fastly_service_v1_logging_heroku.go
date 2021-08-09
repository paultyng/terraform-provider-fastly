package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type HerokuServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingHeroku(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&HerokuServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_heroku",
			serviceMetadata: sa,
		},
	})
}

func (h *HerokuServiceAttributeHandler) Key() string { return h.key }

func (h *HerokuServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Heroku logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"token": {
			Type:        schema.TypeString,
			Required:    true,
			Sensitive:   true,
			Description: "The token to use for authentication (https://www.heroku.com/docs/customer-token-authentication-token/)",
		},

		"url": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The URL to stream logs to",
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

	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
}

func (h *HerokuServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildCreate(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Heroku logging addition opts: %#v", opts)

	if err := createHeroku(conn, opts); err != nil {
		return err
	}
	return nil
}

func (h *HerokuServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// Refresh Heroku.
	log.Printf("[DEBUG] Refreshing Heroku logging endpoints for (%s)", d.Id())
	herokuList, err := conn.ListHerokus(&gofastly.ListHerokusInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Heroku logging endpoints for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	ell := flattenHeroku(herokuList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting Heroku logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *HerokuServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateHerokuInput{
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
	if v, ok := modified["url"]; ok {
		opts.URL = gofastly.String(v.(string))
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

	log.Printf("[DEBUG] Update Heroku Opts: %#v", opts)
	_, err := conn.UpdateHeroku(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *HerokuServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Heroku logging endpoint removal opts: %#v", opts)

	if err := deleteHeroku(conn, opts); err != nil {
		return err
	}
	return nil
}

func createHeroku(conn *gofastly.Client, i *gofastly.CreateHerokuInput) error {
	_, err := conn.CreateHeroku(i)
	return err
}

func deleteHeroku(conn *gofastly.Client, i *gofastly.DeleteHerokuInput) error {
	err := conn.DeleteHeroku(i)

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

func flattenHeroku(herokuList []*gofastly.Heroku) []map[string]interface{} {
	var res []map[string]interface{}
	for _, ll := range herokuList {
		// Convert Heroku logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"token":              ll.Token,
			"url":                ll.URL,
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

		res = append(res, nll)
	}

	return res
}

func (h *HerokuServiceAttributeHandler) buildCreate(herokuMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateHerokuInput {
	df := herokuMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateHerokuInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		Token:             df["token"].(string),
		URL:               df["url"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *HerokuServiceAttributeHandler) buildDelete(herokuMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteHerokuInput {
	df := herokuMap.(map[string]interface{})

	return &gofastly.DeleteHerokuInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
