package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DictionaryServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceDictionary(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&DictionaryServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "dictionary",
			serviceMetadata: sa,
		},
	})
}

func (h *DictionaryServiceAttributeHandler) Key() string { return h.key }

func (h *DictionaryServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				// Required fields
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "A unique name to identify this dictionary. It is important to note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary",
				},
				// Optional fields
				"dictionary_id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "The ID of the dictionary",
				},
				"write_only": {
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
					Description: "If `true`, the dictionary is a private dictionary, and items are not readable in the UI or via API. Default is `false`. It is important to note that changing this attribute will delete and recreate the dictionary, and discard the current items in the dictionary. Using a write-only/private dictionary should only be done if the items are managed outside of Terraform",
				},
				"force_destroy": {
					Type:        schema.TypeBool,
					Default:     false,
					Optional:    true,
					Description: "Allow the dictionary to be deleted, even if it contains entries. Defaults to false.",
				},
			},
		},
	}
}

func (h *DictionaryServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
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

func (h *DictionaryServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Dictionaries for (%s)", d.Id())
	dictList, err := conn.ListDictionaries(&gofastly.ListDictionariesInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Dictionaries for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	dictionaries := flattenDictionaries(dictList)

	// Match up force_destroy on each ACL from schema.ResourceData to avoid d.Set overwriting it with null
	stateDicts := d.Get(h.GetKey()).(*schema.Set).List()
	for _, dictionary := range dictionaries {
		for _, sd := range stateDicts {
			stateDict := sd.(map[string]interface{})
			if dictionary["name"] == stateDict["name"] {
				dictionary["force_destroy"] = stateDict["force_destroy"]
				break
			}
		}
	}

	if err := d.Set(h.GetKey(), dictionaries); err != nil {
		log.Printf("[WARN] Error setting Dictionary for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *DictionaryServiceAttributeHandler) Update(_ context.Context, _ *schema.ResourceData, _, _ map[string]interface {
}, _ int, _ *gofastly.Client) error {
	return nil
}

func (h *DictionaryServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	if !resource["force_destroy"].(bool) {
		mayDelete, err := isDictionaryEmpty(d.Id(), resource["dictionary_id"].(string), conn)
		if err != nil {
			return err
		}

		if !mayDelete {
			return fmt.Errorf("Cannot delete dictionary (%s), it is not empty. Either delete the items first, or set force_destroy to true and apply it before making this change.", resource["dictionary_id"].(string))
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

func flattenDictionaries(dictList []*gofastly.Dictionary) []map[string]interface{} {
	var dl []map[string]interface{}
	for _, currentDict := range dictList {

		dictMapString := map[string]interface{}{
			"dictionary_id": currentDict.ID,
			"name":          currentDict.Name,
			"write_only":    currentDict.WriteOnly,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range dictMapString {
			if v == "" {
				delete(dictMapString, k)
			}
		}

		dl = append(dl, dictMapString)
	}

	return dl
}

func buildDictionary(dictMap interface{}) (*gofastly.CreateDictionaryInput, error) {
	df := dictMap.(map[string]interface{})
	opts := gofastly.CreateDictionaryInput{
		Name:      df["name"].(string),
		WriteOnly: gofastly.Compatibool(df["write_only"].(bool)),
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
