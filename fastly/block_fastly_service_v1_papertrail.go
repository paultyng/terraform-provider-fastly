package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type PaperTrailServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServicePaperTrail(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&PaperTrailServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "papertrail",
			serviceMetadata: sa,
		},
	})
}

func (h *PaperTrailServiceAttributeHandler) Key() string { return h.key }

func (h *PaperTrailServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "A unique name to identify this Papertrail endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"address": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The address of the Papertrail endpoint",
		},
		"port": {
			Type:        schema.TypeInt,
			Required:    true,
			Description: "The port associated with the address where the Papertrail endpoint can be accessed",
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "%h %l %u %t %r %>s",
			Description: "A Fastly [log format string](https://docs.fastly.com/en/guides/custom-log-formats)",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          2,
			Description:      "The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "",
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute",
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of 2 are placed in `vcl_log` and those with `format_version` of 1 are placed in `vcl_deliver`",
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

func (h *PaperTrailServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	var vla = h.getVCLLoggingAttributes(resource)

	opts := gofastly.CreatePapertrailInput{
		ServiceID:         d.Id(),
		ServiceVersion:    serviceVersion,
		Name:              resource["name"].(string),
		Address:           resource["address"].(string),
		Port:              uint(resource["port"].(int)),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		ResponseCondition: vla.responseCondition,
		Placement:         vla.placement,
	}

	log.Printf("[DEBUG] Create Papertrail Opts: %#v", opts)
	_, err := conn.CreatePapertrail(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *PaperTrailServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Papertrail for (%s)", d.Id())
	papertrailList, err := conn.ListPapertrails(&gofastly.ListPapertrailsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Papertrail for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	pl := flattenPapertrails(papertrailList)

	for _, element := range pl {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), pl); err != nil {
		log.Printf("[WARN] Error setting Papertrail for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *PaperTrailServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdatePapertrailInput{
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
	if v, ok := modified["port"]; ok {
		opts.Port = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Papertrail Opts: %#v", opts)
	_, err := conn.UpdatePapertrail(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *PaperTrailServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeletePapertrailInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Papertrail removal opts: %#v", opts)
	err := conn.DeletePapertrail(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenPapertrails(papertrailList []*gofastly.Papertrail) []map[string]interface{} {
	var pl []map[string]interface{}
	for _, p := range papertrailList {
		// Convert Papertrails to a map for saving to state.
		ns := map[string]interface{}{
			"name":               p.Name,
			"address":            p.Address,
			"port":               p.Port,
			"format":             p.Format,
			"format_version":     p.FormatVersion,
			"response_condition": p.ResponseCondition,
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
