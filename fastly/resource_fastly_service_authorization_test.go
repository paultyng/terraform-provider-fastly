package fastly

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"

	gofastly "github.com/fastly/go-fastly/v6/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccFastlyServiceAuthorization_basic(t *testing.T) {
	var sa gofastly.ServiceAuthorization
	permission := "purge_select"
	permission2 := "purge_all"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckServiceAuthorizationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAuthorizationConfig(permission),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceAuthorizationExists("fastly_service_authorization.auth", &sa),
					resource.TestCheckResourceAttr(
						"fastly_service_authorization.auth", "permission", permission),
				),
			},
			{
				Config: testAccServiceAuthorizationConfig(permission2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceAuthorizationExists("fastly_service_authorization.auth", &sa),
					resource.TestCheckResourceAttr(
						"fastly_service_authorization.auth", "permission", permission2),
				),
			},
		},
	})
}

func testAccCheckServiceAuthorizationExists(n string, sa *gofastly.ServiceAuthorization) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No User ID is set")
		}

		conn := testAccProvider.Meta().(*APIClient).conn
		latest, err := conn.GetServiceAuthorization(&gofastly.GetServiceAuthorizationInput{
			ID: rs.Primary.ID,
		})
		if err != nil {
			return err
		}

		*sa = *latest

		return nil
	}
}

func testAccCheckServiceAuthorizationDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_service_authorization" {
			continue
		}

		conn := testAccProvider.Meta().(*APIClient).conn
		_, err := conn.GetServiceAuthorization(&gofastly.GetServiceAuthorizationInput{
			ID: rs.Primary.ID,
		})
		if err == nil {
			return fmt.Errorf("[WARN] Tried deleting service authorization (%s), but it still exists.", rs.Primary.ID)
		}
	}
	return nil
}

func testAccServiceAuthorizationConfig(permission string) string {
	tf := `
resource "fastly_service_vcl" "demo" {
	name = "demo"

	domain {
		name = "%s.com"
	}

	force_destroy = true
}

resource "fastly_user" "user" {
	login = "tf-test@example.com"
	name = "tf-test"
	role = "engineer"
}

resource "fastly_service_authorization" "auth" {
	service_id = fastly_service_vcl.demo.id
	user_id    = fastly_user.user.id
	permission = "%s"
}
`
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf(tf, hex.EncodeToString(b), permission)
}
