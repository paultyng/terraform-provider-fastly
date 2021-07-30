package fastly

import (
	"context"
	"fmt"
	"log"
	"strings"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type RequestSettingServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceRequestSetting(sa ServiceMetadata) ServiceAttributeDefinition {
	return &RequestSettingServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "request_setting",
			serviceMetadata: sa,
		},
	}
}

func (h *RequestSettingServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	os, ns := d.GetChange(h.GetKey())
	if os == nil {
		os = new(schema.Set)
	}
	if ns == nil {
		ns = new(schema.Set)
	}

	oldSet := os.(*schema.Set)
	newSet := ns.(*schema.Set)

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
		opts := gofastly.DeleteRequestSettingInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		log.Printf("[DEBUG] Fastly Request Setting removal opts: %#v", opts)
		err := conn.DeleteRequestSetting(&opts)
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
		opts, err := buildRequestSetting(resource.(map[string]interface{}))
		if err != nil {
			log.Printf("[DEBUG] Error building Request Setting: %s", err)
			return err
		}
		opts.ServiceID = d.Id()
		opts.ServiceVersion = latestVersion

		log.Printf("[DEBUG] Create Request Setting Opts: %#v", opts)
		_, err = conn.CreateRequestSetting(opts)
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

		opts := gofastly.UpdateRequestSettingInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
			Name:           resource["name"].(string),
		}

		// only attempt to update attributes that have changed
		modified := setDiff.Filter(resource, oldSet)

		if v, ok := modified["force_miss"]; ok {
			opts.ForceMiss = gofastly.CBool(v.(bool))
		}
		if v, ok := modified["force_ssl"]; ok {
			opts.ForceSSL = gofastly.CBool(v.(bool))
		}
		if v, ok := modified["action"]; ok {
			switch strings.ToLower(v.(string)) {
			case "lookup":
				opts.Action = gofastly.RequestSettingActionLookup
			case "pass":
				opts.Action = gofastly.RequestSettingActionPass
			}
		}
		if v, ok := modified["bypass_busy_wait"]; ok {
			opts.BypassBusyWait = gofastly.CBool(v.(bool))
		}
		if v, ok := modified["max_stale_age"]; ok {
			opts.MaxStaleAge = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["hash_keys"]; ok {
			opts.HashKeys = gofastly.String(v.(string))
		}
		if v, ok := modified["xff"]; ok {
			switch strings.ToLower(v.(string)) {
			case "clear":
				opts.XForwardedFor = gofastly.RequestSettingXFFClear
			case "leave":
				opts.XForwardedFor = gofastly.RequestSettingXFFLeave
			case "append":
				opts.XForwardedFor = gofastly.RequestSettingXFFAppend
			case "append_all":
				opts.XForwardedFor = gofastly.RequestSettingXFFAppendAll
			case "overwrite":
				opts.XForwardedFor = gofastly.RequestSettingXFFOverwrite
			}
		}
		if v, ok := modified["timer_support"]; ok {
			opts.TimerSupport = gofastly.CBool(v.(bool))
		}
		if v, ok := modified["geo_headers"]; ok {
			opts.GeoHeaders = gofastly.CBool(v.(bool))
		}
		if v, ok := modified["default_host"]; ok {
			opts.DefaultHost = gofastly.String(v.(string))
		}
		if v, ok := modified["request_condition"]; ok {
			opts.RequestCondition = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Request Settings Opts: %#v", opts)
		_, err := conn.UpdateRequestSetting(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *RequestSettingServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Request Settings for (%s)", d.Id())
	rsList, err := conn.ListRequestSettings(&gofastly.ListRequestSettingsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Request Settings for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	rl := flattenRequestSettings(rsList)

	if err := d.Set(h.GetKey(), rl); err != nil {
		log.Printf("[WARN] Error setting Request Settings for (%s): %s", d.Id(), err)
	}
	return nil
}

func (h *RequestSettingServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				// Required fields
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "Unique name to refer to this Request Setting. It is important to note that changing this attribute will delete and recreate the resource",
				},
				// Optional fields
				"request_condition": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "",
					Description: "Name of already defined `condition` to determine if this request setting should be applied",
				},
				"max_stale_age": {
					Type:        schema.TypeInt,
					Optional:    true,
					Description: "How old an object is allowed to be to serve `stale-if-error` or `stale-while-revalidate`, in seconds",
				},
				"force_miss": {
					Type:        schema.TypeBool,
					Optional:    true,
					Description: "Force a cache miss for the request. If specified, can be `true` or `false`",
				},
				"force_ssl": {
					Type:        schema.TypeBool,
					Optional:    true,
					Description: "Forces the request to use SSL (Redirects a non-SSL request to SSL)",
				},
				"action": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Allows you to terminate request handling and immediately perform an action. When set it can be `lookup` or `pass` (Ignore the cache completely)",
				},
				"bypass_busy_wait": {
					Type:        schema.TypeBool,
					Optional:    true,
					Description: "Disable collapsed forwarding, so you don't wait for other objects to origin",
				},
				"hash_keys": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Comma separated list of varnish request object fields that should be in the hash key",
				},
				"xff": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "append",
					Description: "X-Forwarded-For, should be `clear`, `leave`, `append`, `append_all`, or `overwrite`. Default `append`",
				},
				"timer_support": {
					Type:        schema.TypeBool,
					Optional:    true,
					Description: "Injects the X-Timer info into the request for viewing origin fetch durations",
				},
				// TODO: Although Fastly API has been exposing this parameter over years
				// it turned out that setting this parameter does nothing. We should remove this attribute in v1.0.0
				"geo_headers": {
					Type:        schema.TypeBool,
					Optional:    true,
					Deprecated:  "'geo_headers' attribute has been deprecated and will be removed in the next major version release",
					Description: "Injects Fastly-Geo-Country, Fastly-Geo-City, and Fastly-Geo-Region into the request headers",
				},
				"default_host": {
					Type:        schema.TypeString,
					Optional:    true,
					Description: "Sets the host header",
				},
			},
		},
	}
	return nil
}

