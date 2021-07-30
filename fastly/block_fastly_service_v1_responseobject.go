package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ResponseObjectServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceResponseObject(sa ServiceMetadata) ServiceAttributeDefinition {
	return &ResponseObjectServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "response_object",
			serviceMetadata: sa,
		},
	}
}

func (h *ResponseObjectServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	or, nr := d.GetChange(h.GetKey())
	if or == nil {
		or = new(schema.Set)
	}
	if nr == nil {
		nr = new(schema.Set)
	}

	oldSet := or.(*schema.Set)
	newSet := nr.(*schema.Set)

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
		opts := gofastly.DeleteResponseObjectInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Fastly Response Object removal opts: %#v", opts)
		err := conn.DeleteResponseObject(&opts)
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
		resource := resource.(map[string]interface{})

		opts := gofastly.CreateResponseObjectInput{
			ServiceID:        d.Id(),
			ServiceVersion:   latestVersion,
			Name:             resource["name"].(string),
			Status:           uint(resource["status"].(int)),
			Response:         resource["response"].(string),
			Content:          resource["content"].(string),
			ContentType:      resource["content_type"].(string),
			RequestCondition: resource["request_condition"].(string),
			CacheCondition:   resource["cache_condition"].(string),
		}

		log.Printf("[DEBUG] Create Response Object Opts: %#v", opts)
		_, err := conn.CreateResponseObject(&opts)
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

		opts := gofastly.UpdateResponseObjectInput{
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
		if v, ok := modified["status"]; ok {
			opts.Status = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["response"]; ok {
			opts.Response = gofastly.String(v.(string))
		}
		if v, ok := modified["content"]; ok {
			opts.Content = gofastly.String(v.(string))
		}
		if v, ok := modified["content_type"]; ok {
			opts.ContentType = gofastly.String(v.(string))
		}
		if v, ok := modified["request_condition"]; ok {
			opts.RequestCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["cache_condition"]; ok {
			opts.CacheCondition = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Response Object Opts: %#v", opts)
		_, err := conn.UpdateResponseObject(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *ResponseObjectServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Response Object for (%s)", d.Id())
	responseObjectList, err := conn.ListResponseObjects(&gofastly.ListResponseObjectsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Response Object for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	rol := flattenResponseObjects(responseObjectList)

	if err := d.Set(h.GetKey(), rol); err != nil {
		log.Printf("[WARN] Error setting Response Object for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *ResponseObjectServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				// Required
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "A unique name to identify this Response Object. It is important to note that changing this attribute will delete and recreate the resource",
				},
				// Optional fields
				"status": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     200,
					Description: "The HTTP Status Code. Default `200`",
				},
				"response": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "OK",
					Description: "The HTTP Response. Default `OK`",
				},
				"content": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "The content to deliver for the response object",
				},
				"content_type": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "The MIME type of the content",
				},
				"request_condition": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "Name of already defined `condition` to be checked during the request phase. If the condition passes then this object will be delivered. This `condition` must be of type `REQUEST`",
				},
				"cache_condition": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "Name of already defined `condition` to check after we have retrieved an object. If the condition passes then deliver this Request Object instead. This `condition` must be of type `CACHE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)",
				},
			},
		},
	}
	return nil
}

func flattenResponseObjects(responseObjectList []*gofastly.ResponseObject) []map[string]interface{} {
	var rol []map[string]interface{}
	for _, ro := range responseObjectList {
		// Convert ResponseObjects to a map for saving to state.
		nro := map[string]interface{}{
			"name":              ro.Name,
			"status":            ro.Status,
			"response":          ro.Response,
			"content":           ro.Content,
			"content_type":      ro.ContentType,
			"request_condition": ro.RequestCondition,
			"cache_condition":   ro.CacheCondition,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nro {
			if v == "" {
				delete(nro, k)
			}
		}

		rol = append(rol, nro)
	}

	return rol
}
