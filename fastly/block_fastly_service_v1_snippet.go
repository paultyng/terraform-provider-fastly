package fastly

import (
	"context"
	"fmt"
	"log"
	"strings"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SnippetServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceSnippet(sa ServiceMetadata) ServiceAttributeDefinition {
	return &SnippetServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "snippet",
			serviceMetadata: sa,
		},
	}
}

func (h *SnippetServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	// Note: as above with Gzip and S3 logging, we don't utilize the PUT
	// endpoint to update a VCL snippet, we simply destroy it and create a new one.
	oldSnippetVal, newSnippetVal := d.GetChange(h.GetKey())
	if oldSnippetVal == nil {
		oldSnippetVal = new(schema.Set)
	}
	if newSnippetVal == nil {
		newSnippetVal = new(schema.Set)
	}

	oldSet := oldSnippetVal.(*schema.Set)
	newSet := newSnippetVal.(*schema.Set)

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

		log.Printf("[DEBUG] Fastly VCL Snippet Removal opts: %#v", opts)
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
		opts, err := buildSnippet(resource.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building VCL Snippet: %s", err)
			return err
		}
		opts.ServiceID = d.Id()
		opts.ServiceVersion = latestVersion

		log.Printf("[DEBUG] Fastly VCL Snippet Addition opts: %#v", opts)
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

		// Safety check in case keys aren't actually set in the HCL.
		name, _ := resource["name"].(string)
		priority, _ := resource["priority"].(int)
		dynamic, _ := resource["dynamic"].(int)
		content, _ := resource["content"].(string)
		stype, _ := resource["type"].(string)

		opts := gofastly.UpdateSnippetInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           name,
			NewName:        name,
			Priority:       priority,
			Dynamic:        dynamic,
			Content:        content,
			Type:           gofastly.SnippetType(stype),
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
			snippetType := strings.ToLower(v.(string))
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
		}

		log.Printf("[DEBUG] Update VCL Snippet Opts: %#v", opts)
		_, err := conn.UpdateSnippet(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *SnippetServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&gofastly.ListSnippetsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	vsl := flattenSnippets(snippetList)

	if err := d.Set(h.GetKey(), vsl); err != nil {
		log.Printf("[WARN] Error setting VCL Snippets for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *SnippetServiceAttributeHandler) Register(s *schema.Resource) error {
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
				"content": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The VCL code that specifies exactly what the snippet does",
				},
				"priority": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     100,
					Description: "Priority determines the ordering for multiple snippets. Lower numbers execute first. Defaults to `100`",
				},
			},
		},
	}
	return nil
}

func buildSnippet(snippetMap interface{}) (*gofastly.CreateSnippetInput, error) {
	df := snippetMap.(map[string]interface{})
	opts := gofastly.CreateSnippetInput{
		Name:     df["name"].(string),
		Content:  df["content"].(string),
		Priority: df["priority"].(int),
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

func flattenSnippets(snippetList []*gofastly.Snippet) []map[string]interface{} {
	var sl []map[string]interface{}
	for _, snippet := range snippetList {
		// Skip dynamic snippets
		if snippet.Dynamic == 1 {
			continue
		}

		// Convert VCLs to a map for saving to state.
		snippetMap := map[string]interface{}{
			"name":     snippet.Name,
			"type":     snippet.Type,
			"priority": int(snippet.Priority),
			"content":  snippet.Content,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range snippetMap {
			if v == "" {
				delete(snippetMap, k)
			}
		}

		sl = append(sl, snippetMap)
	}

	return sl
}
