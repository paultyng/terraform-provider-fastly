package fastly

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	gofastly "github.com/fastly/go-fastly/v3/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceFastlyFlattenBlobStorage(t *testing.T) {
	cases := []struct {
		remote []*gofastly.BlobStorage
		local  []map[string]interface{}
	}{
		{
			remote: []*gofastly.BlobStorage{
				{
					Name:              "test-blobstorage",
					Path:              "/logs/",
					AccountName:       "test",
					Container:         "fastly",
					SASToken:          "test-sas-token",
					Period:            12,
					TimestampFormat:   "%Y-%m-%dT%H:%M:%S.000",
					GzipLevel:         0,
					PublicKey:         "test-public-key",
					Format:            "%h %l %u %t \"%r\" %>s %b",
					FormatVersion:     2,
					MessageType:       "classic",
					Placement:         "waf_debug",
					ResponseCondition: "error_response",
					FileMaxBytes:      1048576,
					CompressionCodec:  "zstd",
				},
			},
			local: []map[string]interface{}{
				{
					"name":               "test-blobstorage",
					"path":               "/logs/",
					"account_name":       "test",
					"container":          "fastly",
					"sas_token":          "test-sas-token",
					"period":             uint(12),
					"timestamp_format":   "%Y-%m-%dT%H:%M:%S.000",
					"public_key":         "test-public-key",
					"format":             "%h %l %u %t \"%r\" %>s %b",
					"format_version":     uint(2),
					"gzip_level":         uint(0),
					"message_type":       "classic",
					"placement":          "waf_debug",
					"response_condition": "error_response",
					"file_max_bytes":     uint(1048576),
					"compression_codec":  "zstd",
				},
			},
		},
	}

	for _, c := range cases {
		out := flattenBlobStorages(c.remote)
		if !reflect.DeepEqual(out, c.local) {
			t.Fatalf("Error matching:\nexpected: %#v\ngot: %#v", c.local, out)
		}
	}
}

