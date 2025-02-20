package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v9/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// DictionaryServiceAttributeHandler provides a base implementation for ServiceAttributeDefinition.
type DictionaryServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

// NewServiceDictionary returns a new resource.
func NewServiceDictionary(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&DictionaryServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "dictionary",
			serviceMetadata: sa,
		},
	})
}

// Key returns the resource key.
func (h *DictionaryServiceAttributeHandler) Key() string {
	return h.key
}

// GetSchema returns the resource schema.
func (h *DictionaryServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"dictionary_id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "The ID of the dictionary",
				},
				"force_destroy": {
					Type:        schema.TypeBool,
					Default:     false,
					Optional:    true,
					Description: "Allow the dictionary to be deleted, even if it contains entries. Defaults to false.",
				},
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "A unique name to identify this dictionary. It is important to note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary",
				},
				"write_only": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
					Description: "If `true`, the dictionary is a [private dictionary](https://docs.fastly.com/en/guides/private-dictionaries). Default is `false`. Please note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary. `fastly_service_vcl` resource will only manage the dictionary object itself, and items under private dictionaries can not be managed using [`fastly_service_dictionary_items`](https://registry.terraform.io/providers/fastly/fastly/latest/docs/resources/service_dictionary_items#limitations) resource. Therefore, using a write-only/private dictionary should only be done if the items are managed outside of Terraform",
				},
			},
		},
	}
}

// Create creates the resource.
func (h *DictionaryServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]any, serviceVersion int, conn *gofastly.Client) error {
	opts, err := buildDictionary(resource)
	if err != nil {
		log.Printf("[DEBUG] Error building Dicitionary: %s", err)
		return err
	}
	opts.ServiceID = d.Id()
	opts.ServiceVersion = serviceVersion

	log.Printf("[DEBUG] Fastly Dictionary Addition opts: %#v", opts)
	_, err = conn.CreateDictionary(opts)
	if err != nil {
		return err
	}
	return nil
}

// Read refreshes the resource.
func (h *DictionaryServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]any, serviceVersion int, conn *gofastly.Client) error {
	localState := d.Get(h.GetKey()).(*schema.Set).List()

	if len(localState) > 0 || d.Get("imported").(bool) || d.Get("force_refresh").(bool) {
		log.Printf("[DEBUG] Refreshing Dictionaries for (%s)", d.Id())
		remoteState, err := conn.ListDictionaries(&gofastly.ListDictionariesInput{
			ServiceID:      d.Id(),
			ServiceVersion: serviceVersion,
		})
		if err != nil {
			return fmt.Errorf("error looking up Dictionaries for (%s), version (%v): %s", d.Id(), serviceVersion, err)
		}

		dictionaries := flattenDictionaries(remoteState)

		// Match up force_destroy on each ACL from schema.ResourceData to avoid d.Set overwriting it with null
		stateDicts := d.Get(h.GetKey()).(*schema.Set).List()
		for _, dictionary := range dictionaries {
			for _, sd := range stateDicts {
				stateDict := sd.(map[string]any)
				if dictionary["name"] == stateDict["name"] {
					dictionary["force_destroy"] = stateDict["force_destroy"]
					break
				}
			}
		}

		if err := d.Set(h.GetKey(), dictionaries); err != nil {
			log.Printf("[WARN] Error setting Dictionary for (%s): %s", d.Id(), err)
		}
	}

	return nil
}

// Update updates the resource.
func (h *DictionaryServiceAttributeHandler) Update(_ context.Context, _ *schema.ResourceData, _, _ map[string]any, _ int, _ *gofastly.Client) error {
	return nil
}

// Delete deletes the resource.
func (h *DictionaryServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]any, serviceVersion int, conn *gofastly.Client) error {
	if !resource["force_destroy"].(bool) {
		if resource["write_only"].(bool) {
			return fmt.Errorf("cannot delete dictionary (%s), it is write_only, so it may contain data. Set force_destroy to true and apply it before making this change", resource["dictionary_id"].(string))
		}

		mayDelete, err := isDictionaryEmpty(d.Id(), resource["dictionary_id"].(string), conn)
		if err != nil {
			return err
		}

		if !mayDelete {
			return fmt.Errorf("cannot delete dictionary (%s), it is not empty. Either delete the items first, or set force_destroy to true and apply it before making this change", resource["dictionary_id"].(string))
		}
	}

	opts := gofastly.DeleteDictionaryInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Dictionary Removal opts: %#v", opts)
	err := conn.DeleteDictionary(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

// flattenDictionaries models data into format suitable for saving to Terraform state.
func flattenDictionaries(remoteState []*gofastly.Dictionary) []map[string]any {
	var result []map[string]any
	for _, resource := range remoteState {
		data := map[string]any{}

		if resource.DictionaryID != nil {
			data["dictionary_id"] = *resource.DictionaryID
		}
		if resource.Name != nil {
			data["name"] = *resource.Name
		}
		if resource.WriteOnly != nil {
			data["write_only"] = *resource.WriteOnly
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range data {
			if v == "" {
				delete(data, k)
			}
		}

		result = append(result, data)
	}

	return result
}

func buildDictionary(dictMap any) (*gofastly.CreateDictionaryInput, error) {
	resource := dictMap.(map[string]any)
	opts := gofastly.CreateDictionaryInput{
		Name:      gofastly.ToPointer(resource["name"].(string)),
		WriteOnly: gofastly.ToPointer(gofastly.Compatibool(resource["write_only"].(bool))),
	}

	return &opts, nil
}

func isDictionaryEmpty(serviceID, dictID string, conn *gofastly.Client) (bool, error) {
	items, err := conn.ListDictionaryItems(&gofastly.ListDictionaryItemsInput{
		ServiceID:    serviceID,
		DictionaryID: dictID,
	})
	if err != nil {
		return false, err
	}

	return len(items) == 0, nil
}
