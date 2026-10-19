package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOperationApiKeyResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	keyName := fmt.Sprintf("test-%s", randomString(10))
	// The API rejects an expiration later than the provider's own key, so stay close.
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	updatedExpiresAt := time.Now().Add(12 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)

	// Asserts the key id is identical across the steps it's registered in, so an update
	// never recreates the key, which would issue a new secret and break its clients.
	keyID := statecheck.CompareValue(compare.ValuesSame())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_operation_api_key" "test" {
  name       = "%s"
  owner      = tsuga_team.test.id
  scope      = "owning-team-only"
  expires_at = "%s"
  permissions = {
    dashboard = "write"
    monitor   = "write"
    logs      = "read"
  }
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, keyName, expiresAt),
				ConfigStateChecks: []statecheck.StateCheck{
					keyID.AddStateValue("tsuga_operation_api_key.test", tfjsonpath.New("id")),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "name", keyName),
					resource.TestCheckResourceAttrSet("tsuga_operation_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("tsuga_operation_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("tsuga_operation_api_key.test", "key_last_characters"),
					resource.TestCheckResourceAttrPair("tsuga_operation_api_key.test", "owner", "tsuga_team.test", "id"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "scope", "owning-team-only"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "expires_at", expiresAt),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.%", "3"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.dashboard", "write"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.monitor", "write"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.logs", "read"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.0.value", "dev"),
				),
			},
			// ImportState — key is not returned by GET, and the API normalizes expires_at
			{
				ResourceName:            "tsuga_operation_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key", "expires_at"},
			},
			// Update scope, expiration and tags, and lower monitor to read.
			// Permissions stay within the provider key's own, which the API enforces.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_operation_api_key" "test" {
  name       = "%s-updated"
  owner      = tsuga_team.test.id
  scope      = "owning-team-and-public"
  expires_at = "%s"
  permissions = {
    dashboard = "write"
    monitor   = "read"
    logs      = "read"
  }
  tags = [
    {
      key   = "env"
      value = "prod"
    }
  ]
}
`, teamName, keyName, updatedExpiresAt),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_operation_api_key.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Same id as the create step -> the key was updated in place, not recreated.
					keyID.AddStateValue("tsuga_operation_api_key.test", tfjsonpath.New("id")),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "name", keyName+"-updated"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "scope", "owning-team-and-public"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.%", "3"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.dashboard", "write"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.monitor", "read"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "permissions.logs", "read"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.#", "1"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_operation_api_key.test", "tags.0.value", "prod"),
					// key must be preserved across updates
					resource.TestCheckResourceAttrSet("tsuga_operation_api_key.test", "key"),
				),
			},
		},
	})
}
