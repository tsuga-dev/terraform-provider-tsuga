package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamDataSource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

data "tsuga_team" "test" {
  id = tsuga_team.test.id
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.tsuga_team.test", "name", teamName),
					resource.TestCheckResourceAttr("data.tsuga_team.test", "visibility", "public"),
					resource.TestCheckResourceAttrSet("data.tsuga_team.test", "id"),
				),
			},
			// Ongoing validation: data source reflects updates to the underlying resource.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "private"
}

data "tsuga_team" "test" {
  id = tsuga_team.test.id
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.tsuga_team.test", "name", teamName),
					resource.TestCheckResourceAttr("data.tsuga_team.test", "visibility", "private"),
					resource.TestCheckResourceAttrSet("data.tsuga_team.test", "id"),
				),
			},
		},
	})
}
