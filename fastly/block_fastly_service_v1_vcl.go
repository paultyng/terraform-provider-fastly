package fastly

import (
	"context"
	"errors"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type VCLServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceVCL(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&VCLServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "vcl",
			serviceMetadata: sa,
		},
	})
}

func (h *VCLServiceAttributeHandler) Key() string { return h.key }

func (h *VCLServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "A unique name for this configuration block. It is important to note that changing this attribute will delete and recreate the resource",
				},
				"content": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The custom VCL code to upload",
				},
				"main": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
					Description: "If `true`, use this block as the main configuration. If `false`, use this block as an includable library. Only a single VCL block can be marked as the main block. Default is `false`",
				},
			},
		},
	}
}

func (h *VCLServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.CreateVCLInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
		Content:        resource["content"].(string),
		Main:           resource["main"].(bool),
	}

	log.Printf("[DEBUG] Fastly VCL Addition opts: %#v", opts)
	_, err := conn.CreateVCL(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *VCLServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing VCLs for (%s)", d.Id())
	vclList, err := conn.ListVCLs(&gofastly.ListVCLsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCLs for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	vl := flattenVCLs(vclList)

	if err := d.Set(h.GetKey(), vl); err != nil {
		log.Printf("[WARN] Error setting VCLs for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *VCLServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateVCLInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	if v, ok := modified["content"]; ok {
		opts.Content = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update VCL Opts: %#v", opts)
	_, err := conn.UpdateVCL(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *VCLServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteVCLInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly VCL Removal opts: %#v", opts)
	err := conn.DeleteVCL(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenVCLs(vclList []*gofastly.VCL) []map[string]interface{} {
	var vl []map[string]interface{}
	for _, vcl := range vclList {
		// Convert VCLs to a map for saving to state.
		vclMap := map[string]interface{}{
			"name":    vcl.Name,
			"content": vcl.Content,
			"main":    vcl.Main,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range vclMap {
			if v == "" {
				delete(vclMap, k)
			}
		}

		vl = append(vl, vclMap)
	}

	return vl
}

func validateVCLs(d *schema.ResourceData) error {
	// TODO: this would be nice to move into a resource/collection validation function, once that is available
	// (see https://github.com/hashicorp/terraform/pull/4348 and https://github.com/hashicorp/terraform/pull/6508)
	vcls, exists := d.GetOk("vcl")
	if !exists {
		return nil
	}

	numberOfMainVCLs, numberOfIncludeVCLs := 0, 0
	for _, vclElem := range vcls.(*schema.Set).List() {
		vcl := vclElem.(map[string]interface{})
		if mainVal, hasMain := vcl["main"]; hasMain && mainVal.(bool) {
			numberOfMainVCLs++
		} else {
			numberOfIncludeVCLs++
		}
	}
	if numberOfMainVCLs == 0 && numberOfIncludeVCLs > 0 {
		return errors.New("if you include VCL configurations, one of them should have main = true")
	}
	if numberOfMainVCLs > 1 {
		return errors.New("you cannot have more than one VCL configuration with main = true")
	}
	return nil
}
