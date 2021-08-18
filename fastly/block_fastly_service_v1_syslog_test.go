package fastly

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceFastlyFlattenSyslog(t *testing.T) {
	key, cert, err := generateKeyAndCert()
	if err != nil {
		t.Errorf("Failed to generate key and cert: %s", err)
	}

	cases := []struct {
		remote []*gofastly.Syslog
		local  []map[string]interface{}
	}{
		{
			remote: []*gofastly.Syslog{
				{
					ServiceVersion:    1,
					Name:              "somesyslogname",
					Address:           "127.0.0.1",
					IPV4:              "127.0.0.1",
					Port:              8080,
					Format:            `%h %l %u %t "%r" %>s %b`,
					FormatVersion:     1,
					ResponseCondition: "response_condition_test",
					MessageType:       "classic",
					Token:             "abcd1234",
					UseTLS:            true,
					TLSCACert:         cert,
					TLSHostname:       "example.com",
					TLSClientCert:     cert,
					TLSClientKey:      key,
				},
			},
			local: []map[string]interface{}{
				{
					"name":               "somesyslogname",
					"address":            "127.0.0.1",
					"port":               uint(8080),
					"format":             `%h %l %u %t "%r" %>s %b`,
					"format_version":     uint(1),
					"response_condition": "response_condition_test",
					"message_type":       "classic",
					"token":              "abcd1234",
					"use_tls":            true,
					"tls_hostname":       "example.com",
					"tls_ca_cert":        cert,
					"tls_client_cert":    cert,
					"tls_client_key":     key,
				},
			},
		},
	}

	for _, c := range cases {
		out := flattenSyslogs(c.remote)
		if !reflect.DeepEqual(out, c.local) {
			t.Fatalf("Error matching:\nexpected: %#v\n got: %#v", c.local, out)
		}
	}

}

