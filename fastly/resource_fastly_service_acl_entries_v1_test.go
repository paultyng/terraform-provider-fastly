package fastly

import (
	"fmt"
	gofastly "github.com/fastly/go-fastly/fastly"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
	"reflect"
	"testing"
)

func TestResourceFastlyFlattenAclEntries(t *testing.T) {
	cases := []struct {
		remote []*gofastly.ACLEntry
		local  []map[string]interface{}
	}{
		{
			remote: []*gofastly.ACLEntry{
				{
					ServiceID: "service-id",
					ACLID:     "1234567890",
					IP:        "127.0.0.1",
					Subnet:    "24",
					Negated:   false,
					Comment:   "ALC Entry 1",
				},
				{
					ServiceID: "service-id",
					ACLID:     "0987654321",
					IP:        "192.168.0.1",
					Subnet:    "16",
					Negated:   true,
					Comment:   "ALC Entry 2",
				},
			},
			local: []map[string]interface{}{
				{
					"ip":      "127.0.0.1",
					"subnet":  "24",
					"negated": false,
					"comment": "ALC Entry 1",
				},
				{
					"ip":      "192.168.0.1",
					"subnet":  "16",
					"negated": true,
					"comment": "ALC Entry 2",
				},
			},
		},
	}

	for _, c := range cases {
		out := flattenAclEntries(c.remote)
		if !reflect.DeepEqual(out, c.local) {
			t.Fatalf("Error matching:\nexpected: %#v\ngot: %#v", c.local, out)
		}
	}
}

func TestAccFastlyServiceAclEntriesV1_create(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("ACL %s", acctest.RandString(10))

	expectedRemoteEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ALC Entry 1",
		},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAclEntriesV1Config_create(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceAclEntriesV1_update(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("ACL %s", acctest.RandString(10))

	expectedRemoteEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ALC Entry 1",
		},
	}

	expectedRemoteEntriesAfterUpdate := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.2",
			"subnet":  "24",
			"negated": false,
			"comment": "ALC Entry 1",
		},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAclEntriesV1Config_create(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				Config: testAccServiceAclEntriesV1Config_update(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntriesAfterUpdate),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceAclEntriesV1_delete(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("ACL %s", acctest.RandString(10))

	expectedRemoteEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ALC Entry 1",
		},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAclEntriesV1Config_create(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				Config: testAccServiceAclEntriesV1Config_delete(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					resource.TestCheckNoResourceAttr("fastly_service_v1.foo", "entry"),
				),
			},
		},
	})
}

func testAccCheckFastlyServiceAclEntriesV1RemoteState(service *gofastly.ServiceDetail, serviceName, aclName string, expectedEntries []map[string]interface{}) resource.TestCheckFunc {

	return func(s *terraform.State) error {

		if service.Name != serviceName {
			return fmt.Errorf("[ERR] Bad name, expected (%s), got (%s)", serviceName, service.Name)
		}

		conn := testAccProvider.Meta().(*FastlyClient).conn
		acl, err := conn.GetACL(&gofastly.GetACLInput{
			Service: service.ID,
			Version: service.ActiveVersion.Number,
			Name:    aclName,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up ACL records for (%s), version (%v): %s", service.Name, service.ActiveVersion.Number, err)
		}

		aclEntries, err := conn.ListACLEntries(&gofastly.ListACLEntriesInput{
			Service: service.ID,
			ACL:     acl.ID,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up ACL entry records for (%s), ACL (%s): %s", service.Name, acl.ID, err)
		}

		flatAclEntries := flattenAclEntries(aclEntries)
		// Clear out the id values to allow a deep equal of the other attributes set in terraform.
		for _, val := range flatAclEntries {
			val["id"] = ""
		}

		if !reflect.DeepEqual(flatAclEntries, expectedEntries) {
			return fmt.Errorf("[ERR] Error matching:\nexpected: %#v\ngot: %#v", expectedEntries, flatAclEntries)
		}

		return nil
	}
}

func testAccServiceAclEntriesV1Config_create(serviceName, aclName string) string {
	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
variable "myacl_name" {
	type = string
	default = "%s"
}

resource "fastly_service_v1" "foo" {
	name = "%s"
	domain {
		name    = "%s"
		comment = "tf-testing-domain"
	}
	backend {
		address = "%s"
		name    = "tf-testing-backend"
	}
	acl {
		name       = var.myacl_name
	}
	force_destroy = true
}
 resource "fastly_service_acl_entries_v1" "entries" {
	service_id = fastly_service_v1.foo.id
	acl_id = {for s in fastly_service_v1.foo.acl : s.name => s.acl_id}[var.myacl_name]
	entry {
		ip = "127.0.0.1"
		subnet = "24"
		negated = false
		comment = "ALC Entry 1"
	}
}`, aclName, serviceName, domainName, backendName)
}

func testAccServiceAclEntriesV1Config_update(serviceName, aclName string) string {
	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
variable "myacl_name" {
	type = string
	default = "%s"
}

resource "fastly_service_v1" "foo" {
	name = "%s"
	domain {
		name    = "%s"
		comment = "tf-testing-domain"
	}
	backend {
		address = "%s"
		name    = "tf-testing-backend"
	}
	acl {
		name       = var.myacl_name
	}
	force_destroy = true
}
 resource "fastly_service_acl_entries_v1" "entries" {
	service_id = fastly_service_v1.foo.id
	acl_id = {for s in fastly_service_v1.foo.acl : s.name => s.acl_id}[var.myacl_name]
	entry {
		ip = "127.0.0.2"
		subnet = "24"
		negated = false
		comment = "ALC Entry 1"
	}
}`, aclName, serviceName, domainName, backendName)
}

func testAccServiceAclEntriesV1Config_delete(serviceName, aclName string) string {
	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
	name = "%s"
	domain {
		name    = "%s"
		comment = "tf-testing-domain"
	}
	backend {
		address = "%s"
		name    = "tf-testing-backend"
	}

	force_destroy = true
}`, serviceName, domainName, backendName)
}
