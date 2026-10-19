package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRumPublicTokenResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	tokenName := fmt.Sprintf("test-%s", randomString(10))

	// Asserts the token id differs between the steps it's registered in, so changing
	// the application type recreates the token instead of updating it in place.
	tokenID := statecheck.CompareValue(compare.ValuesDiffer())

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

resource "tsuga_rum_public_token" "test" {
  name                 = "%s"
  owner                = tsuga_team.test.id
  application_type     = "web"
  allowed_url_prefixes = ["https://app.example.com"]
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, tokenName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "name", tokenName),
					resource.TestCheckResourceAttrSet("tsuga_rum_public_token.test", "id"),
					resource.TestCheckResourceAttrSet("tsuga_rum_public_token.test", "token"),
					resource.TestCheckResourceAttrPair("tsuga_rum_public_token.test", "owner", "tsuga_team.test", "id"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "application_type", "web"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "allowed_url_prefixes.0", "https://app.example.com"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "tags.0.value", "dev"),
				),
			},
			// ImportState — the token is returned by GET, so nothing is ignored
			{
				ResourceName:      "tsuga_rum_public_token.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update name, prefixes and team override fields in place
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_rum_public_token" "test" {
  name                 = "%s-updated"
  owner                = tsuga_team.test.id
  application_type     = "web"
  allowed_url_prefixes = ["https://app.example.com", "https://*.example.com"]
  team_override_fields = ["service"]
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, tokenName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_rum_public_token.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					tokenID.AddStateValue("tsuga_rum_public_token.test", tfjsonpath.New("id")),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "name", tokenName+"-updated"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "allowed_url_prefixes.#", "2"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "team_override_fields.0", "service"),
					resource.TestCheckResourceAttrSet("tsuga_rum_public_token.test", "token"),
				),
			},
			// Changing the application type replaces the token
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_rum_public_token" "test" {
  name                 = "%s-updated"
  owner                = tsuga_team.test.id
  application_type     = "ios"
  allowed_url_prefixes = []
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, tokenName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_rum_public_token.test", plancheck.ResourceActionReplace),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Different id from the previous step -> the token was recreated.
					tokenID.AddStateValue("tsuga_rum_public_token.test", tfjsonpath.New("id")),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "application_type", "ios"),
					resource.TestCheckResourceAttr("tsuga_rum_public_token.test", "allowed_url_prefixes.#", "0"),
					resource.TestCheckNoResourceAttr("tsuga_rum_public_token.test", "team_override_fields"),
				),
			},
		},
	})
}
