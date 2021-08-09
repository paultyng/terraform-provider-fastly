package fastly

import (
	"context"
	"fmt"
	"log"
	"strings"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DynamicSnippetServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceDynamicSnippet(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&DynamicSnippetServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "dynamicsnippet",
			serviceMetadata: sa,
		},
	})
}

func (h *DynamicSnippetServiceAttributeHandler) Key() string { return h.key }

func (h *DynamicSnippetServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: `A name that is unique across "regular" and "dynamic" VCL Snippet configuration blocks. It is important to note that changing this attribute will delete and recreate the resource`,
				},
				"type": {
					Type:             schema.TypeString,
					Required:         true,
					Description:      "The location in generated VCL where the snippet should be placed (can be one of `init`, `recv`, `hit`, `miss`, `pass`, `fetch`, `error`, `deliver`, `log` or `none`)",
					ValidateDiagFunc: validateSnippetType(),
				},
				"priority": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     100,
					Description: "Priority determines the ordering for multiple snippets. Lower numbers execute first. Defaults to `100`",
				},
				"snippet_id": {
					Type:        schema.TypeString,
					Computed:    true,
					Description: "The ID of the dynamic snippet",
				},
			},
		},
	}
}

func (h *DynamicSnippetServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts, err := buildDynamicSnippet(resource)
	if err != nil {
		log.Printf("[DEBUG] Error building VCL Dynamic Snippet: %s", err)
		return err
	}
	opts.ServiceID = d.Id()
	opts.ServiceVersion = serviceVersion

	log.Printf("[DEBUG] Fastly VCL Dynamic Snippet Addition opts: %#v", opts)
	_, err = conn.CreateSnippet(opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *DynamicSnippetServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&gofastly.ListSnippetsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	dynamicSnippets := flattenDynamicSnippets(snippetList)
	if err := d.Set(h.GetKey(), dynamicSnippets); err != nil {
		log.Printf("[WARN] Error setting VCL Dynamic Snippets for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *DynamicSnippetServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateSnippetInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["priority"]; ok {
		opts.Priority = v.(int)
	}
	if v, ok := modified["dynamic"]; ok {
		opts.Dynamic = v.(int)
	}
	if v, ok := modified["content"]; ok {
		opts.Content = v.(string)
	}
	if v, ok := modified["type"]; ok {
		opts.Type = v.(gofastly.SnippetType)
	}

	log.Printf("[DEBUG] Update Dynamic Snippet Opts: %#v", opts)
	_, err := conn.UpdateSnippet(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *DynamicSnippetServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteSnippetInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly VCL Dynamic Snippet Removal opts: %#v", opts)
	err := conn.DeleteSnippet(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func buildDynamicSnippet(dynamicSnippetMap interface{}) (*gofastly.CreateSnippetInput, error) {
	df := dynamicSnippetMap.(map[string]interface{})
	opts := gofastly.CreateSnippetInput{
		Name:     df["name"].(string),
		Priority: df["priority"].(int),
		Dynamic:  1,
	}

	snippetType := strings.ToLower(df["type"].(string))
	switch snippetType {
	case "init":
		opts.Type = gofastly.SnippetTypeInit
	case "recv":
		opts.Type = gofastly.SnippetTypeRecv
	case "hash":
		opts.Type = gofastly.SnippetTypeHash
	case "hit":
		opts.Type = gofastly.SnippetTypeHit
	case "miss":
		opts.Type = gofastly.SnippetTypeMiss
	case "pass":
		opts.Type = gofastly.SnippetTypePass
	case "fetch":
		opts.Type = gofastly.SnippetTypeFetch
	case "error":
		opts.Type = gofastly.SnippetTypeError
	case "deliver":
		opts.Type = gofastly.SnippetTypeDeliver
	case "log":
		opts.Type = gofastly.SnippetTypeLog
	case "none":
		opts.Type = gofastly.SnippetTypeNone
	}

	return &opts, nil
}

func flattenDynamicSnippets(dynamicSnippetList []*gofastly.Snippet) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, dynamicSnippet := range dynamicSnippetList {
		// Skip non-dynamic snippets
		if dynamicSnippet.Dynamic == 0 {
			continue
		}

		// Convert VCLs to a map for saving to state.
		dynamicSnippetMap := map[string]interface{}{
			"snippet_id": dynamicSnippet.ID,
			"name":       dynamicSnippet.Name,
			"type":       dynamicSnippet.Type,
			"priority":   int(dynamicSnippet.Priority),
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range dynamicSnippetMap {
			if v == "" {
				delete(dynamicSnippetMap, k)
			}
		}

		sl = append(sl, dynamicSnippetMap)
	}

	return sl
}
