package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRetentionPolicyResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_retention_policy" "test" {
  env           = "prod"
  team_id       = tsuga_team.test-team.id
  data_source   = "logs"
  duration_days = "30-days"
  is_enabled    = true
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "data_source", "logs"),
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "duration_days", "30-days"),
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "is_enabled", "true"),
					resource.TestCheckResourceAttrSet("tsuga_retention_policy.test", "id"),
				),
			},
			// Update
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_retention_policy" "test" {
  env           = "prod"
  team_id       = tsuga_team.test-team.id
  data_source   = "logs"
  duration_days = "60-days"
  is_enabled    = false
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "data_source", "logs"),
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "duration_days", "60-days"),
					resource.TestCheckResourceAttr("tsuga_retention_policy.test", "is_enabled", "false"),
				),
			},
		},
	})
}
