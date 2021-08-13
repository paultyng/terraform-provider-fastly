package fastly

import (
	"context"
	"fmt"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ServiceCRUDAttributeDefinition is an interface for most ServiceAttributeDefinition implementations which can be
// represented by the four CRUD operations. Most service attributes will fall into this category and should implement
// this interface instead of ServiceAttributeDefinition directly. An attribute that implements
// ServiceCRUDAttributeDefinition can use the ToServiceAttributeDefinition function defined below in its constructor to
// convert it to the ServiceAttributeDefinition that the service resources expect.
// The requirements for a service attribute to be defined in terms of this interface are:
// - the attribute must be a nested block with a schema type of schema.TypeSet
// - the nested block must have its own "name" attribute which uniquely defines the nested resource
// - the block must support all four of the CRUD operations, or at least Create, Read, and Delete if updating the
//   resource is not supported
// Some service attributes don't fit into these constraints are better suited to implementing the
// ServiceAttributeDefinition directly. One example is the "package" block in block_fastly_service_v1_package.go, which
// only uses an Update operation, and therefore implements ServiceAttributeDefinition directly without
// ServiceCRUDAttributeDefinition.
type ServiceCRUDAttributeDefinition interface {
	// Key returns the name of the nested block. This is used when composing the schema into the parent Service schema.
	Key() string

	// GetSchema returns the schema.Schema of the nested block. This gets composed into the parent Service schema.
	GetSchema() *schema.Schema

	// Create should create an instance of the nested block. The resource argument will be a map containing all of the
	// attributes from the schema of the nested block. The d argument, of type schema.ResourceData, will contain the
	// data for the whole service, so the resource argument should be used for most of the creation parameters. The d
	// argument should just be used for things like getting the service ID. The serviceVersion argument should be used
	// to decide which version of the service to make updates to. This will be an unlocked version that the base service
	// update function created.
	Create(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	// Read should refresh the state of all of the instance of the nested blocks. See the description of Create for more
	// details about the arguments.
	Read(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	// Update should make changes to an existing instance of the nested block. The arguments are as described in the
	// Create comments, with the exception of modified which will contain only the attributes that have changed.
	Update(ctx context.Context, d *schema.ResourceData, resource, modified map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	// Delete should remove the instance of the nested block. See the description of Create for more details about the
	// arguments.
	Delete(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
}

// ToServiceAttributeDefinition returns an implementation of ServiceAttributeDefinition for a particular implementation
// of ServiceCRUDAttributeDefinition. It implements the Process and Read methods from ServiceAttributeDefinition using
// the SetDiff functions.
func ToServiceAttributeDefinition(definition ServiceCRUDAttributeDefinition) ServiceAttributeDefinition {
	return &blockSetAttributeHandler{definition}
}

type blockSetAttributeHandler struct {
	handler ServiceCRUDAttributeDefinition
}

func (h *blockSetAttributeHandler) Register(s *schema.Resource) error {
	s.Schema[h.handler.Key()] = h.handler.GetSchema()
	return nil
}

func (h *blockSetAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	return h.handler.Read(ctx, d, nil, s.ActiveVersion.Number, conn)
}

func (h *blockSetAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, serviceVersion int, conn *gofastly.Client) error {
	oldVal, newVal := d.GetChange(h.handler.Key())
	if oldVal == nil {
		oldVal = new(schema.Set)
	}
	if newVal == nil {
		newVal = new(schema.Set)
	}

	oldSet := oldVal.(*schema.Set)
	newSet := newVal.(*schema.Set)

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

	for _, resource := range diffResult.Deleted {
		resource := resource.(map[string]interface{})
		err := h.handler.Delete(ctx, d, resource, serviceVersion, conn)
		if err != nil {
			return err
		}
	}

	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		err := h.handler.Create(ctx, d, resource, serviceVersion, conn)
		if err != nil {
			return err
		}
	}

	for _, resource := range diffResult.Modified {
		resource := resource.(map[string]interface{})

		modified := setDiff.Filter(resource, oldSet)

		err := h.handler.Update(ctx, d, resource, modified, serviceVersion, conn)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *blockSetAttributeHandler) HasChange(d *schema.ResourceData) bool {
	return d.HasChanges(h.handler.Key())
}

func (h *blockSetAttributeHandler) MustProcess(d *schema.ResourceData, _ bool) bool {
	return h.HasChange(d)
}
