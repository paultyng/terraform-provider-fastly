package fastly

import (
	"context"
	"fmt"
	"log"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type GooglePubSubServiceAttributeHandler struct {
	*DefaultServiceAttributeHandler
}

func NewServiceLoggingGooglePubSub(sa ServiceMetadata) ServiceAttributeDefinition {
	return &GooglePubSubServiceAttributeHandler{
		&DefaultServiceAttributeHandler{
			key:             "logging_googlepubsub",
			serviceMetadata: sa,
		},
	}
}

func (h *GooglePubSubServiceAttributeHandler) Register(s *schema.Resource) error {
	var blockAttributes = map[string]*schema.Schema{
		// Required fields
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The unique name of the Google Cloud Pub/Sub logging endpoint. It is important to note that changing this attribute will delete and recreate the resource",
		},

		"user": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Your Google Cloud Platform service account email address. The `client_email` field in your service account authentication JSON. You may optionally provide this via an environment variable, `FASTLY_GOOGLE_PUBSUB_EMAIL`.",
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_GOOGLE_PUBSUB_EMAIL", ""),
		},

		"secret_key": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Your Google Cloud Platform account secret key. The `private_key` field in your service account authentication JSON. You may optionally provide this secret via an environment variable, `FASTLY_GOOGLE_PUBSUB_SECRET_KEY`.",
			DefaultFunc: schema.EnvDefaultFunc("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", ""),
			Sensitive:   true,
		},

		"project_id": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The ID of your Google Cloud Platform project",
		},

		"topic": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The Google Cloud Pub/Sub topic to which logs will be published",
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
			Description:      "The version of the custom logging format used for the configured endpoint. Can be either 1 or 2. (default: 2).",
			ValidateDiagFunc: validateLoggingFormatVersion(),
		}
		blockAttributes["placement"] = &schema.Schema{
			Type:             schema.TypeString,
			Optional:         true,
			Description:      "Where in the generated VCL the logging call should be placed.",
			ValidateDiagFunc: validateLoggingPlacement(),
		}
		blockAttributes["response_condition"] = &schema.Schema{
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
		}
	}

	s.Schema[h.GetKey()] = &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		Elem: &schema.Resource{
			Schema: blockAttributes,
		},
	}
	return nil
}

