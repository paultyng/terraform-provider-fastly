package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type HealthCheckServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceHealthCheck(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&HealthCheckServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "healthcheck",
			serviceMetadata: sa,
		},
	})
}

func (h *HealthCheckServiceAttributeHandler) Key() string { return h.key }

func (h *HealthCheckServiceAttributeHandler) GetSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				// required fields
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "A unique name to identify this Healthcheck. It is important to note that changing this attribute will delete and recreate the resource",
				},
				"host": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The Host header to send for this Healthcheck",
				},
				"path": {
					Type:        schema.TypeString,
					Required:    true,
					Description: "The path to check",
				},
				// optional fields
				"check_interval": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     5000,
					Description: "How often to run the Healthcheck in milliseconds. Default `5000`",
				},
				"expected_response": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     200,
					Description: "The status code expected from the host. Default `200`",
				},
				"http_version": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "1.1",
					Description: "Whether to use version 1.0 or 1.1 HTTP. Default `1.1`",
				},
				"initial": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     3,
					Description: "When loading a config, the initial number of probes to be seen as OK. Default `3`",
				},
				"method": {
					Type:        schema.TypeString,
					Optional:    true,
					Default:     "HEAD",
					Description: "Which HTTP method to use. Default `HEAD`",
				},
				"threshold": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     3,
					Description: "How many Healthchecks must succeed to be considered healthy. Default `3`",
				},
				"timeout": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     500,
					Description: "Timeout in milliseconds. Default `500`",
				},
				"window": {
					Type:        schema.TypeInt,
					Optional:    true,
					Default:     5,
					Description: "The number of most recent Healthcheck queries to keep for this Healthcheck. Default `5`",
				},
			},
		},
	}
}

func (h *HealthCheckServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.CreateHealthCheckInput{
		ServiceID:        d.Id(),
		ServiceVersion:   serviceVersion,
		Name:             resource["name"].(string),
		Host:             resource["host"].(string),
		Path:             resource["path"].(string),
		CheckInterval:    uint(resource["check_interval"].(int)),
		ExpectedResponse: uint(resource["expected_response"].(int)),
		HTTPVersion:      resource["http_version"].(string),
		Initial:          uint(resource["initial"].(int)),
		Method:           resource["method"].(string),
		Threshold:        uint(resource["threshold"].(int)),
		Timeout:          uint(resource["timeout"].(int)),
		Window:           uint(resource["window"].(int)),
	}

	log.Printf("[DEBUG] Create Healthcheck Opts: %#v", opts)
	_, err := conn.CreateHealthCheck(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *HealthCheckServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	log.Printf("[DEBUG] Refreshing Healthcheck for (%s)", d.Id())
	healthcheckList, err := conn.ListHealthChecks(&gofastly.ListHealthChecksInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Healthcheck for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	hcl := flattenHealthchecks(healthcheckList)

	if err := d.Set(h.GetKey(), hcl); err != nil {
		log.Printf("[WARN] Error setting Healthcheck for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *HealthCheckServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateHealthCheckInput{
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
	if v, ok := modified["method"]; ok {
		opts.Method = gofastly.String(v.(string))
	}
	if v, ok := modified["host"]; ok {
		opts.Host = gofastly.String(v.(string))
	}
	if v, ok := modified["path"]; ok {
		opts.Path = gofastly.String(v.(string))
	}
	if v, ok := modified["http_version"]; ok {
		opts.HTTPVersion = gofastly.String(v.(string))
	}
	if v, ok := modified["timeout"]; ok {
		opts.Timeout = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["check_interval"]; ok {
		opts.CheckInterval = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["expected_response"]; ok {
		opts.ExpectedResponse = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["window"]; ok {
		opts.Window = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["threshold"]; ok {
		opts.Threshold = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["initial"]; ok {
		opts.Initial = gofastly.Uint(uint(v.(int)))
	}

	log.Printf("[DEBUG] Update Healthcheck Opts: %#v", opts)
	_, err := conn.UpdateHealthCheck(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *HealthCheckServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.DeleteHealthCheckInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	log.Printf("[DEBUG] Fastly Healthcheck removal opts: %#v", opts)
	err := conn.DeleteHealthCheck(&opts)
	if errRes, ok := err.(*gofastly.HTTPError); ok {
		if errRes.StatusCode != 404 {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func flattenHealthchecks(healthcheckList []*gofastly.HealthCheck) []map[string]interface{} {
	var hl []map[string]interface{}
	for _, h := range healthcheckList {
		// Convert HealthChecks to a map for saving to state.
		nh := map[string]interface{}{
			"name":              h.Name,
			"host":              h.Host,
			"path":              h.Path,
			"check_interval":    h.CheckInterval,
			"expected_response": h.ExpectedResponse,
			"http_version":      h.HTTPVersion,
			"initial":           h.Initial,
			"method":            h.Method,
			"threshold":         h.Threshold,
			"timeout":           h.Timeout,
			"window":            h.Window,
		}

		// prune any empty values that come from the default string value in structs
		for k, v := range nh {
			if v == "" {
				delete(nh, k)
			}
		}

		hl = append(hl, nh)
	}

	return hl
}
