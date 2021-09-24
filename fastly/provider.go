package fastly

import (
	"context"

	gofastly "github.com/fastly/go-fastly/v5/fastly"
	"github.com/fastly/terraform-provider-fastly/version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const TerraformProviderProductUserAgent = "terraform-provider-fastly"

// Provider returns a *schema.Provider.
func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_API_KEY", nil),
				Description: "Fastly API Key from https://app.fastly.com/#account",
			},
			"base_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("FASTLY_API_URL", gofastly.DefaultEndpoint),
				Description: "Fastly API URL",
			},
			"no_auth": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Set this to `true` if you only need data source that does not require authentication such as `fastly_ip_ranges`",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"fastly_ip_ranges":                    dataSourceFastlyIPRanges(),
			"fastly_tls_activation":               dataSourceFastlyTLSActivation(),
			"fastly_tls_activation_ids":           dataSourceFastlyTLSActivationIds(),
			"fastly_tls_certificate":              dataSourceFastlyTLSCertificate(),
			"fastly_tls_certificate_ids":          dataSourceFastlyTLSCertificateIDs(),
			"fastly_tls_configuration":            dataSourceFastlyTLSConfiguration(),
			"fastly_tls_configuration_ids":        dataSourceFastlyTLSConfigurationIDs(),
			"fastly_tls_domain":                   dataSourceFastlyTLSDomain(),
			"fastly_tls_platform_certificate":     dataSourceFastlyTLSPlatformCertificate(),
			"fastly_tls_platform_certificate_ids": dataSourceFastlyTLSPlatformCertificateIDs(),
			"fastly_tls_private_key":              dataSourceFastlyTLSPrivateKey(),
			"fastly_tls_private_key_ids":          dataSourceFastlyTLSPrivateKeyIDs(),
			"fastly_tls_subscription":             dataSourceFastlyTLSSubscription(),
			"fastly_tls_subscription_ids":         dataSourceFastlyTLSSubscriptionIDs(),
			"fastly_waf_rules":                    dataSourceFastlyWAFRules(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"fastly_service_v1":                         resourceServiceV1(),
			"fastly_service_compute":                    resourceServiceComputeV1(),
			"fastly_service_acl_entries_v1":             resourceServiceAclEntriesV1(),
			"fastly_service_dictionary_items_v1":        resourceServiceDictionaryItemsV1(),
			"fastly_service_dynamic_snippet_content_v1": resourceServiceDynamicSnippetContentV1(),
			"fastly_service_waf_configuration":          resourceServiceWAFConfigurationV1(),
			"fastly_tls_activation":                     resourceFastlyTLSActivation(),
			"fastly_tls_certificate":                    resourceFastlyTLSCertificate(),
			"fastly_tls_private_key":                    resourceFastlyTLSPrivateKey(),
			"fastly_tls_platform_certificate":           resourceFastlyTLSPlatformCertificate(),
			"fastly_tls_subscription":                   resourceFastlyTLSSubscription(),
			"fastly_tls_subscription_validation":        resourceFastlyTLSSubscriptionValidation(),
			"fastly_user_v1":                            resourceUserV1(),
		},
	}

	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		config := Config{
			ApiKey:    d.Get("api_key").(string),
			BaseURL:   d.Get("base_url").(string),
			NoAuth:    d.Get("no_auth").(bool),
			UserAgent: provider.UserAgent(TerraformProviderProductUserAgent, version.ProviderVersion),
		}
		return config.Client()
	}

	return provider
}
