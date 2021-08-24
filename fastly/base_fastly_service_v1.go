package fastly

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var fastlyNoServiceFoundErr = errors.New("No matching Fastly Service found")

const (
	// ServiceTypeVCL is the type for VCL services.
	ServiceTypeVCL = "vcl"
	// ServiceTypeCompute is the type for Compute services.
	ServiceTypeCompute = "wasm"
)

// ServiceAttributeDefinition provides an interface for service attributes.
// We compose a service resource out of attribute objects to allow us to construct both the VCL and Compute service
// resources from common components.
type ServiceAttributeDefinition interface {
	// Register add the attribute to the resource schema.
	Register(s *schema.Resource) error

	// Read refreshes the attribute state against the Fastly API.
	Read(d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error

	// Process creates or updates the attribute against the Fastly API.
	Process(d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error

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

// See interface definition for comments.
func (h *DefaultServiceAttributeHandler) HasChange(d *schema.ResourceData) bool {
	return d.HasChange(h.key)
}

// See interface definition for comments.
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

// ServiceDefinition defines the data model for service definitions
// There are two types of service: VCL and Compute. This interface specifies the data object from which service resources
// are constructed.
type ServiceDefinition interface {
	// GetType returns whether this is a VCL or Compute service.
	GetType() string

	// GetAttributeHandler returns the list of attributes/handlers supported by this service.
	GetAttributeHandler() []ServiceAttributeDefinition
}

// BaseServiceDefinition is the base implementation of the BaseServiceDefinition interface.
type BaseServiceDefinition struct {
	Attributes []ServiceAttributeDefinition
	Type       string
}

func (d *BaseServiceDefinition) GetType() string {
	return d.Type
}

func (d *BaseServiceDefinition) GetAttributeHandler() []ServiceAttributeDefinition {
	return d.Attributes
}

// resourceService returns a Terraform resource schema for VCL or Compute.
func resourceService(serviceDef ServiceDefinition) *schema.Resource {
	s := &schema.Resource{
		CreateContext: resourceCreate(serviceDef),
		ReadContext:   resourceRead(serviceDef),
		UpdateContext: resourceUpdate(serviceDef),
		DeleteContext: resourceDelete(serviceDef),
		Importer:      resourceImport(),
		CustomizeDiff: customdiff.All(
			customdiff.ComputedIf("cloned_version", func(_ context.Context, d *schema.ResourceDiff, _ interface{}) bool {
				// If anything other than name, comment and version_comment has changed, the current version will be
				// cloned in resourceServiceUpdate so set it as recomputed. These three fields can be updated without
				// creating a new version
				for _, changedKey := range d.GetChangedKeysPrefix("") {
					if changedKey == "name" || changedKey == "comment" || changedKey == "version_comment" {
						continue
					}
					return true
				}
				return false
			}),
			customdiff.ComputedIf("active_version", func(_ context.Context, d *schema.ResourceDiff, _ interface{}) bool {
				// If cloned_version is recomputed and we are automatically activating new versions (controlled with the
				// activate flag) then the active_version will be recomputed too.
				return d.HasChange("cloned_version") && d.Get("activate") == true
			}),
		),

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique name for the Service to create",
			},

			"comment": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "Managed by Terraform",
				Description: "Description field for the service. Default `Managed by Terraform`",
			},

			"version_comment": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Description field for the version",
			},

			// Active Version represents the currently activated version in Fastly. In
			// Terraform, we abstract this number away from the users and manage
			// creating and activating. It's used internally, but also exported for
			// users to see.
			"active_version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The currently active version of your Fastly Service",
			},

			// Cloned Version represents the latest cloned version by the provider. It
			// gets set whenever Terraform detects changes and clones the currently
			// activated version in order to modify it. Active Version and Cloned
			// Version can be different if the Activate field is set to false in order
			// to prevent the service from being activated.
			"cloned_version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The latest cloned version by the provider",
			},

			"activate": {
				Type:        schema.TypeBool,
				Description: "Conditionally prevents the Service from being activated. The apply step will continue to create a new draft version but will not activate it if this is set to `false`. Default `true`",
				Default:     true,
				Optional:    true,
			},

			"force_destroy": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Services that are active cannot be destroyed. In order to destroy the Service, set `force_destroy` to `true`. Default `false`",
			},
		},
	}

	// This loops over all the attribute handlers in the service definition and calls Register.
	// Register adds schema attributes to the overall schema for the resource. This allows each AttributeHandler to
	// define its own attributes while allowing the overall set to be composed.
	for _, a := range serviceDef.GetAttributeHandler() {
		a.Register(s) // Mutates s, adding handler-specific schema items to the list.
	}

	return s
}

