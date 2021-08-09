package fastly

import (
	"context"
	"fmt"
	"log"
	"strings"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ConditionServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceCondition(sa ServiceMetadata) ServiceAttributeDefinition {
	return BlockSetToServiceAttributeDefinition(&ConditionServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "condition",
			serviceMetadata: sa,
		},
	})
}

func (h *ConditionServiceAttributeHandler) Key() string { return h.key }

func (h *ConditionServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The unique name for the condition. It is important to note that changing this attribute will delete and recreate the resource",
				},
				"statement": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The statement used to determine if the condition is met",
				},
				"priority": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     10,
					Description: "A number used to determine the order in which multiple conditions execute. Lower numbers execute first. Default `10`",
				},
				"type": {
					Type:             schema.TypeString,
					Required:         true,
					Description:      "Type of condition, either `REQUEST` (req), `RESPONSE` (req, resp), or `CACHE` (req, beresp)",
					ValidateDiagFunc: validateConditionType(),
				},
			},
		},
	}
}

func (h *ConditionServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.CreateConditionInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
		Type:           resource["type"].(string),
		// need to trim leading/tailing spaces, incase the config has HEREDOC
		// formatting and contains a trailing new line
		Statement: strings.TrimSpace(resource["statement"].(string)),
		Priority:  resource["priority"].(int),
	}

	log.Printf("[DEBUG] Create Conditions Opts: %#v", opts)
	_, err := conn.CreateCondition(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ConditionServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Conditions for (%s)", d.Id())
	conditionList, err := conn.ListConditions(&gofastly.ListConditionsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Conditions for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	cl := flattenConditions(conditionList)

	if err := d.Set(h.GetKey(), cl); err != nil {
		log.Printf("[WARN] Error setting Conditions for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *ConditionServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateConditionInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["comment"]; ok {
		opts.Comment = gofastly.String(v.(string))
	}
	if v, ok := modified["statement"]; ok {
		opts.Statement = gofastly.String(v.(string))
	}
	if v, ok := modified["type"]; ok {
		opts.Type = gofastly.String(v.(string))
	}
	if v, ok := modified["priority"]; ok {
		opts.Priority = gofastly.Int(v.(int))
	}

	log.Printf("[DEBUG] Update Condition Opts: %#v", opts)
	_, err := conn.UpdateCondition(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ConditionServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteConditionInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Conditions Removal opts: %#v", opts)
	err := conn.DeleteCondition(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenConditions(conditionList []*gofastly.Condition) []map[string]interface{} {
	var cl []map[string]interface{}
	for _, c := range conditionList {
		// Convert Conditions to a map for saving to state.
		nc := map[string]interface{}{
			"name":      c.Name,
			"statement": c.Statement,
			"type":      c.Type,
			"priority":  c.Priority,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nc {
			if v == "" {
				delete(nc, k)
			}
		}

		cl = append(cl, nc)
	}

	return cl
}