func TestAccFastlyServiceV1_blobstoragelogging_basic(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	blobStorageLogOne := gofastly.BlobStorage{
		Name:              "test-blobstorage-1",
		Path:              "/5XX/",
		AccountName:       "test",
		Container:         "fastly",
		SASToken:          "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:            12,
		TimestampFormat:   "%Y-%m-%dT%H:%M:%S.000",
		PublicKey:         pgpPublicKey(t),
		Format:            "%h %l %u %t \"%r\" %>s %b",
		FormatVersion:     1,
		MessageType:       "blank",
		Placement:         "waf_debug",
		ResponseCondition: "error_response_5XX",
		FileMaxBytes:      1048576,
		CompressionCodec:  "zstd",
	}

	blobStorageLogOneUpdated := gofastly.BlobStorage{
		Name:              "test-blobstorage-1",
		Path:              "/5XX/",
		AccountName:       "test",
		Container:         "fastly",
		SASToken:          "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:            12,
		TimestampFormat:   "%Y-%m-%dT%H:%M:%S.000",
		PublicKey:         pgpPublicKey(t),
		Format:            "%h %l %u %{now}V %{req.method}V %{req.url}V %>s %{resp.http.Content-Length}V",
		FormatVersion:     2,
		GzipLevel:         1,
		MessageType:       "blank",
		Placement:         "waf_debug",
		ResponseCondition: "error_response_5XX",
		FileMaxBytes:      1048576,
	}

	blobStorageLogTwo := gofastly.BlobStorage{
		Name:              "test-blobstorage-2",
		Path:              "/2XX/",
		AccountName:       "test",
		Container:         "fastly",
		SASToken:          "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:            12,
		TimestampFormat:   "%Y-%m-%dT%H:%M:%S.000",
		PublicKey:         pgpPublicKey(t),
		Format:            "%h %l %u %{now}V %{req.method}V %{req.url}V %>s %{resp.http.Content-Length}V",
		FormatVersion:     2,
		MessageType:       "blank",
		Placement:         "waf_debug",
		ResponseCondition: "ok_response_2XX",
		FileMaxBytes:      2097152,
		CompressionCodec:  "zstd",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1BlobStorageLoggingConfig_complete(serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(&service, []*gofastly.BlobStorage{&blobStorageLogOne}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", serviceName),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "blobstoragelogging.#", "1"),
				),
			},

			{
				Config: testAccServiceV1BlobStorageLoggingConfig_update(serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(&service, []*gofastly.BlobStorage{&blobStorageLogOneUpdated, &blobStorageLogTwo}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", serviceName),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "blobstoragelogging.#", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceV1_blobstoragelogging_basic_compute(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	blobStorageLogOne := gofastly.BlobStorage{
		Name:             "test-blobstorage-1",
		Path:             "/5XX/",
		AccountName:      "test",
		Container:        "fastly",
		SASToken:         "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:           12,
		TimestampFormat:  "%Y-%m-%dT%H:%M:%S.000",
		PublicKey:        pgpPublicKey(t),
		MessageType:      "blank",
		FileMaxBytes:     1048576,
		CompressionCodec: "zstd",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1BlobStorageLoggingConfig_complete_compute(serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_compute.foo", &service),
					testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(&service, []*gofastly.BlobStorage{&blobStorageLogOne}, ServiceTypeCompute),
					resource.TestCheckResourceAttr(
						"fastly_service_compute.foo", "name", serviceName),
					resource.TestCheckResourceAttr(
						"fastly_service_compute.foo", "blobstoragelogging.#", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceV1_blobstoragelogging_default(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	blobStorageLog := gofastly.BlobStorage{
		Name:            "test-blobstorage",
		AccountName:     "test",
		Container:       "fastly",
		SASToken:        "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:          3600,
		TimestampFormat: "%Y-%m-%dT%H:%M:%S.000",
		Format:          "%h %l %u %t \"%r\" %>s %b",
		FormatVersion:   2,
		MessageType:     "classic",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1BlobStorageLoggingConfig_default(serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(&service, []*gofastly.BlobStorage{&blobStorageLog}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", serviceName),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "blobstoragelogging.#", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceV1_blobstoragelogging_env(t *testing.T) {
	var service gofastly.ServiceDetail
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))

	// set env variable to something we expect
	resetEnv := setBlobStorageEnv("sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D", t)
	defer resetEnv()

	blobStorageLog := gofastly.BlobStorage{
		Name:            "test-blobstorage",
		AccountName:     "test",
		Container:       "fastly",
		SASToken:        "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%3A00%3A00Z&sig=3ABdLOJZosCp0o491T%2BqZGKIhafF1nlM3MzESDDD3Gg%3D",
		Period:          3600,
		TimestampFormat: "%Y-%m-%dT%H:%M:%S.000",
		Format:          "%h %l %u %t \"%r\" %>s %b",
		FormatVersion:   2,
		MessageType:     "classic",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceV1BlobStorageLoggingConfig_env(serviceName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceV1Exists("fastly_service_v1.foo", &service),
					testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(&service, []*gofastly.BlobStorage{&blobStorageLog}, ServiceTypeVCL),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "name", serviceName),
					resource.TestCheckResourceAttr(
						"fastly_service_v1.foo", "blobstoragelogging.#", "1"),
				),
			},
		},
	})
}

func testAccCheckFastlyServiceV1BlobStorageLoggingAttributes(service *gofastly.ServiceDetail, localBlobStorageList []*gofastly.BlobStorage, serviceType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		conn := testAccProvider.Meta().(*FastlyClient).conn
		remoteBlobStorageList, err := conn.ListBlobStorages(&gofastly.ListBlobStoragesInput{
			ServiceID:      service.ID,
			ServiceVersion: service.ActiveVersion.Number,
		})

		if err != nil {
			return fmt.Errorf("[ERR] Error looking up Blob Storage Logging for (%s), version (%v): %s", service.Name, service.ActiveVersion.Number, err)
		}

		if len(remoteBlobStorageList) != len(localBlobStorageList) {
			return fmt.Errorf("Blob Storage List count mismatch, expected (%d), got (%d)", len(localBlobStorageList), len(remoteBlobStorageList))
		}

		var found int
		for _, lbs := range localBlobStorageList {
			for _, rbs := range remoteBlobStorageList {
				if lbs.Name == rbs.Name {
					// we don't know these things ahead of time, so populate them now
					lbs.ServiceID = service.ID
					lbs.ServiceVersion = service.ActiveVersion.Number

					// Ignore VCL attributes for Compute and set to whatever is returned from the API.
					if serviceType == ServiceTypeCompute {
						lbs.FormatVersion = rbs.FormatVersion
						lbs.Format = rbs.Format
						lbs.ResponseCondition = rbs.ResponseCondition
						lbs.Placement = rbs.Placement
					}

					// We don't track these, so clear them out because we also wont know
					// these ahead of time
					rbs.CreatedAt = nil
					rbs.UpdatedAt = nil
					if !reflect.DeepEqual(lbs, rbs) {
						return fmt.Errorf("Bad match Blob Storage logging match, expected (%#v), got (%#v)", lbs, rbs)
					}
					found++
				}
			}
		}

		if found != len(localBlobStorageList) {
			return fmt.Errorf("Error matching Blob Storage Logging rules")
		}

		return nil
	}
}

func testAccServiceV1BlobStorageLoggingConfig_complete(serviceName string) string {
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))
	format := "%h %l %u %t \"%r\" %>s %b"

	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"

  domain {
    name = "%s"
    comment = "tf-testing-domain"
  }

  backend {
    address = "aws.amazon.com"
    name = "tf-test-backend"
  }

  condition {
    name = "error_response_5XX"
    statement = "resp.status >= 500 && resp.status < 600"
    priority = 10
    type = "RESPONSE"
  }

  blobstoragelogging {
    name = "test-blobstorage-1"
    path = "/5XX/"
    account_name = "test"
    container = "fastly"
    sas_token = "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%%3A00%%3A00Z&sig=3ABdLOJZosCp0o491T%%2BqZGKIhafF1nlM3MzESDDD3Gg%%3D"
    period = 12
    timestamp_format = "%%Y-%%m-%%dT%%H:%%M:%%S.000"
    public_key = file("test_fixtures/fastly_test_publickey")
    format = %q
    format_version = 1
    message_type = "blank"
    placement = "waf_debug"
    response_condition = "error_response_5XX"
    file_max_bytes     = 1048576
    compression_codec = "zstd"
  }

  force_destroy = true
}`, serviceName, domainName, format)
}

func testAccServiceV1BlobStorageLoggingConfig_complete_compute(serviceName string) string {
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
resource "fastly_service_compute" "foo" {
  name = "%s"

  domain {
    name = "%s"
    comment = "tf-testing-domain"
  }

  backend {
    address = "aws.amazon.com"
    name = "tf-test-backend"
  }

  blobstoragelogging {
    name = "test-blobstorage-1"
    path = "/5XX/"
    account_name = "test"
    container = "fastly"
    sas_token = "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%%3A00%%3A00Z&sig=3ABdLOJZosCp0o491T%%2BqZGKIhafF1nlM3MzESDDD3Gg%%3D"
    period = 12
    timestamp_format = "%%Y-%%m-%%dT%%H:%%M:%%S.000"
    public_key = file("test_fixtures/fastly_test_publickey")
    message_type = "blank"
    file_max_bytes     = 1048576
    compression_codec = "zstd"
  }

  package {
    filename = "test_fixtures/package/valid.tar.gz"
    source_code_hash = filesha512("test_fixtures/package/valid.tar.gz")
  }

  force_destroy = true
}`, serviceName, domainName)
}