// resourceCreate satisfies the Terraform resource schema Create "interface"
// while injecting the ServiceDefinition into the true Create functionality.
func resourceCreate(serviceDef ServiceDefinition) schema.CreateContextFunc {
	return func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		return resourceServiceCreate(ctx, d, meta, serviceDef)
	}
}

// resourceRead satisfies the Terraform resource schema Read "interface"
// while injecting the ServiceDefinition into the true Read functionality.
func resourceRead(serviceDef ServiceDefinition) schema.ReadContextFunc {
	return func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		return resourceServiceRead(ctx, d, meta, serviceDef)
	}
}

// resourceUpdate satisfies the Terraform resource schema Update "interface"
// while injecting the ServiceDefinition into the true Update functionality.
func resourceUpdate(serviceDef ServiceDefinition) schema.UpdateContextFunc {
	return func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		return resourceServiceUpdate(ctx, d, meta, serviceDef, false)
	}
}

// resourceDelete satisfies the Terraform resource schema Delete "interface"
// while injecting the ServiceDefinition into the true Delete functionality.
func resourceDelete(serviceDef ServiceDefinition) schema.DeleteContextFunc {
	return func(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		return resourceServiceDelete(ctx, d, meta, serviceDef)
	}
}

// resourceImport satisfies the Terraform resource schema Importer "interface"
func resourceImport() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
			parts := strings.Split(d.Id(), "@")
			if len(parts) > 2 {
				return nil, fmt.Errorf("expected import ID to either be the service ID, or be specified as <service id>@<service version>, e.g. nci48cow8ncw8ocn75@3")
			}

			id := parts[0]
			d.SetId(id)

			if len(parts) == 2 {
				version, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("error parsing %s an integer", parts[1])
				}

				err = d.Set("cloned_version", version)
				if err != nil {
					return nil, err
				}
			}

			return []*schema.ResourceData{d}, nil
		},
	}
}

// resourceServiceCreate provides service resource Create functionality.
func resourceServiceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}, serviceDef ServiceDefinition) diag.Diagnostics {
	if err := validateVCLs(d); err != nil {
		return diag.FromErr(err)
	}

	conn := meta.(*FastlyClient).conn
	service, err := conn.CreateService(&gofastly.CreateServiceInput{
		Name:    d.Get("name").(string),
		Comment: d.Get("comment").(string),
		Type:    serviceDef.GetType(),
	})

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(service.ID)

	// If the service was just created, there is an empty Version 1 available
	// that is unlocked and can be updated.
	err = d.Set("cloned_version", 1)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceServiceUpdate(ctx, d, meta, serviceDef, true)
}

