package fastly

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceFastlyFlattenDictionaryItems(t *testing.T) {
	cases := []struct {
		remote []*gofastly.DictionaryItem
		local  map[string]string
	}{
		{
			remote: []*gofastly.DictionaryItem{
				{
					ServiceID:    "service-id",
					DictionaryID: "1234567890",
					ItemKey:      "key-1",
					ItemValue:    "value-1",
				},
				{
					ServiceID:    "service-id",
					DictionaryID: "1234567890",
					ItemKey:      "key-2",
					ItemValue:    "value-2",
				},
			},
			local: map[string]string{
				"key-1": "value-1",
				"key-2": "value-2",
			},
		},
	}

	for _, c := range cases {
		out := flattenDictionaryItems(c.remote)
		if !reflect.DeepEqual(out, c.local) {
			t.Fatalf("Error matching:\nexpected: %#v\ngot: %#v", c.local, out)
		}
	}
}

func TestAccFastlyServiceDictionaryItemV1_create(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
			{
				ResourceName:            "fastly_service_dictionary_items_v1.items",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"manage_items"},
			},
		},
	})
}

// TestAccFastlyServiceDictionaryItemV1_create_inactive_service validates that
// when creating a new inactive service consisting of a dictionary along with a
// predefined list of items to populate it with, are applied successfully
// instead of generating an error suggesting the dictionary ID was missing.
//
// NOTE: This error stemmed from a bug in our code (#345) where we discovered
// that if a configuration has the activate field set to false AND it has no
// previous active version, then the state wasn't being read. This manifested
// itself as a runtime error in certain situations, such as another resource
// referencing the state in its configuration.
func TestAccFastlyServiceDictionaryItemV1_create_inactive_service(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, false, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_create_dynamic(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"alpha": "alpha.demo.notexample.com",
		"beta":  "beta.demo.notexample.com",
		"gamma": "gamma.demo.notexample.com",
		"delta": "delta.demo.notexample.com",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_create_dynamic(name, dictName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.myservice", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.common", "items.%", "4"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_update(t *testing.T) {
	var service gofastly.ServiceDetail
	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	expectedRemoteItemsAfterUpdate := map[string]string{
		"key1": "valueOne",
		"key2": "value2",
		"key3": "value3",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItemsAfterUpdate, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItemsAfterUpdate),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "3"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_external_item_is_removed(t *testing.T) {

	var service gofastly.ServiceDetail

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	config := testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, true, true)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
			{
				PreConfig: func() { createDictionaryItemThroughApi(t, &service, dictName, "key3", "value3") },
				Config:    config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_external_item_deleted(t *testing.T) {

	var service gofastly.ServiceDetail

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	expectedRemoteItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	expectedRemoteItemsAfterUpdate := map[string]string{}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
			{
				PreConfig: func() { createDictionaryItemThroughApi(t, &service, dictName, "key3", "value3") },
				Config:    testAccServiceDictionaryItemsV1Config_one_dictionary_no_items(name, dictName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItemsAfterUpdate),
					testAccCheckFastlyServiceDictionaryItemsV1DoesNotExists("fastly_service_dictionary_items_v1.items"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_batch_1001_items(t *testing.T) {

	var service gofastly.ServiceDetail

	name := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	var expectedRemoteItems = make(map[string]string)
	expectedBatchSize := gofastly.BatchModifyMaximumOperations + 1

	for i := 0; i < expectedBatchSize; i++ {
		expectedRemoteItems[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, expectedRemoteItems, true, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, expectedRemoteItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", strconv.Itoa(expectedBatchSize)),
				),
			},
		},
	})
}

func TestAccFastlyServiceDictionaryItemV1_manage_items_false(t *testing.T) {
	var service gofastly.ServiceDetail
	name := acctest.RandomWithPrefix(testResourcePrefix)
	dictName := fmt.Sprintf("dict %s", acctest.RandString(10))

	initialItems := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	updatedItems := map[string]string{
		"key1": "valueOne",
		"key2": "value2",
		"key3": "value3",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, initialItems, true, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, initialItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
			{
				Config: testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(name, dictName, updatedItems, true, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceDictionaryItemsV1RemoteState(&service, name, dictName, initialItems),
					resource.TestCheckResourceAttr("fastly_service_dictionary_items_v1.items", "items.%", "2"),
				),
			},
		},
	})
}

func testAccCheckFastlyServiceDictionaryItemsV1DoesNotExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[n]
		if ok {
			return fmt.Errorf("Found: %s", n)
		}

		return nil
	}
}

func testAccCheckFastlyServiceDictionaryItemsV1RemoteState(service *gofastly.ServiceDetail, name, dictName string, expectedItems map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if service.Name != name {
			return fmt.Errorf("Bad name, expected (%s), got (%s)", name, service.Name)
		}

		conn := testAccProvider.Meta().(*FastlyClient).conn
		dict, err := conn.GetDictionary(&gofastly.GetDictionaryInput{
			ServiceID:      service.ID,
			ServiceVersion: service.Version.Number,
			Name:           dictName,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Dictionary records for (%s), version (%v): %s", service.Name, service.ActiveVersion.Number, err)
		}

		dictItems, err := conn.ListDictionaryItems(&gofastly.ListDictionaryItemsInput{
			ServiceID:    service.ID,
			DictionaryID: dict.ID,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Dictionary Items records for (%s), dictionary (%s): %s", service.Name, dict.ID, err)
		}

		dictItemsMap := flattenDictionaryItems(dictItems)

		if !reflect.DeepEqual(dictItemsMap, expectedItems) {
			return fmt.Errorf("[ERR] Error matching:\nexpected: %#v\ngot: %#v", expectedItems, dictItemsMap)
		}

		return nil
	}
}

func createDictionaryItemThroughApi(t *testing.T, service *gofastly.ServiceDetail, dictName, expectedKey, expectedValue string) {

	conn := testAccProvider.Meta().(*FastlyClient).conn

	dict, err := getDictionaryByName(service, dictName)

	if err != nil {
		t.Fatalf("[ERR] Error looking up Dictionary records for (%s), version (%v): %s", service.Name, service.ActiveVersion.Number, err)
	}

	_, err = conn.CreateDictionaryItem(&gofastly.CreateDictionaryItemInput{
		ServiceID:    service.ID,
		DictionaryID: dict.ID,

		ItemKey:   expectedKey,
		ItemValue: expectedValue,
	})

	if err != nil {
		t.Fatalf("[ERR] Error Createing Dictionary item for (%s), dictionary (%s): %s", service.Name, dict.Name, err)
	}

}

func getDictionaryByName(service *gofastly.ServiceDetail, dictName string) (*gofastly.Dictionary, error) {
	conn := testAccProvider.Meta().(*FastlyClient).conn

	dict, err := conn.GetDictionary(&gofastly.GetDictionaryInput{
		ServiceID:      service.ID,
		ServiceVersion: service.ActiveVersion.Number,
		Name:           dictName,
	})
	return dict, err
}

func testAccServiceDictionaryItemsV1Config_one_dictionary_with_items(serviceName, dictName string, dictItemsList map[string]string, activate, manageItems bool) string {
	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	var dictItems = "{\n"

	for key, value := range dictItemsList {
		dictItems += fmt.Sprintf("%s: \"%s\"\n", key, value)
	}

	dictItems += "}\n"

	return fmt.Sprintf(`
variable "mydict" {
	type = object({ name=string, items=map(string) })
	default = {
		name = "%s"
		items = %s
	}
}

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

  dictionary {
	name       = var.mydict.name
  }

  activate = %t

  force_destroy = true
}

resource "fastly_service_dictionary_items_v1" "items" {
    service_id = fastly_service_v1.foo.id
    dictionary_id = {for s in fastly_service_v1.foo.dictionary : s.name => s.dictionary_id}[var.mydict.name]
	manage_items = %t
    items = var.mydict.items
}`, dictName, dictItems, serviceName, domainName, backendName, activate, manageItems)
}

func testAccServiceDictionaryItemsV1Config_one_dictionary_no_items(serviceName, dictName string) string {

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

  dictionary {
	name       = "%s"
  }

  force_destroy = true
}`, serviceName, domainName, backendName, dictName)
}

func testAccServiceDictionaryItemsV1Config_create_dynamic(serviceName, dictName string) string {

	backendName := fmt.Sprintf("%s.aws.amazon.com", acctest.RandString(3))
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
locals {
  dictionary_name = "%s"
  host_base = "demo.notexample.com"
  host_divisions = ["alpha", "beta", "gamma", "delta"]
}

resource "fastly_service_v1" "myservice" {
  name = "%s"

  domain {
    name    = "%s"
    comment = "tf-testing-domain"
	}

  backend {
    address = "%s"
    name    = "tf -test backend"
  }

  dictionary {
    name       = local.dictionary_name
  }

  force_destroy = true
}

resource "fastly_service_dictionary_items_v1" "common" {
  service_id = fastly_service_v1.myservice.id
  dictionary_id = {for d in fastly_service_v1.myservice.dictionary : d.name => d.dictionary_id}[local.dictionary_name]
  items = {
    for division in local.host_divisions:
      division => format("%%s.%%s", division, local.host_base)
  }

}`, dictName, serviceName, domainName, backendName)
}