func testAccServiceV1BlobStorageLoggingConfig_update(serviceName string) string {
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))
	format := "%h %l %u %%{now}V %%{req.method}V %%{req.url}V %>s %%{resp.http.Content-Length}V"

	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"

  domain {
    name = "%s"
    comment = "tf-testing-domain"
  }

  backend {
    address = "aws.amazon.com"
    name = "tf-test-backend"
  }

  condition {
    name = "error_response_5XX"
    statement = "resp.status >= 500 && resp.status < 600"
    priority = 10
    type = "RESPONSE"
  }

  condition {
    name = "ok_response_2XX"
    statement = "resp.status >= 200 && resp.status < 300"
    priority = 10
    type = "RESPONSE"
  }

  blobstoragelogging {
    name = "test-blobstorage-1"
    path = "/5XX/"
    account_name = "test"
    container = "fastly"
    sas_token = "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%%3A00%%3A00Z&sig=3ABdLOJZosCp0o491T%%2BqZGKIhafF1nlM3MzESDDD3Gg%%3D"
    period = 12
    timestamp_format = "%%Y-%%m-%%dT%%H:%%M:%%S.000"
    public_key = file("test_fixtures/fastly_test_publickey")
    format = %q
    gzip_level = 1
    format_version = 2
    message_type = "blank"
    placement = "waf_debug"
    response_condition = "error_response_5XX"
    file_max_bytes     = 1048576
  }

  blobstoragelogging {
    name = "test-blobstorage-2"
    path = "/2XX/"
    account_name = "test"
    container = "fastly"
    sas_token = "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%%3A00%%3A00Z&sig=3ABdLOJZosCp0o491T%%2BqZGKIhafF1nlM3MzESDDD3Gg%%3D"
    period = 12
    timestamp_format = "%%Y-%%m-%%dT%%H:%%M:%%S.000"
    public_key = file("test_fixtures/fastly_test_publickey")
    format = %q
    format_version = 2
    message_type = "blank"
    placement = "waf_debug"
    response_condition = "ok_response_2XX"
    file_max_bytes     = 2097152
    compression_codec  = "zstd"
  }

  force_destroy = true
}`, serviceName, domainName, format, format)
}

func testAccServiceV1BlobStorageLoggingConfig_default(serviceName string) string {
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"

  domain {
    name = "%s"
    comment = "tf-testing-domain"
  }

  backend {
    address = "aws.amazon.com"
    name = "tf-test-backend"
  }

  blobstoragelogging {
    name = "test-blobstorage"
    account_name = "test"
    container = "fastly"
    sas_token = "sv=2018-04-05&ss=b&srt=sco&sp=rw&se=2050-07-21T18%%3A00%%3A00Z&sig=3ABdLOJZosCp0o491T%%2BqZGKIhafF1nlM3MzESDDD3Gg%%3D"
  }

  force_destroy = true
}`, serviceName, domainName)
}