// resourceServiceUpdate provides service resource Update functionality.
func resourceServiceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}, serviceDef ServiceDefinition, isCreate bool) diag.Diagnostics {
	if err := validateVCLs(d); err != nil {
		return diag.FromErr(err)
	}

	conn := meta.(*FastlyClient).conn

	// Update Name and/or Comment. No new version is required for this.
	if d.HasChanges("name", "comment") {
		_, err := conn.UpdateService(&gofastly.UpdateServiceInput{
			ServiceID: d.Id(),
			Name:      gofastly.String(d.Get("name").(string)),
			Comment:   gofastly.String(d.Get("comment").(string)),
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// Once activated, Versions are locked and become immutable.
	// This loops over all AttributeHandlers calling HasChange. In this way each attribute handler can contribute
	// whether their current state and proposed changes mean a new version must be created.
	// So where changes are required, a new version must be created first, and updates posted to that
	// version. We only need one change to trigger this, so a break is OK.
	var needsChange bool
	for _, a := range serviceDef.GetAttributeHandler() {
		if a.HasChange(d) {
			needsChange = true
			break
		}
	}

	// Update the cloned version's comment. No new version is required for this.
	if d.HasChange("version_comment") && (!needsChange || isCreate) {
		opts := gofastly.UpdateVersionInput{
			ServiceID:      d.Id(),
			ServiceVersion: d.Get("cloned_version").(int),
			Comment:        gofastly.String(d.Get("version_comment").(string)),
		}

		log.Printf("[DEBUG] Update Version opts: %#v", opts)
		_, err := conn.UpdateVersion(&opts)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	initialVersion := false

	if needsChange {
		var latestVersion int
		if isCreate {
			initialVersion = true
			// If the service was just created, there is an empty Version 1 available
			// that is unlocked and can be updated.
			latestVersion = 1
		} else {
			latestVersion = d.Get("cloned_version").(int)
			// Clone the latest version, giving us an unlocked version we can modify.
			log.Printf("[DEBUG] Creating clone of version (%d) for updates", latestVersion)
			newVersion, err := conn.CloneVersion(&gofastly.CloneVersionInput{
				ServiceID:      d.Id(),
				ServiceVersion: latestVersion,
			})
			if err != nil {
				return diag.FromErr(err)
			}

			// The new version number is named "Number", but it's actually a string.
			latestVersion = newVersion.Number

			// New versions are not immediately found in the API, or are not
			// immediately mutable, so we need to sleep a few and let Fastly ready
			// itself. Typically, 7 seconds is enough.
			log.Print("[DEBUG] Sleeping 7 seconds to allow Fastly Version to be available")
			time.Sleep(7 * time.Second)

			// Update the cloned version's comment.
			if d.Get("version_comment").(string) != "" {
				opts := gofastly.UpdateVersionInput{
					ServiceID:      d.Id(),
					ServiceVersion: latestVersion,
					Comment:        gofastly.String(d.Get("version_comment").(string)),
				}

				log.Printf("[DEBUG] Update Version opts: %#v", opts)
				_, err := conn.UpdateVersion(&opts)
				if err != nil {
					return diag.FromErr(err)
				}
			}
		}

		// This delegates the bulk of processing to attribute handlers which manage state
		// for their own attributes.
		for _, a := range serviceDef.GetAttributeHandler() {
			if a.MustProcess(d, initialVersion) {
				// Check if the Update has been cancelled and return early if so
				if err := ctx.Err(); err != nil {
					if errors.Is(err, context.Canceled) {
						return nil
					}

					return diag.FromErr(err)
				}

				if err := a.Process(d, latestVersion, conn); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		// Validate version.
		log.Printf("[DEBUG] Validating Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
		valid, msg, err := conn.ValidateVersion(&gofastly.ValidateVersionInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
		})

		if err != nil {
			return diag.Errorf("[ERR] Error checking validation: %s", err)
		}

		if !valid {
			return diag.Errorf("[ERR] Invalid configuration for Fastly Service (%s): %s", d.Id(), msg)
		}

		err = d.Set("cloned_version", latestVersion)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	shouldActivate := d.Get("activate").(bool)
	versionNotYetActivated := d.Get("cloned_version") != d.Get("active_version")
	latestVersion := d.Get("cloned_version").(int)
	if shouldActivate && versionNotYetActivated {
		log.Printf("[DEBUG] Activating Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
		_, err := conn.ActivateVersion(&gofastly.ActivateVersionInput{
			ServiceID:      d.Id(),
			ServiceVersion: latestVersion,
		})
		if err != nil {
			return diag.Errorf("[ERR] Error activating version (%d): %s", latestVersion, err)
		}

		// Only if the version is valid and activated do we set the active_version.
		// This prevents us from getting stuck in cloning an invalid version.
		err = d.Set("active_version", latestVersion)
		if err != nil {
			return diag.FromErr(err)
		}
	} else {
		log.Printf("[INFO] Skipping activation of Fastly Service (%s), Version (%v)", d.Id(), latestVersion)
		log.Print("[INFO] The Terraform definition is explicitly specified to not activate the changes on Fastly")
		log.Printf("[INFO] Version (%v) has been pushed and validated", latestVersion)
		log.Printf("[INFO] Visit https://manage.fastly.com/configure/services/%s/versions/%v and activate it manually", d.Id(), latestVersion)
	}

	return resourceServiceRead(ctx, d, meta, serviceDef)
}

// resourceServiceRead provides service resource Read functionality.
func resourceServiceRead(ctx context.Context, d *schema.ResourceData, meta interface{}, serviceDef ServiceDefinition) diag.Diagnostics {
	conn := meta.(*FastlyClient).conn

	s, err := conn.GetServiceDetails(&gofastly.GetServiceInput{
		ID: d.Id(),
	})
	if err != nil {
		// Check if not found, if so, clear ID field and exit early.
		if e, ok := err.(*gofastly.HTTPError); ok && e.IsNotFound() {
			log.Printf("[WARN] %s for ID (%s)", fastlyNoServiceFoundErr, d.Id())
			d.SetId("")
			return diag.FromErr(err)
		}
		return diag.FromErr(err)
	}
	// Check if deleted, if so, clear ID field and exit early.
	if s.DeletedAt != nil {
		log.Printf("[WARN] Service ID (%s) has been deleted", d.Id())
		d.SetId("")
		return nil
	}

	// Check for service type mismatch (i.e. when importing)
	if s.Type != serviceDef.GetType() {
		return diag.Errorf("[ERR] Service type mismatch in READ, expected: %s, got: %s", serviceDef.GetType(), s.Type)
	}

	err = d.Set("name", s.Name)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("comment", s.Comment)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("version_comment", s.ActiveVersion.Comment)
	if err != nil {
		return diag.FromErr(err)
	}
	err = d.Set("active_version", s.ActiveVersion.Number)
	if err != nil {
		return diag.FromErr(err)
	}

	// If cloned_version is not set, and there is no active version, temporarily
	// set the service.ActiveVersion number to the latest version supplied via
	// the get service version details call. This is to ensure we still read all
	// of the state below. Then set the cloned_version to this version.
	// This could either happen if the current state was from v0.28.0 of the
	// provider or lower, i.e. the user has upgraded from an earlier version, or
	// if the service is being imported and no version was specified. This
	// prevents us from getting into the state where the attribute has never
	// been set and gets passed into CloneVersion in the Update function and
	// fails.
	clonedVersionNotSet := d.Get("cloned_version") == 0
	if clonedVersionNotSet {
		if s.ActiveVersion.Number == 0 {
			s.ActiveVersion.Number = s.Version.Number
		}

		err = d.Set("cloned_version", s.ActiveVersion.Number)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// If activate is false, then read the state from cloned_version instead of
	// the active version.
	// Otherwise, cloned_version should track the active version
	if d.Get("activate") == false {
		s.ActiveVersion.Number = d.Get("cloned_version").(int)
	} else {
		err := d.Set("cloned_version", s.ActiveVersion.Number)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// If CreateService succeeds, but initial updates to the Service fail, we'll
	// have an empty ActiveService version (no version is active, so we can't
	// query for information on it).
	if s.ActiveVersion.Number != 0 {

		// This delegates read to all the attribute handlers which can then manage reading state for
		// their own attributes.
		for _, a := range serviceDef.GetAttributeHandler() {
			// Check if the Read has been cancelled and return early if so
			if err := ctx.Err(); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}

				return diag.FromErr(err)
			}

			if err := a.Read(d, s, conn); err != nil {
				return diag.FromErr(err)
			}
		}
	} else {
		log.Printf("[DEBUG] Active Version for Service (%s) is empty, no state to refresh", d.Id())
	}

	return nil
}

// resourceServiceDelete provides service resource Delete functionality.
func resourceServiceDelete(_ context.Context, d *schema.ResourceData, meta interface{}, _ ServiceDefinition) diag.Diagnostics {
	conn := meta.(*FastlyClient).conn

	// Fastly will fail to delete any service with an Active Version.
	// If `force_destroy` is given, we deactivate the active version and then send
	// the DELETE call.
	if d.Get("force_destroy").(bool) {
		s, err := conn.GetServiceDetails(&gofastly.GetServiceInput{
			ID: d.Id(),
		})

		if err != nil {
			return diag.FromErr(err)
		}

		if s.ActiveVersion.Number != 0 {
			_, err := conn.DeactivateVersion(&gofastly.DeactivateVersionInput{
				ServiceID:      d.Id(),
				ServiceVersion: s.ActiveVersion.Number,
			})
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}

	err := conn.DeleteService(&gofastly.DeleteServiceInput{
		ID: d.Id(),
	})
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
