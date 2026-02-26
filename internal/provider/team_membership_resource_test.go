package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamMembershipResource(t *testing.T) {
	userId := os.Getenv("TSUGA_USER_ID")
	if userId == "" {
		t.Skip("TSUGA_USER_ID must be set for acceptance tests")
	}

	teamName := fmt.Sprintf("test-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
data "tsuga_user" "test" {
  id = "%s"
}

resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_team_membership" "test" {
  user_id  = data.tsuga_user.test.id
  team_id  = tsuga_team.test.id
  role_key = "viewer"
}
`, userId, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("tsuga_team_membership.test", "id"),
					resource.TestCheckResourceAttr("tsuga_team_membership.test", "role_key", "viewer"),
				),
			},
			// Update role
			{
				Config: providerConfig + fmt.Sprintf(`
data "tsuga_user" "test" {
  id = "%s"
}

resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_team_membership" "test" {
  user_id  = data.tsuga_user.test.id
  team_id  = tsuga_team.test.id
  role_key = "editor"
}
`, userId, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("tsuga_team_membership.test", "id"),
					resource.TestCheckResourceAttr("tsuga_team_membership.test", "role_key", "editor"),
				),
			},
		},
	})
}