func TestAccFastlyServiceV1_syslog_basic(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName1 := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	log1 := gofastly.Syslog{
		ServiceVersion:    1,
		Name:              "somesyslogname",
		Address:           "127.0.0.1",
		IPV4:              "127.0.0.1",
		Port:              uint(514),
		Format:            `%h %l %u %t "%r" %>s %b`,
		FormatVersion:     2,
		ResponseCondition: "response_condition_test",
		MessageType:       "classic",
	}

	log1_after_update := gofastly.Syslog{
		ServiceVersion:    1,
		Name:              "somesyslogname",
		Address:           "127.0.0.1",
		IPV4:              "127.0.0.1",
		Port:              uint(514),
		Format:            `%h %l %u %t "%r" %>s %b`,
		FormatVersion:     2,
		ResponseCondition: "response_condition_test",
		MessageType:       "blank",
	}

	log2 := gofastly.Syslog{
		ServiceVersion: 1,
		Name:           "somesyslogname2",
		Address:        "127.0.0.2",
		IPV4:           "127.0.0.2",
		Port:           uint(10514),
		Format:         `%h %l %u %t "%r" %>s %b`,
		FormatVersion:  2,
		MessageType:    "classic",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{

			{
				Config: testAccServiceV1SyslogConfig(name, domainName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1SyslogAttributes(&service, []*gofastly.Syslog{&log1}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "syslog.#", "1"),
				),
			},

			{
				Config: testAccServiceV1SyslogConfig_update(name, domainName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1SyslogAttributes(&service, []*gofastly.Syslog{&log1_after_update, &log2}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "syslog.#", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceV1_syslog_basic_compute(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName1 := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	log1 := gofastly.Syslog{
		ServiceVersion: 1,
		Name:           "somesyslogname",
		Address:        "127.0.0.1",
		IPV4:           "127.0.0.1",
		Port:           uint(514),
		MessageType:    "classic",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1SyslogComputeConfig(name, domainName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_compute.foo", &service),
					testAccCheckFastlyServiceV1SyslogAttributes(&service, []*gofastly.Syslog{&log1}, ServiceTypeCompute),
					resource.TestCheckResourceAttr(
						"fastly_service_compute.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_compute.foo", "syslog.#", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceV1_syslog_formatVersion(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName1 := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	log1 := gofastly.Syslog{
		ServiceVersion: 1,
		Name:           "somesyslogname",
		Address:        "127.0.0.1",
		IPV4:           "127.0.0.1",
		Port:           uint(514),
		Format:         "%a %l %u %t %m %U%q %H %>s %b %T",
		FormatVersion:  2,
		MessageType:    "classic",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1SyslogConfig_formatVersion(name, domainName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1SyslogAttributes(&service, []*gofastly.Syslog{&log1}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "syslog.#", "1"),
				),
			},
		},
	})
}

// This test should not be run in parallel due to its use of schema.EnvDefaultFunc to set/reset environment variables,
// which conflicts with other running tests.
func TestAccFastlyServiceV1_syslog_useTLS(t *testing.T) {
	key, cert, err := generateKeyAndCert()
	if err != nil {
		t.Errorf("Failed to generate key and cert: %s", err)
	}
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName1 := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	// set env Vars to something we expect
	resetEnv := setSyslogEnv(key, cert, t)
	defer resetEnv()

	log1 := gofastly.Syslog{
		ServiceVersion: 1,
		Name:           "somesyslogname",
		Address:        "127.0.0.1",
		IPV4:           "127.0.0.1",
		Port:           uint(514),
		Format:         `%h %l %u %t "%r" %>s %b`,
		FormatVersion:  2,
		MessageType:    "classic",
		UseTLS:         true,
		TLSCACert:      cert,
		TLSHostname:    "example.com",
		TLSClientCert:  cert,
		TLSClientKey:   key,
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1SyslogConfig_useTls(name, domainName1),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1SyslogAttributes(&service, []*gofastly.Syslog{&log1}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", name),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "syslog.#", "1"),
				),
			},
		},
	})
}

func testAccCheckFastlyServiceV1SyslogAttributes(service *gofastly.ServiceDetail, syslogs []*gofastly.Syslog, serviceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		conn := testAccProvider.Meta().(*FastlyClient).conn
		syslogList, err := conn.ListSyslogs(&gofastly.ListSyslogsInput{
			ServiceID:      service.ID,
			ServiceVersion: service.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Syslog Logging for (%s), version (%d): %s", service.Name, service.ActiveVersion.Number, err)
		}

		if len(syslogList) != len(syslogs) {
			return fmt.Errorf("Syslog List count mismatch, expected (%d), got (%d)", len(syslogs), len(syslogList))
		}

		log.Printf("[DEBUG] syslogList = %+v\n", syslogList)

		var found int
		for _, s := range syslogs {
			for _, ls := range syslogList {
				if s.Name == ls.Name {
					// we don't know these things ahead of time, so populate them now
					s.ServiceID = service.ID
					s.ServiceVersion = service.ActiveVersion.Number
					// We don't track these, so clear them out because we also wont know
					// these ahead of time
					ls.CreatedAt = nil
					ls.UpdatedAt = nil

					// Ignore VCL attributes for Compute and set to whatever is returned from the API.
					if serviceType == ServiceTypeCompute {
						ls.FormatVersion = s.FormatVersion
						ls.Format = s.Format
						ls.ResponseCondition = s.ResponseCondition
						ls.Placement = s.Placement
					}

					if !reflect.DeepEqual(s, ls) {
						return fmt.Errorf("Bad match Syslog logging match,\nexpected:\n(%#v),\ngot:\n(%#v)", s, ls)
					}
					found++
				}
			}
		}

		if found != len(syslogs) {
			return fmt.Errorf("Error matching Syslog Logging rules")
		}

		return nil
	}
}

func testAccServiceV1SyslogComputeConfig(name, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_compute" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-testing-domain"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  syslog {
    name               = "somesyslogname"
    address            = "127.0.0.1"
  }
  package {
      	filename = "test_fixtures/package/valid.tar.gz"
	  	source_code_hash = filesha512("test_fixtures/package/valid.tar.gz")
   	}
  force_destroy = true
}`, name, domain)
}

func testAccServiceV1SyslogConfig(name, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-testing-domain"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  condition {
    name      = "response_condition_test"
    type      = "RESPONSE"
    priority  = 8
    statement = "resp.status == 418"
  }
  syslog {
    name               = "somesyslogname"
    address            = "127.0.0.1"
    response_condition = "response_condition_test"
  }
  force_destroy = true
}`, name, domain)
}

func testAccServiceV1SyslogConfig_update(name, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-testing-domain"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  condition {
    name      = "response_condition_test"
    type      = "RESPONSE"
    priority  = 8
    statement = "resp.status == 418"
  }
  syslog {
    name               = "somesyslogname"
    address            = "127.0.0.1"
    port               = 514
    response_condition = "response_condition_test"
    message_type       = "blank"
  }
  syslog {
    name    = "somesyslogname2"
    address = "127.0.0.2"
    port    = 10514
  }
  force_destroy = true
}`, name, domain)
}

func testAccServiceV1SyslogConfig_formatVersion(name, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-testing-domain"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  syslog {
    name           = "somesyslogname"
    address        = "127.0.0.1"
    port           = 514
    format         = "%%a %%l %%u %%t %%m %%U%%q %%H %%>s %%b %%T"
    format_version = 2
  }
  force_destroy = true
}`, name, domain)
}

