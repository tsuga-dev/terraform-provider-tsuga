package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagPolicyResource_telemetry(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with telemetry configuration
			{
				Config: testAccTagPolicyResource_telemetry("test-tag-policy"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "test-tag-policy"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "is_active", "true"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "tag_key", "environment"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "is_required", "true"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "allowed_tag_values.#", "3"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "configuration.telemetry.asset_types.#", "2"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "configuration.telemetry.should_insert_warning", "true"),
					resource.TestCheckResourceAttrSet("tsuga_tag_policy.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "tsuga_tag_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update to tsuga_asset configuration
			{
				Config: testAccTagPolicyResource_tsugaAsset("test-tag-policy-updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "test-tag-policy-updated"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "configuration.tsuga_asset.asset_types.#", "2"),
				),
			},
		},
	})
}

func TestAccTagPolicyResource_withTeamScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagPolicyResource_withTeamScope(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "scoped-tag-policy"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "team_scope.mode", "include"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "team_scope.team_ids.#", "1"),
				),
			},
		},
	})
}

func testAccTagPolicyResource_telemetry(name string) string {
	return fmt.Sprintf(`
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = %[1]q
  description = "Test tag policy for telemetry"
  is_active   = true
  tag_key     = "environment"
  allowed_tag_values = ["production", "staging", "development"]
  is_required = true
  owner       = tsuga_team.owner.id

  configuration = {
    telemetry = {
      asset_types           = ["logs", "metrics"]
      should_insert_warning = true
      drop_sample           = 10.5
    }
  }
}
`, name)
}

func testAccTagPolicyResource_tsugaAsset(name string) string {
	return fmt.Sprintf(`
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = %[1]q
  description = "Test tag policy for Tsuga assets"
  is_active   = true
  tag_key     = "cost-center"
  allowed_tag_values = ["engineering", "sales", "marketing"]
  is_required = false
  owner       = tsuga_team.owner.id

  configuration = {
    tsuga_asset = {
      asset_types = ["dashboard", "monitor"]
    }
  }
}
`, name)
}

func testAccTagPolicyResource_withTeamScope() string {
	return `
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_team" "scoped" {
  name       = "scoped-team"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = "scoped-tag-policy"
  description = "Tag policy with team scope"
  is_active   = true
  tag_key     = "team"
  allowed_tag_values = ["platform", "infrastructure"]
  is_required = true
  owner       = tsuga_team.owner.id

  team_scope = {
    team_ids = [tsuga_team.scoped.id]
    mode     = "include"
  }

  configuration = {
    tsuga_asset = {
      asset_types = ["notification-rule", "notification-silence"]
    }
  }
}
`
}
