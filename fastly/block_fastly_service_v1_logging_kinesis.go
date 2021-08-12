package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type KinesisServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingKinesis(sa ServiceMetadata) ServiceAttributeDefinition {
	return ToServiceAttributeDefinition(&KinesisServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_kinesis",
			serviceMetadata: sa,
		},
	})
}

func (h *KinesisServiceAttributeHandler) Key() string { return h.key }

func (h *KinesisServiceAttributeHandler) GetSchema() *schema.Schema {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Kinesis logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"topic": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The Kinesis stream name",
		},

		"region": {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     "us-east-1",
			Description: "The AWS region the stream resides in. (Default: `us-east-1`)",
		},

		"access_key": {
			Type:        schema.TypeString,
			Optional:    true,
			Sensitive:   true,
			Description: "The AWS access key to be used to write to the stream",
		},

		"secret_key": {
			Type:        schema.TypeString,
			Optional:    true,
			Sensitive:   true,
			Description: "The AWS secret access key to authenticate with",
		},

		"iam_role": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The Amazon Resource Name (ARN) for the IAM role granting Fastly access to Kinesis. Not required if `access_key` and `secret_key` are provided.",
			Sensitive:   false,
		},
	}

	if h.GetServiceMetadata().serviceType == ServiceTypeVCL {
		blockAttributes["format"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Apache style log formatting.",
		}
		blockAttributes["format_version"] = &schema.Schema{
			Type:             schema.TypeInt,
			Optional:         true,
			Default:          2,
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either `1` or `2`. (default: `2`).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed. Can be `none` or `waf_debug`.",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
		}
	}

	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
}

func (h *KinesisServiceAttributeHandler) Create(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildCreate(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Kinesis logging addition opts: %#v", opts)

	if err := createKinesis(conn, opts); err != nil {
		return err
	}
	return nil
}

func (h *KinesisServiceAttributeHandler) Read(_ context.Context, d *schema.ResourceData, _ map[string]interface{}, serviceVersion int, conn *gofastly.Client) error {
	// Refresh Kinesis.
	log.Printf("[DEBUG] Refreshing Kinesis logging endpoints for (%s)", d.Id())
	kinesisList, err := conn.ListKinesis(&gofastly.ListKinesisInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Kinesis logging endpoints for (%s), version (%v): %s", d.Id(), serviceVersion, err)
	}

	ell := flattenKinesis(kinesisList)

	for _, element := range ell {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), ell); err != nil {
		log.Printf("[WARN] Error setting Kinesis logging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func (h *KinesisServiceAttributeHandler) Update(_ context.Context, d *schema.ResourceData, resource, modified map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := gofastly.UpdateKinesisInput{
		ServiceID:      d.Id(),
		ServiceVersion: serviceVersion,
		Name:           resource["name"].(string),
	}

	// NOTE: where we transition between interface{} we lose the ability to
	// infer the underlying type being either a uint vs an int. This
	// materializes as a panic (yay) and so it's only at runtime we discover
	// this and so we've updated the below code to convert the type asserted
	// int into a uint before passing the value to gofastly.Uint().
	if v, ok := modified["topic"]; ok {
		opts.StreamName = gofastly.String(v.(string))
	}
	if v, ok := modified["region"]; ok {
		opts.Region = gofastly.String(v.(string))
	}
	if v, ok := modified["access_key"]; ok {
		opts.AccessKey = gofastly.String(v.(string))
	}
	if v, ok := modified["secret_key"]; ok {
		opts.SecretKey = gofastly.String(v.(string))
	}
	if v, ok := modified["iam_role"]; ok {
		opts.IAMRole = gofastly.String(v.(string))
	}
	if v, ok := modified["format"]; ok {
		opts.Format = gofastly.String(v.(string))
	}
	if v, ok := modified["format_version"]; ok {
		opts.FormatVersion = gofastly.Uint(uint(v.(int)))
	}
	if v, ok := modified["response_condition"]; ok {
		opts.ResponseCondition = gofastly.String(v.(string))
	}
	if v, ok := modified["placement"]; ok {
		opts.Placement = gofastly.String(v.(string))
	}

	log.Printf("[DEBUG] Update Kinesis Opts: %#v", opts)
	_, err := conn.UpdateKinesis(&opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *KinesisServiceAttributeHandler) Delete(_ context.Context, d *schema.ResourceData, resource map[string]interface {
}, serviceVersion int, conn *gofastly.Client) error {
	opts := h.buildDelete(resource, d.Id(), serviceVersion)

	log.Printf("[DEBUG] Fastly Kinesis logging endpoint removal opts: %#v", opts)

	if err := deleteKinesis(conn, opts); err != nil {
		return err
	}
	return nil
}

func createKinesis(conn *gofastly.Client, i *gofastly.CreateKinesisInput) error {
	_, err := conn.CreateKinesis(i)
	return err
}

func deleteKinesis(conn *gofastly.Client, i *gofastly.DeleteKinesisInput) error {
	err := conn.DeleteKinesis(i)

	errRes, ok := err.(*gofastly.HTTPError)
	if !ok {
		return err
	}

	// 404 response codes don't result in an error propagating because a 404 could
	// indicate that a resource was deleted elsewhere.
	if !errRes.IsNotFound() {
		return err
	}

	return nil
}

func flattenKinesis(kinesisList []*gofastly.Kinesis) []map[string]interface{} {
	var lsl []map[string]interface{}
	for _, ll := range kinesisList {
		// Convert Kinesis logging to a map for saving to state.
		nll := map[string]interface{}{
			"name":               ll.Name,
			"topic":              ll.StreamName,
			"region":             ll.Region,
			"access_key":         ll.AccessKey,
			"secret_key":         ll.SecretKey,
			"iam_role":           ll.IAMRole,
			"format":             ll.Format,
			"format_version":     ll.FormatVersion,
			"placement":          ll.Placement,
			"response_condition": ll.ResponseCondition,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range nll {
			if v == "" {
				delete(nll, k)
			}
		}

		lsl = append(lsl, nll)
	}

	return lsl
}

func (h *KinesisServiceAttributeHandler) buildCreate(kinesisMap interface{}, serviceID string, serviceVersion int) *gofastly.CreateKinesisInput {
	df := kinesisMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreateKinesisInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		StreamName:        df["topic"].(string),
		Region:            df["region"].(string),
		AccessKey:         df["access_key"].(string),
		SecretKey:         df["secret_key"].(string),
		IAMRole:           df["iam_role"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *KinesisServiceAttributeHandler) buildDelete(kinesisMap interface{}, serviceID string, serviceVersion int) *gofastly.DeleteKinesisInput {
	df := kinesisMap.(map[string]interface{})

	return &gofastly.DeleteKinesisInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