func testAccServiceV1SyslogConfig_useTls(name, domain string) string {
	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"
  domain {
    name    = "%s"
    comment = "tf-testing-domain"
  }
  backend {
    address = "aws.amazon.com"
    name    = "amazon docs"
  }
  syslog {
    name               = "somesyslogname"
    address            = "127.0.0.1"
    port               = 514
    use_tls            = true
    tls_hostname       = "example.com"
  }
  force_destroy = true
}`, name, domain)
}

func setSyslogEnv(key string, cert string, t *testing.T) func() {
	e := getSyslogEnv()
	// Set all the envs to a dummy value
	if err := os.Setenv("FASTLY_SYSLOG_CA_CERT", cert); err != nil {
		t.Fatalf("Error setting env var FASTLY_SYSLOG_CA_CERT: %s", err)
	}
	if err := os.Setenv("FASTLY_SYSLOG_CLIENT_CERT", cert); err != nil {
		t.Fatalf("Error setting env var FASTLY_SYSLOG_CLIENT_CERT: %s", err)
	}
	if err := os.Setenv("FASTLY_SYSLOG_CLIENT_KEY", key); err != nil {
		t.Fatalf("Error setting env var FASTLY_SYSLOG_CLIENT_KEY: %s", err)
	}

	return func() {
		// re-set all the envs we unset above
		if err := os.Setenv("FASTLY_SYSLOG_CA_CERT", e.CaCert); err != nil {
			t.Fatalf("Error resetting env var FASTLY_SYSLOG_CA_CERT: %s", err)
		}
		if err := os.Setenv("FASTLY_SYSLOG_CLIENT_CERT", e.ClientCert); err != nil {
			t.Fatalf("Error resetting env var FASTLY_SYSLOG_CLIENT_CERT: %s", err)
		}
		if err := os.Setenv("FASTLY_SYSLOG_CLIENT_KEY", e.ClientKey); err != nil {
			t.Fatalf("Error resetting env var FASTLY_SYSLOG_CLIENT_KEY: %s", err)
		}
	}
}

// struct to preserve the current environment
type currentSyslogEnv struct {
	CaCert, ClientCert, ClientKey string
}

func getSyslogEnv() *currentSyslogEnv {
	// Grab any existing Fastly Syslog certs and keys and preserve, in the off chance
	// they're actually set in the enviornment
	return &currentSyslogEnv{
		CaCert:     os.Getenv("FASTLY_SYSLOG_CA_CERT"),
		ClientCert: os.Getenv("FASTLY_SYSLOG_CLIENT_CERT"),
		ClientKey:  os.Getenv("FASTLY_SYSLOG_CLIENT_KEY"),
	}
}