func flattenRequestSettings(rsList []*gofastly.RequestSetting) []map[string]interface{} {
	var rl []map[string]interface{}
	for _, r := range rsList {
		// Convert Request Settings to a map for saving to state.
		nrs := map[string]interface{}{
			"name":              r.Name,
			"max_stale_age":     r.MaxStaleAge,
			"force_miss":        r.ForceMiss,
			"force_ssl":         r.ForceSSL,
			"action":            r.Action,
			"bypass_busy_wait":  r.BypassBusyWait,
			"hash_keys":         r.HashKeys,
			"xff":               r.XForwardedFor,
			"timer_support":     r.TimerSupport,
			"geo_headers":       r.GeoHeaders,
			"default_host":      r.DefaultHost,
			"request_condition": r.RequestCondition,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nrs {
			if v == "" {
				delete(nrs, k)
			}
		}

		rl = append(rl, nrs)
	}

	return rl
}

func buildRequestSetting(requestSettingMap interface{}) (*gofastly.CreateRequestSettingInput, error) {
	resource := requestSettingMap.(map[string]interface{})
	opts := gofastly.CreateRequestSettingInput{
		Name:             resource["name"].(string),
		MaxStaleAge:      uint(resource["max_stale_age"].(int)),
		ForceMiss:        gofastly.Compatibool(resource["force_miss"].(bool)),
		ForceSSL:         gofastly.Compatibool(resource["force_ssl"].(bool)),
		BypassBusyWait:   gofastly.Compatibool(resource["bypass_busy_wait"].(bool)),
		HashKeys:         resource["hash_keys"].(string),
		TimerSupport:     gofastly.Compatibool(resource["timer_support"].(bool)),
		GeoHeaders:       gofastly.Compatibool(resource["geo_headers"].(bool)),
		DefaultHost:      resource["default_host"].(string),
		RequestCondition: resource["request_condition"].(string),
	}

	act := strings.ToLower(resource["action"].(string))
	switch act {
	case "lookup":
		opts.Action = gofastly.RequestSettingActionLookup
	case "pass":
		opts.Action = gofastly.RequestSettingActionPass
	}

	xff := strings.ToLower(resource["xff"].(string))
	switch xff {
	case "clear":
		opts.XForwardedFor = gofastly.RequestSettingXFFClear
	case "leave":
		opts.XForwardedFor = gofastly.RequestSettingXFFLeave
	case "append":
		opts.XForwardedFor = gofastly.RequestSettingXFFAppend
	case "append_all":
		opts.XForwardedFor = gofastly.RequestSettingXFFAppendAll
	case "overwrite":
		opts.XForwardedFor = gofastly.RequestSettingXFFOverwrite
	}

	return &opts, nil
}
