package fastly

import (
	"context"
	"fmt"
	"log"
	"strings"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DirectorServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceDirector(sa ServiceMetadata) ServiceAttributeDefinition {
	return &DirectorServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "director",
			serviceMetadata: sa,
		},
	}
}

func (h *DirectorServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
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
		opts := gofastly.DeleteDirectorInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Director Removal opts: %#v", opts)
		err := conn.DeleteDirector(&opts)
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
		opts := gofastly.CreateDirectorInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
			Comment:        resource["comment"].(string),
			Shield:         resource["shield"].(string),
			Capacity:       uint(resource["capacity"].(int)),
			Quorum:         uint(resource["quorum"].(int)),
			Retries:        uint(resource["retries"].(int)),
		}

		switch resource["type"].(int) {
		case 1:
			opts.Type = gofastly.DirectorTypeRandom
		case 2:
			opts.Type = gofastly.DirectorTypeRoundRobin
		case 3:
			opts.Type = gofastly.DirectorTypeHash
		case 4:
			opts.Type = gofastly.DirectorTypeClient
		}

		log.Printf("[DEBUG] Director Create opts: %#v", opts)
		_, err := conn.CreateDirector(&opts)
		if err != nil {
			return err
		}

		if v, ok := resource["backends"]; ok {
			backends := v.(*schema.Set).List()
			if len(backends) > 0 {
				for _, backend := range backends {
					opts := gofastly.CreateDirectorBackendInput{
						ServiceID:      d.Id(),
						ServiceVersion: latestVersion,
						Director:       resource["name"].(string),
						Backend:        backend.(string),
					}

					log.Printf("[DEBUG] Director Backend Create opts: %#v", opts)
					_, err := conn.CreateDirectorBackend(&opts)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// UPDATE modified resources
	//
	// NOTE: although the go-fastly API client enables updating of a resource by
	// its 'name' attribute, this isn't possible within terraform due to
	// constraints in the data model/schema of the resources not having a uid.
	for _, resource := range diffResult.Modified {
		resource := resource.(map[string]interface{})

		opts := gofastly.UpdateDirectorInput{
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
		if v, ok := modified["comment"]; ok {
			opts.Comment = gofastly.String(v.(string))
		}
		if v, ok := modified["shield"]; ok {
			opts.Shield = gofastly.String(v.(string))
		}
		if v, ok := modified["quorum"]; ok {
			opts.Quorum = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["type"]; ok {
			switch v.(int) {
			case 1:
				opts.Type = gofastly.DirectorTypeRandom
			case 2:
				opts.Type = gofastly.DirectorTypeRoundRobin
			case 3:
				opts.Type = gofastly.DirectorTypeHash
			case 4:
				opts.Type = gofastly.DirectorTypeClient
			}
		}
		if v, ok := modified["retries"]; ok {
			opts.Retries = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["capacity"]; ok {
			opts.Capacity = gofastly.Uint(uint(v.(int)))
		}

		log.Printf("[DEBUG] Update Director Opts: %#v", opts)
		_, err := conn.UpdateDirector(&opts)
		if err != nil {
			return err
		}

		if v, ok := modified["backends"]; ok {
			backends := v.(*schema.Set).List()
			if len(backends) > 0 {
				for _, backend := range backends {
					opts := gofastly.CreateDirectorBackendInput{
						ServiceID:      d.Id(),
						ServiceVersion: latestVersion,
						Director:       resource["name"].(string),
						Backend:        backend.(string),
					}

					log.Printf("[DEBUG] Director Backend Update opts: %#v", opts)
					_, err := conn.CreateDirectorBackend(&opts)
					if err != nil {
						// If we end up trying to create a backend that already exists, then the
						// API will return a '409 Conflict'. We don't want to return those errors
						// as they ultimately don't mean anything useful to the user.
						if !strings.Contains(err.Error(), "409 - Conflict") {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (h *DirectorServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Directors for (%s)", d.Id())
	directorList, err := conn.ListDirectors(&gofastly.ListDirectorsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Directors for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	log.Printf("[DEBUG] Refreshing Backends for (%s)", d.Id())
	backendList, err := conn.ListBackends(&gofastly.ListBackendsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Backends for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	log.Printf("[DEBUG] Refreshing Director Backends for (%s)", d.Id())
	var directorBackendList []*gofastly.DirectorBackend

	for _, director := range directorList {
		for _, backend := range backendList {
			directorBackendGet, err := conn.GetDirectorBackend(&gofastly.GetDirectorBackendInput{
				ServiceID:      d.Id(),
				ServiceVersion: s.ActiveVersion.Number,
				Director:       director.Name,
				Backend:        backend.Name,
			})
			if err == nil {
				directorBackendList = append(directorBackendList, directorBackendGet)
			}
		}
	}

	dirl := flattenDirectors(directorList, directorBackendList)

	if err := d.Set(h.GetKey(), dirl); err != nil {
		log.Printf("[WARN] Error setting Directors for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *DirectorServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Unique name for this Director. It is important to note that changing this attribute will delete and recreate the resource",
				},
				"backends": {
					Type:        schema.TypeSet,
					Required:    true,
					Description: "Names of defined backends to map the director to. Example: `[ \"origin1\", \"origin2\" ]`",
					Elem:        &schema.Schema{Type: schema.TypeString},
				},
				// optional fields
				"capacity": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     100,
					Description: "Load balancing weight for the backends. Default `100`",
				},
				"comment": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "An optional comment about the Director",
				},
				"shield": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "Selected POP to serve as a \"shield\" for backends. Valid values for `shield` are included in the [`GET /datacenters`](https://developer.fastly.com/reference/api/utils/datacenter/) API response",
				},
				"quorum": {
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          75,
					Description:      "Percentage of capacity that needs to be up for the director itself to be considered up. Default `75`",
					ValidateDiagFunc: validateDirectorQuorum(),
				},
				"type": {
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          1,
					Description:      "Type of load balance group to use. Integer, 1 to 4. Values: `1` (random), `3` (hash), `4` (client). Default `1`",
					ValidateDiagFunc: validateDirectorType(),
				},
				"retries": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     5,
					Description: "How many backends to search if it fails. Default `5`",
				},
			},
		},
	}
	return nil
}

func flattenDirectors(directorList []*gofastly.Director, directorBackendList []*gofastly.DirectorBackend) []map[string]interface{} {
	var dl []map[string]interface{}
	for _, d := range directorList {
		// Convert Director to a map for saving to state.
		nd := map[string]interface{}{
			"name":     d.Name,
			"comment":  d.Comment,
			"shield":   d.Shield,
			"type":     d.Type,
			"quorum":   int(d.Quorum),
			"capacity": int(d.Capacity),
			"retries":  int(d.Retries),
		}

		var b []interface{}
		for _, db := range directorBackendList {
			if d.Name == db.Director {
				b = append(b, db.Backend)
			}
		}
		if len(b) > 0 {
			nd["backends"] = schema.NewSet(schema.HashString, b)
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nd {
			if v == "" {
				delete(nd, k)
			}
		}

		dl = append(dl, nd)
	}
	return dl
}
