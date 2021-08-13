package fastly

import (
	"context"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ServiceAttributeDefinition provides an interface for service attributes.
// We compose a service resource out of attribute objects to allow us to construct both the VCL and Compute service
// resources from common components.
type ServiceAttributeDefinition interface {
	// Register add the attribute to the resource schema.
	Register(s *schema.Resource) error

	// Read refreshes the attribute state against the Fastly API.
	Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error

	// Process creates or updates the attribute against the Fastly API.
	Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error

	// HasChange returns whether the state of the attribute has changed against Terraform stored state.
	HasChange(d *schema.ResourceData) bool

	// MustProcess returns whether we must process the resource (usually HasChange==true but allowing exceptions).
	// For example: at present, the settings attributeHandler (block_fastly_service_v1_settings.go) must process when
	// default_ttl==0 and it is the initialVersion - as well as when default_ttl or default_host have changed.
	MustProcess(d *schema.ResourceData, initialVersion bool) bool
}

// ServiceMetadata provides a container to pass service attributes into an Attribute handler.
type ServiceMetadata struct {
	serviceType string
}

// DefaultServiceAttributeHandler provides a base implementation for ServiceAttributeDefinition.
type DefaultServiceAttributeHandler struct {
	key             string
	serviceMetadata ServiceMetadata
}

// GetKey is provided since most attributes will just use their private "key" for interacting with the service.
func (h *DefaultServiceAttributeHandler) GetKey() string {
	return h.key
}

// GetServiceMetadata is provided to allow internal methods to get the service Metadata
func (h *DefaultServiceAttributeHandler) GetServiceMetadata() ServiceMetadata {
	return h.serviceMetadata
}

func (h *DefaultServiceAttributeHandler) HasChange(d *schema.ResourceData) bool {
	return d.HasChange(h.key)
}

func (h *DefaultServiceAttributeHandler) MustProcess(d *schema.ResourceData, _ bool) bool {
	return h.HasChange(d)
}

type VCLLoggingAttributes struct {
	format            string
	formatVersion     *uint
	placement         string
	responseCondition string
}

// getVCLLoggingAttributes provides default values to Compute services for VCL only logging attributes
func (h *DefaultServiceAttributeHandler) getVCLLoggingAttributes(data map[string]interface{}) VCLLoggingAttributes {
	var vla = VCLLoggingAttributes{
		placement: "none",
	}
	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		if val, ok := data["format"]; ok {
			vla.format = val.(string)
		}
		if val, ok := data["format_version"]; ok {
			vla.formatVersion = gofastly.Uint(uint(val.(int)))
		}
		if val, ok := data["placement"]; ok {
			vla.placement = val.(string)
		}
		if val, ok := data["response_condition"]; ok {
			vla.responseCondition = val.(string)
		}
	}
	return vla
}

// pruneVCLLoggingAttributes deletes the keys corresponding to VCL-only logging attributes which aren't present for
// Compute services.
func (h *DefaultServiceAttributeHandler) pruneVCLLoggingAttributes(data map[string]interface{}) map[string]interface{} {
	if h.GetServiceMetadata().serviceType == ServiceTypeCompute {
		delete(data, "format")
		delete(data, "format_version")
		delete(data, "placement")
		delete(data, "response_condition")
	}
	return data
}
