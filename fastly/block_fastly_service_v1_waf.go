package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type WAFServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceWAF(sa ServiceMetadata) ServiceAttributeDefinition {
	return &WAFServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "waf",
			serviceMetadata: sa,
		},
	}
}

func (h *WAFServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"response_object": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The name of the response object used by the Web Application Firewall",
				},
				"prefetch_condition": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "The `condition` to determine which requests will be run past your Fastly WAF. This `condition` must be of type `PREFETCH`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)",
				},
				"waf_id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "The ID of the WAF",
				},
				"disabled": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
					Description: "A flag used to completely disable a Web Application Firewall. This is intended to only be used in an emergency",
				},
			},
		},
	}

	return nil
}

func (h *WAFServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, serviceVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	oldWAFVal, newWAFVal := d.GetChange(h.GetKey())

	if len(newWAFVal.([]interface{})) == 1 {
		wf := newWAFVal.([]interface{})[0].(map[string]interface{})

		var err error
		if wafExists(conn, serviceID, serviceVersion, wf["waf_id"].(string)) {
			opts := buildUpdateWAF(d, wf, serviceID, serviceVersion)
			log.Printf("[DEBUG] Fastly WAF update opts: %#v", opts)
			_, err = conn.UpdateWAF(opts)
		} else {
			opts := buildCreateWAF(wf, serviceID, serviceVersion)
			log.Printf("[DEBUG] Fastly WAF Addition opts: %#v", opts)

			_, err = conn.CreateWAF(opts)
		}
		if err != nil {
			return err
		}
	} else if len(oldWAFVal.([]interface{})) > 0 {

		wf := oldWAFVal.([]interface{})[0].(map[string]interface{})

		opts := buildDeleteWAF(wf, serviceVersion)
		log.Printf("[DEBUG] Fastly WAF Removal opts: %#v", opts)
		err := conn.DeleteWAF(opts)
		if errRes, ok := err.(*gofastly.HTTPError); ok {
			if errRes.StatusCode != 404 {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (h *WAFServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// refresh WAFs
	log.Printf("[DEBUG] Refreshing WAFs for (%s)", d.Id())
	wafList, err := conn.ListWAFs(&gofastly.ListWAFsInput{
		FilterService: d.Id(),
		FilterVersion: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up WAFs for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	waf := flattenWAFs(wafList.Items)

	if err := d.Set("waf", waf); err != nil {
		log.Printf("[WARN] Error setting waf for (%s): %s", d.Id(), err)
	}
	return nil
}

func wafExists(conn *gofastly.Client, s string, v int, id string) bool {
	_, err := conn.GetWAF(&gofastly.GetWAFInput{
		ServiceID:      s,
		ServiceVersion: v,
		ID:             id,
	})
	if err != nil {
		return false
	}
	return true
}

func flattenWAFs(wafList []*gofastly.WAF) []map[string]interface{} {

	var wl []map[string]interface{}
	if len(wafList) == 0 {
		return wl
	}

	w := wafList[0]
	WAFMapString := map[string]interface{}{
		"waf_id":             w.ID,
		"response_object":    w.Response,
		"prefetch_condition": w.PrefetchCondition,
	}

	// prune any empty values that come from the default string value in structs
	for k, v := range WAFMapString {
		if v == "" {
			delete(WAFMapString, k)
		}
	}
	return append(wl, WAFMapString)
}

func buildCreateWAF(WAFMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateWAFInput {
	df := WAFMap.(map[string]interface{})

	opts := gofastly.CreateWAFInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		ID:                df["waf_id"].(string),
		PrefetchCondition: df["prefetch_condition"].(string),
		Response:          df["response_object"].(string),
	}
	return &opts
}

func buildDeleteWAF(WAFMap interface{}, serviceVersion int) *gofastly.DeleteWAFInput {
	df := WAFMap.(map[string]interface{})

	opts := gofastly.DeleteWAFInput{
		ID:             df["waf_id"].(string),
		ServiceVersion: serviceVersion,
	}
	return &opts
}

func buildUpdateWAF(d *schema.ResourceData, wafMap interface{}, serviceID string, serviceVersion int) *gofastly.UpdateWAFInput {
	df := wafMap.(map[string]interface{})

	input := gofastly.UpdateWAFInput{
		ServiceID:      gofastly.String(serviceID),
		ServiceVersion: gofastly.Int(serviceVersion),
		ID:             df["waf_id"].(string),
	}

	// NOTE: to access the WAF data we need to link to a specific list index.
	// This is because the schema defines the service as being of TypeList.
	//
	// Although there should only ever be one service (hence MaxItems: 1) we are
	// unable to change the schema to a TypeMap as that would contrain the map's
	// value to a single type (e.g. TypeString, TypeBool, TypeInt, or TypeFloat).

	if v, ok := d.GetOk("waf.0.prefetch_condition"); ok {
		input.PrefetchCondition = gofastly.String(v.(string))
	}
	if v, ok := d.GetOk("waf.0.response_object"); ok {
		input.Response = gofastly.String(v.(string))
	}
	if v, ok := d.GetOk("waf.0.disabled"); ok {
		input.Disabled = gofastly.Bool(v.(bool))
	}

	return &input
}
