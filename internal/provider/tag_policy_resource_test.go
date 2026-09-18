package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagPolicyResource_telemetry(t *testing.T) {
	telemetryTagKey := fmt.Sprintf("environment-%s", randomString(8))
	assetTagKey := fmt.Sprintf("cost-center-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with telemetry configuration
			{
				Config: testAccTagPolicyResource_telemetry("test-tag-policy", telemetryTagKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "test-tag-policy"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "is_active", "true"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "tag_key", telemetryTagKey),
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
				Config: testAccTagPolicyResource_tsugaAsset("test-tag-policy-updated", assetTagKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "test-tag-policy-updated"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "configuration.tsuga_asset.asset_types.#", "3"),
				),
			},
		},
	})
}

func TestAccTagPolicyResource_withTeamScope(t *testing.T) {
	tagKey := fmt.Sprintf("hi-this-is-a-test-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagPolicyResource_withTeamScope(tagKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "scoped-tag-policy"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "team_scope.mode", "include"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "team_scope.team_ids.#", "1"),
				),
			},
		},
	})
}

func TestAccTagPolicyResource_emptyAllowedTagValues(t *testing.T) {
	tagKey := fmt.Sprintf("some-random-test-key-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagPolicyResource_emptyAllowedTagValues(tagKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "name", "allow-all-values-policy"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "allowed_tag_values.#", "0"),
					resource.TestCheckResourceAttr("tsuga_tag_policy.test", "is_active", "true"),
					resource.TestCheckResourceAttrSet("tsuga_tag_policy.test", "id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "tsuga_tag_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTagPolicyResource_telemetry(name, tagKey string) string {
	return fmt.Sprintf(`
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = %[1]q
  description = "Test tag policy for telemetry"
  is_active   = true
  tag_key     = %[2]q
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
`, name, tagKey)
}

func testAccTagPolicyResource_tsugaAsset(name, tagKey string) string {
	return fmt.Sprintf(`
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = %[1]q
  description = "Test tag policy for Tsuga assets"
  is_active   = true
  tag_key     = %[2]q
  allowed_tag_values = ["engineering", "sales", "marketing"]
  is_required = false
  owner       = tsuga_team.owner.id

  configuration = {
    tsuga_asset = {
      asset_types = ["dashboard", "monitor", "slo"]
    }
  }
}
`, name, tagKey)
}

func testAccTagPolicyResource_withTeamScope(tagKey string) string {
	return fmt.Sprintf(`
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
  tag_key     = %q
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
`, tagKey)
}

func testAccTagPolicyResource_emptyAllowedTagValues(tagKey string) string {
	return fmt.Sprintf(`
resource "tsuga_team" "owner" {
  name       = "tag-policy-test-owner"
  visibility = "public"
}

resource "tsuga_tag_policy" "test" {
  name        = "allow-all-values-policy"
  description = "Tag policy that allows all values"
  is_active   = true
  tag_key     = %q
  allowed_tag_values = []
  is_required = true
  owner       = tsuga_team.owner.id

  configuration = {
    telemetry = {
      asset_types           = ["logs"]
      should_insert_warning = true
    }
  }
}
`, tagKey)
}
