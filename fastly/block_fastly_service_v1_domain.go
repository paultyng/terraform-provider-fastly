package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DomainServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceDomain(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&DomainServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "domain",
			serviceMetadata: sa,
		},
	})
}

func (h *DomainServiceAttributeHandler) Key() string { return h.key }

func (h *DomainServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeSet,
		Required:    true,
		Description: "A set of Domain names to serve as entry points for your Service",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The domain that this Service will respond to. It is important to note that changing this attribute will delete and recreate the resource.",
				},

				"comment": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "An optional comment about the Domain.",
				},
			},
		},
	}
}

func (h *DomainServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.CreateDomainInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	if v, ok := resource["comment"]; ok {
		opts.Comment = v.(string)
	}

	log.Printf("[DEBUG] Fastly Domain Addition opts: %#v", opts)
	_, err := conn.CreateDomain(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *DomainServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// TODO: update go-fastly to support an ActiveVersion struct, which contains
	// domain and backend info in the response. Here we do 2 additional queries
	// to find out that info
	log.Printf("[DEBUG] Refreshing Domains for (%s)", d.Id())
	domainList, err := conn.ListDomains(&gofastly.ListDomainsInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Domains for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	// Refresh Domains
	dl := flattenDomains(domainList)

	if err := d.Set(h.GetKey(), dl); err != nil {
		log.Printf("[WARN] Error setting Domains for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *DomainServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateDomainInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	if v, ok := modified["comment"]; ok {
		opts.Comment = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Domain Opts: %#v", opts)
	_, err := conn.UpdateDomain(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *DomainServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteDomainInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Domain removal opts: %#v", opts)
	err := conn.DeleteDomain(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenDomains(list []*gofastly.Domain) []map[string]interface{} {
	dl := make([]map[string]interface{}, 0, len(list))

	for _, d := range list {
		dl = append(dl, map[string]interface{}{
			"name":    d.Name,
			"comment": d.Comment,
		})
	}

	return dl
}