func testAccServiceV1BlobStorageLoggingConfig_env(serviceName string) string {
	domainName := fmt.Sprintf("fastly-test.tf-%s.com", acctest.RandString(10))

	return fmt.Sprintf(`
resource "fastly_service_v1" "foo" {
  name = "%s"

  domain {
    name = "%s"
    comment = "tf-testing-domain"
  }

  backend {
    address = "aws.amazon.com"
    name = "tf-test-backend"
  }

  blobstoragelogging {
    name = "test-blobstorage"
    account_name = "test"
    container = "fastly"
  }

  force_destroy = true
}`, serviceName, domainName)
}

func setBlobStorageEnv(sas string, t *testing.T) func() {
	e := getBlobStorageEnv()
	// Set all the envs to a dummy value
	if err := os.Setenv("FASTLY_AZURE_SHARED_ACCESS_SIGNATURE", sas); err != nil {
		t.Fatalf("Error setting env var FASTLY_AZURE_SHARED_ACCESS_SIGNATURE: %s", err)
	}

	return func() {
		// re-set all the envs we unset above
		if err := os.Setenv("FASTLY_AZURE_SHARED_ACCESS_SIGNATURE", e.SASToken); err != nil {
			t.Fatalf("Error resetting env var FASTLY_AZURE_SHARED_ACCESS_SIGNATURE: %s", err)
		}
	}
}

// struct to preserve the current environment
type currentBlobStorageEnv struct {
	SASToken string
}

func getBlobStorageEnv() *currentBlobStorageEnv {
	// Grab the existing Fastly Azure SAS token and preserve, in the off chance
	// they're actually set in the enviornment
	return &currentBlobStorageEnv{
		SASToken: os.Getenv("FASTLY_AZURE_SHARED_ACCESS_SIGNATURE"),
	}
}
