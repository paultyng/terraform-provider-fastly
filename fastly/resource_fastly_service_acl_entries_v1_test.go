package fastly

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"testing"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

func TestAccFastlyServiceAclEntriesV1_create_update_import(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("ACL %s", acctest.RandString(10))

	expectedRemoteEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 1",
		},
	}

	expectedRemoteEntriesAfterUpdate := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.2",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 1",
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, expectedRemoteEntries, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, expectedRemoteEntriesAfterUpdate, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntriesAfterUpdate),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				ResourceName:            "fastly_service_acl_entries_v1.entries",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"manage_entries"},
			},
		},
	})
}

func TestAccFastlyServiceAclEntriesV1_update_additional_fields(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := "ACL Test Update Negated Field"

	expectedRemoteEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 1",
		},
	}

	expectedRemoteEntriesAfterUpdate := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "20",
			"negated": true,
			"comment": "ACL Entry 1 Updated",
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, expectedRemoteEntries, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_service_acl_entries_v1.entries", "entry.*", map[string]string{
						"ip":      "127.0.0.1",
						"subnet":  "24",
						"negated": "false",
						"comment": "ACL Entry 1",
					}),
				),
			},
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, expectedRemoteEntriesAfterUpdate, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntriesAfterUpdate),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("fastly_service_acl_entries_v1.entries", "entry.*", map[string]string{
						"ip":      "127.0.0.1",
						"subnet":  "20",
						"negated": "true",
						"comment": "ACL Entry 1 Updated",
					}),
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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, expectedRemoteEntries, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				Config: testAccServiceDictionaryItemsV1Config_one_acl_no_entries(serviceName, aclName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					resource.TestCheckNoResourceAttr("fastly_service_v1.foo", "entry"),
				),
			},
		},
	})
}

func TestAccFastlyServiceAclEntriesV1_process_1001_entries(t *testing.T) {

	var service gofastly.ServiceDetail

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("acl %s", acctest.RandString(10))

	expectedBatchSize := gofastly.BatchModifyMaximumOperations + 1

	expectedRemoteEntries := make([]map[string]interface{}, 0)

	ipPart3 := 0
	ipPart4 := 1
	for i := 0; i < expectedBatchSize; i++ {

		if ipPart4 > 254 {
			ipPart3++
			ipPart4 = 1
		}

		expectedRemoteEntries = append(expectedRemoteEntries, map[string]interface{}{
			"id":      "",
			"ip":      fmt.Sprintf("127.0.%d.%d", ipPart3, ipPart4),
			"subnet":  "22",
			"negated": false,
			"comment": fmt.Sprintf("ALC Entry %d %d", ipPart3, ipPart4),
		})

		ipPart4++
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(name, aclName, expectedRemoteEntries, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, name, aclName, expectedRemoteEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", strconv.Itoa(expectedBatchSize)),
				),
			},
		},
	})
}

func TestAccFastlyServiceAclEntriesV1_manage_entries_false(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	aclName := fmt.Sprintf("ACL %s", acctest.RandString(10))

	initialEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 1",
		},
	}

	updatedEntries := []map[string]interface{}{
		{
			"id":      "",
			"ip":      "127.0.0.1",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 1",
		},
		{
			"id":      "",
			"ip":      "127.0.0.2",
			"subnet":  "24",
			"negated": false,
			"comment": "ACL Entry 2",
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, initialEntries, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, initialEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
				),
			},
			{
				Config: testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName, updatedEntries, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceAclEntriesV1RemoteState(&service, serviceName, aclName, initialEntries),
					resource.TestCheckResourceAttr("fastly_service_acl_entries_v1.entries", "entry.#", "1"),
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
			ServiceID:      service.ID,
			ServiceVersion: service.ActiveVersion.Number,
			Name:           aclName,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up ACL records for (%s), version (%v): %s", service.Name, service.ActiveVersion.Number, err)
		}

		aclEntries, err := conn.ListACLEntries(&gofastly.ListACLEntriesInput{
			ServiceID: service.ID,
			ACLID:     acl.ID,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up ACL entry records for (%s), ACL (%s): %s", service.Name, acl.ID, err)
		}

		flatAclEntries := flattenAclEntries(aclEntries)
		// Clear out the id values to allow a deep equal of the other attributes set in terraform.
		for _, val := range flatAclEntries {
			val["id"] = ""
		}

		sort.Slice(flatAclEntries, func(i, j int) bool {
			return flatAclEntries[i]["ip"].(string) < flatAclEntries[j]["ip"].(string)
		})

		sort.Slice(expectedEntries, func(i, j int) bool {
			return expectedEntries[i]["ip"].(string) < expectedEntries[j]["ip"].(string)
		})

		if !reflect.DeepEqual(flatAclEntries, expectedEntries) {
			return fmt.Errorf("[ERR] Error matching:\nexpected: %#v\ngot: %#v", expectedEntries, flatAclEntries)
		}

		return nil
	}
}

func testAccServiceDictionaryItemsV1Config_one_acl_no_entries(serviceName, aclName string) string {

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
    name    = "tf -test backend"
  }
  acl {
	name       = "%s"
  }
  force_destroy = true
}`, serviceName, domainName, backendName, aclName)
}

func testAccServiceACLEntriesV1Config_one_acl_with_entries(serviceName, aclName string, aclEntriesList []map[string]interface{}, manageEntries bool) string {
	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	aclEntries := ""

	for _, val := range aclEntriesList {
		aclEntries += "entry {\n"
		aclEntries += fmt.Sprintf("ip = \"%s\"\n", val["ip"])
		aclEntries += fmt.Sprintf("subnet = \"%s\"\n", val["subnet"])
		aclEntries += fmt.Sprintf("negated = %t\n", val["negated"])
		aclEntries += fmt.Sprintf("comment = \"%s\"\n", val["comment"])
		aclEntries += "}\n"
	}

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
	manage_entries = %t
	%s
}`, aclName, serviceName, domainName, backendName, manageEntries, aclEntries)
}
