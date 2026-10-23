package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccCustomRoleResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	roleName := fmt.Sprintf("test-%s", randomString(10))
	teamConfig := fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}
`, teamName)

	// Asserts the role id is identical across steps, so an update never recreates the role,
	// which would fail while users hold it.
	roleID := statecheck.CompareValue(compare.ValuesSame())
	importStep := resource.TestStep{
		ResourceName:      "tsuga_custom_role.test",
		ImportState:       true,
		ImportStateVerify: true,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create. The conditional on the team id, unknown until the team exists, makes the whole
			// team_scope object unknown on the first plan, which config validation must tolerate.
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "tsuga_custom_role" "test" {
  name        = "%s"
  description = "Edits dashboards of the test team"
  team_scope = tsuga_team.test.id != "" ? {
    type     = "specific"
    team_ids = [tsuga_team.test.id]
    } : {
    type     = "all"
    team_ids = null
  }
  assets = {
    team      = "read"
    dashboard = "write"
    monitor   = "read"
  }
  telemetry = ["trace", "log"]
  telemetry_query = [
    {
      key    = "context.env"
      values = ["staging"]
    }
  ]
}
`, roleName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue("tsuga_custom_role.test", tfjsonpath.New("team_scope")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					roleID.AddStateValue("tsuga_custom_role.test", tfjsonpath.New("id")),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("tsuga_custom_role.test", "id"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "name", roleName),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "description", "Edits dashboards of the test team"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "team_scope.type", "specific"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "team_scope.team_ids.#", "1"),
					resource.TestCheckTypeSetElemAttrPair("tsuga_custom_role.test", "team_scope.team_ids.*", "tsuga_team.test", "id"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.%", "3"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.team", "read"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.dashboard", "write"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.monitor", "read"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry.#", "2"),
					resource.TestCheckTypeSetElemAttr("tsuga_custom_role.test", "telemetry.*", "log"),
					resource.TestCheckTypeSetElemAttr("tsuga_custom_role.test", "telemetry.*", "trace"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry_query.#", "1"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry_query.0.key", "context.env"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry_query.0.values.#", "1"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry_query.0.values.0", "staging"),
				),
			},
			importStep,
			// Update: rename, drop the description and the telemetry restriction, widen the scope to every team
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "tsuga_custom_role" "test" {
  name = "%s-updated"
  team_scope = {
    type = "all"
  }
  assets = {
    team      = "write"
    dashboard = "write"
  }
  telemetry = ["log", "metric", "trace", "rum"]
}
`, roleName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_custom_role.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					roleID.AddStateValue("tsuga_custom_role.test", tfjsonpath.New("id")),
					// Dropped optional attributes must read back as null, not as empty values.
					statecheck.ExpectKnownValue("tsuga_custom_role.test", tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue("tsuga_custom_role.test", tfjsonpath.New("team_scope").AtMapKey("team_ids"), knownvalue.Null()),
					statecheck.ExpectKnownValue("tsuga_custom_role.test", tfjsonpath.New("telemetry_query"), knownvalue.Null()),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "name", roleName+"-updated"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "team_scope.type", "all"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.%", "2"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "assets.team", "write"),
					resource.TestCheckResourceAttr("tsuga_custom_role.test", "telemetry.#", "4"),
				),
			},
			importStep,
			// An explicitly empty telemetry restriction must stay an empty list, distinct from the null above.
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "tsuga_custom_role" "test" {
  name = "%s-updated"
  team_scope = {
    type = "all"
  }
  assets = {
    team      = "write"
    dashboard = "write"
  }
  telemetry       = ["log", "metric", "trace", "rum"]
  telemetry_query = []
}
`, roleName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_custom_role.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					roleID.AddStateValue("tsuga_custom_role.test", tfjsonpath.New("id")),
					statecheck.ExpectKnownValue("tsuga_custom_role.test", tfjsonpath.New("telemetry_query"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			importStep,
		},
	})
}
