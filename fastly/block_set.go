package fastly

import (
	"context"
	"fmt"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type BlockSetAttributeDefinition interface {
	Key() string

	GetSchema() *schema.Schema

	Create(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	Read(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	Update(ctx context.Context, d *schema.ResourceData, resource, modified map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
	Delete(ctx context.Context, d *schema.ResourceData, resource map[string]interface{}, serviceVersion int, conn *gofastly.Client) error
}

func BlockSetToServiceAttributeDefinition(definition BlockSetAttributeDefinition) ServiceAttributeDefinition {
	return &blockSetAttributeHandler{definition}
}

type blockSetAttributeHandler struct {
	handler BlockSetAttributeDefinition
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
