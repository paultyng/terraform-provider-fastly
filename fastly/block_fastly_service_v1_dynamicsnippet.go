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
	return &DynamicSnippetServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "dynamicsnippet",
			serviceMetadata: sa,
		},
	}
}

func (h *DynamicSnippetServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	// Note: as above with Gzip and S3 logging, we don't utilize the PUT
	// endpoint to update a VCL dynamic snippet, we simply destroy it and create a new one.
	oldDynamicSnippetVal, newDynamicSnippetVal := d.GetChange(h.GetKey())
	if oldDynamicSnippetVal == nil {
		oldDynamicSnippetVal = new(schema.Set)
	}
	if newDynamicSnippetVal == nil {
		newDynamicSnippetVal = new(schema.Set)
	}

	oldSet := oldDynamicSnippetVal.(*schema.Set)
	newSet := newDynamicSnippetVal.(*schema.Set)

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
		opts := gofastly.DeleteSnippetInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
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
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		opts, err := buildDynamicSnippet(resource.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building VCL Dynamic Snippet: %s", err)
			return err
		}
		opts.ServiceID = d.Id()
		opts.ServiceVersion = latestVersion

		log.Printf("[DEBUG] Fastly VCL Dynamic Snippet Addition opts: %#v", opts)
		_, err = conn.CreateSnippet(opts)
		if err != nil {
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

		opts := gofastly.UpdateSnippetInput{
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
	}

	return nil
}

func (h *DynamicSnippetServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&gofastly.ListSnippetsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	dynamicSnippets := flattenDynamicSnippets(snippetList)
	if err := d.Set(h.GetKey(), dynamicSnippets); err != nil {
		log.Printf("[WARN] Error setting VCL Dynamic Snippets for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *DynamicSnippetServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
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
