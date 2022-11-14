package fastly

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	gofastly "github.com/fastly/go-fastly/v7/fastly"
	"github.com/fastly/terraform-provider-fastly/fastly/hashcode"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceFastlyDatacenters() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceFastlyDatacentersRead,

		Schema: map[string]*schema.Schema{
			"pops": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "A list of all Fastly POPs.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"code": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "A code representing the POP location.",
						},
						"group": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "A code representing the general region of the world in which the POP location resides.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the POP.",
						},
						"shield": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "A code representing the shielding name of the POP. The value may be empty if the POP is not available for shielding.",
						},
					},
				},
			},
		},
	}
}

func dataSourceFastlyDatacentersRead(_ context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	log.Printf("[DEBUG] Reading datacenters")

	datacenters, err := conn.AllDatacenters()
	if err != nil {
		return diag.Errorf("error fetching datacenters: %s", err)
	}

	hashBase, _ := json.Marshal(datacenters)
	hashString := strconv.Itoa(hashcode.String(string(hashBase)))
	d.SetId(hashString)

	if err := d.Set("pops", flattenDatacenters(datacenters)); err != nil {
		return diag.Errorf("error setting datacenters: %s", err)
	}

	return nil
}

// flattenDatacenters models data into format suitable for saving to Terraform state.
func flattenDatacenters(datacenters []gofastly.Datacenter) []map[string]any {
	result := make([]map[string]any, len(datacenters))
	if len(datacenters) == 0 {
		return result
	}

	for i, resource := range datacenters {
		data := map[string]any{
			"code":   resource.Code,
			"name":   resource.Name,
			"group":  resource.Group,
			"shield": resource.Shield,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range data {
			if v == "" {
				delete(data, k)
			}
		}
		result[i] = data
	}

	return result
}
