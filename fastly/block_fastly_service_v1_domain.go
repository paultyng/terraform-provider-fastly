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
	return &DomainServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "domain",
			serviceMetadata: sa,
		},
	}
}

func (h *DomainServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	od, nd := d.GetChange(h.GetKey())
	if od == nil {
		od = new(schema.Set)
	}
	if nd == nil {
		nd = new(schema.Set)
	}

	oldSet := od.(*schema.Set)
	newSet := nd.(*schema.Set)

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
		opts := gofastly.DeleteDomainInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
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
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := gofastly.CreateDomainInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
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
	}

	// UPDATE modified resources
	//
	// NOTE: although the go-fastly API client enables updating of a resource by
	// its 'name' attribute, this isn't possible within terraform due to
	// constraints in the data model/schema of the resources not having a uid.
	for _, resource := range diffResult.Modified {
		resource := resource.(map[string]interface{})

		opts := gofastly.UpdateDomainInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		// only attempt to update attributes that have changed
		modified := setDiff.Filter(resource, oldSet)

		if v, ok := modified["comment"]; ok {
			opts.Comment = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Domain Opts: %#v", opts)
		_, err := conn.UpdateDomain(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *DomainServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// TODO: update go-fastly to support an ActiveVersion struct, which contains
	// domain and backend info in the response. Here we do 2 additional queries
	// to find out that info
	log.Printf("[DEBUG] Refreshing Domains for (%s)", d.Id())
	domainList, err := conn.ListDomains(&gofastly.ListDomainsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Domains for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	// Refresh Domains
	dl := flattenDomains(domainList)

	if err := d.Set(h.GetKey(), dl); err != nil {
		log.Printf("[WARN] Error setting Domains for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *DomainServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
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
