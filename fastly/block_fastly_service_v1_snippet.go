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
	return BlockSetToServiceAttributeDefinition(&SnippetServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "snippet",
			serviceMetadata: sa,
		},
	})
}

func (h *SnippetServiceAttributeHandler) Key() string { return h.key }

func (h *SnippetServiceAttributeHandler) GetSchema() *schema.Schema {
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
}

func (h *SnippetServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts, err := buildSnippet(resource)
	if err != nil {
		log.Printf("[DEBUG] Error building VCL Snippet: %s", err)
		return err
	}
	opts.ServiceID = d.Id()
	opts.ServiceVersion = serviceVersion

	log.Printf("[DEBUG] Fastly VCL Snippet Addition opts: %#v", opts)
	_, err = conn.CreateSnippet(opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *SnippetServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing VCL Snippets for (%s)", d.Id())
	snippetList, err := conn.ListSnippets(&gofastly.ListSnippetsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})
	if err != nil {
		return fmt.Errorf("[ERR] Error looking up VCL Snippets for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	vsl := flattenSnippets(snippetList)

	if err := d.Set(h.GetKey(), vsl); err != nil {
		log.Printf("[WARN] Error setting VCL Snippets for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *SnippetServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	// Safety check in case keys aren't actually set in the HCL.
	name, _ := resource["name"].(string)
	priority, _ := resource["priority"].(int)
	dynamic, _ := resource["dynamic"].(int)
	content, _ := resource["content"].(string)
	stype, _ := resource["type"].(string)

	opts := gofastly.UpdateSnippetInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           name,
		NewName:        name,
		Priority:       priority,
		Dynamic:        dynamic,
		Content:        content,
		Type:           gofastly.SnippetType(stype),
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
	return nil
}

func (h *SnippetServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteSnippetInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
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