func (h *GooglePubSubServiceAttributeHandler) Process(ctx context.Context, d *schema.ResourceData, latestVersion int, conn *gofastly.Client) error {
	serviceID := d.Id()
	oldLogCfg, newLogCfg := d.GetChange(h.GetKey())

	if oldLogCfg == nil {
		oldLogCfg = new(schema.Set)
	}
	if newLogCfg == nil {
		newLogCfg = new(schema.Set)
	}

	oldSet := oldLogCfg.(*schema.Set)
	newSet := newLogCfg.(*schema.Set)

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
		opts := h.buildDelete(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Google Cloud Pub/Sub logging endpoint removal opts: %#v", opts)

		if err := deleteGooglePubSub(conn, opts); err != nil {
			return err
		}
	}

	// CREATE new resources
	for _, resource := range diffResult.Added {
		resource := resource.(map[string]interface{})
		opts := h.buildCreate(resource, serviceID, latestVersion)

		log.Printf("[DEBUG] Fastly Google Cloud Pub/Sub logging addition opts: %#v", opts)

		if err := createGooglePubSub(conn, opts); err != nil {
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

		opts := gofastly.UpdatePubsubInput{
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
		if v, ok := modified["topic"]; ok {
			opts.Topic = gofastly.String(v.(string))
		}
		if v, ok := modified["user"]; ok {
			opts.User = gofastly.String(v.(string))
		}
		if v, ok := modified["secret_key"]; ok {
			opts.SecretKey = gofastly.String(v.(string))
		}
		if v, ok := modified["project_id"]; ok {
			opts.ProjectID = gofastly.String(v.(string))
		}
		if v, ok := modified["format_version"]; ok {
			opts.FormatVersion = gofastly.Uint(uint(v.(int)))
		}
		if v, ok := modified["format"]; ok {
			opts.Format = gofastly.String(v.(string))
		}
		if v, ok := modified["response_condition"]; ok {
			opts.ResponseCondition = gofastly.String(v.(string))
		}
		if v, ok := modified["placement"]; ok {
			opts.Placement = gofastly.String(v.(string))
		}

		log.Printf("[DEBUG] Update Google Cloud Pub/Sub Opts: %#v", opts)
		_, err := conn.UpdatePubsub(&opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *GooglePubSubServiceAttributeHandler) Read(ctx context.Context, d *schema.ResourceData, s *gofastly.ServiceDetail, conn *gofastly.Client) error {
	// Refresh Google Cloud Pub/Sub logging endpoints.
	log.Printf("[DEBUG] Refreshing Google Cloud Pub/Sub logging endpoints for (%s)", d.Id())
	googlepubsubList, err := conn.ListPubsubs(&gofastly.ListPubsubsInput{
		ServiceID:      d.Id(),
		ServiceVersion: s.ActiveVersion.Number,
	})

	if err != nil {
		return fmt.Errorf("[ERR] Error looking up Google Cloud Pub/Sub logging endpoints for (%s), version (%v): %s", d.Id(), s.ActiveVersion.Number, err)
	}

	googlepubsubLogList := flattenGooglePubSub(googlepubsubList)

	for _, element := range googlepubsubLogList {
		element = h.pruneVCLLoggingAttributes(element)
	}

	if err := d.Set(h.GetKey(), googlepubsubLogList); err != nil {
		log.Printf("[WARN] Error setting Google Cloud Pub/Sublogging endpoints for (%s): %s", d.Id(), err)
	}

	return nil
}

func createGooglePubSub(conn *gofastly.Client, i *gofastly.CreatePubsubInput) error {
	_, err := conn.CreatePubsub(i)
	return err
}

func deleteGooglePubSub(conn *gofastly.Client, i *gofastly.DeletePubsubInput) error {
	err := conn.DeletePubsub(i)

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

func flattenGooglePubSub(googlepubsubList []*gofastly.Pubsub) []map[string]interface{} {
	var flattened []map[string]interface{}
	for _, s := range googlepubsubList {
		// Convert logging to a map for saving to state.
		flatGooglePubSub := map[string]interface{}{
			"name":               s.Name,
			"user":               s.User,
			"secret_key":         s.SecretKey,
			"project_id":         s.ProjectID,
			"topic":              s.Topic,
			"format":             s.Format,
			"format_version":     s.FormatVersion,
			"placement":          s.Placement,
			"response_condition": s.ResponseCondition,
		}

		// Prune any empty values that come from the default string value in structs.
		for k, v := range flatGooglePubSub {
			if v == "" {
				delete(flatGooglePubSub, k)
			}
		}

		flattened = append(flattened, flatGooglePubSub)
	}

	return flattened
}

func (h *GooglePubSubServiceAttributeHandler) buildCreate(googlepubsubMap interface{}, serviceID string, serviceVersion int) *gofastly.CreatePubsubInput {
	df := googlepubsubMap.(map[string]interface{})

	var vla = h.getVCLLoggingAttributes(df)
	return &gofastly.CreatePubsubInput{
		ServiceID:         serviceID,
		ServiceVersion:    serviceVersion,
		Name:              df["name"].(string),
		User:              df["user"].(string),
		SecretKey:         df["secret_key"].(string),
		ProjectID:         df["project_id"].(string),
		Topic:             df["topic"].(string),
		Format:            vla.format,
		FormatVersion:     uintOrDefault(vla.formatVersion),
		Placement:         vla.placement,
		ResponseCondition: vla.responseCondition,
	}
}

func (h *GooglePubSubServiceAttributeHandler) buildDelete(googlepubsubMap interface{}, serviceID string, serviceVersion int) *gofastly.DeletePubsubInput {
	df := googlepubsubMap.(map[string]interface{})

	return &gofastly.DeletePubsubInput{
		ServiceID:      serviceID,
		ServiceVersion: serviceVersion,
		Name:           df["name"].(string),
	}
}
