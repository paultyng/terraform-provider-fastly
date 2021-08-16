package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type SettingsServiceAttributeHandler struct {
}

func NewServiceSettings() ServiceAttributeDefinition {
	return &SettingsServiceAttributeHandler{}
}

func (h *SettingsServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateSettingsInput{
		ServiceID:      d.Id(),
		ServiceVersion: latestVersion,
		DefaultHost:    gofastly.String(d.Get("default_host").(string)),
		// default_ttl has the same default value of 3600 that is provided by
		// the Fastly API, so it's safe to include here
		DefaultTTL: uint(d.Get("default_ttl").(int)),
	}

	log.Printf("[DEBUG] Update Settings opts: %#v", opts)
	_, err := conn.UpdateSettings(&opts)

	return err
}

func (h *SettingsServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	settingsOpts := gofastly.GetSettingsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	}
	if settings, err := conn.GetSettings(&settingsOpts); err == nil {
		d.Set("default_host", settings.DefaultHost)
		d.Set("default_ttl", int(settings.DefaultTTL))
	} else {
		return fmt.Errorf("[ERR] Error looking up Version settings for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}
	return nil
}

func (h *SettingsServiceAttributeHandler) HasChange(d *schema.ResourceData) bool {
	return d.HasChanges("default_ttl", "default_host")
}

// If the requested default_ttl is 0, and this is the first
// version being created, HasChange will return false, but we need
// to set it anyway, so ensure we update the settings in that
// case.
func (h *SettingsServiceAttributeHandler) MustProcess(d *schema.ResourceData, initialVersion bool) bool {
	return h.HasChange(d) || (d.Get("default_ttl") == 0 && initialVersion)
}

func (h *SettingsServiceAttributeHandler) Register(s *schema.Resource) error {
	s.Schema["default_ttl"] = &schema.Schema{
		Type:        schema.TypeInt,
		Optional:    true,
		Default:     3600,
		Description: "The default Time-to-live (TTL) for requests",
	}
	s.Schema["default_host"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		Description: "The default hostname",
	}
	return nil
}
