package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDashboardFolderResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(8))
	folderName := fmt.Sprintf("test-folder-%s", randomString(8))

	teamConfig := fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}
`, teamName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a top-level folder
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "tsuga_dashboard_folder" "test" {
  name  = "%s"
  owner = tsuga_team.test-team.id
  tags = [
    {
      key   = "team"
      value = "platform"
    }
  ]
}
`, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_dashboard_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("tsuga_dashboard_folder.test", "tags.0.key", "team"),
					resource.TestCheckResourceAttr("tsuga_dashboard_folder.test", "tags.0.value", "platform"),
					resource.TestCheckNoResourceAttr("tsuga_dashboard_folder.test", "parent_folder_id"),
					resource.TestCheckResourceAttrSet("tsuga_dashboard_folder.test", "id"),
				),
			},
			// Rename, and nest a child folder under it
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "tsuga_dashboard_folder" "test" {
  name  = "%s-renamed"
  owner = tsuga_team.test-team.id
}

resource "tsuga_dashboard_folder" "child" {
  name             = "%s-child"
  owner            = tsuga_team.test-team.id
  parent_folder_id = tsuga_dashboard_folder.test.id
}
`, folderName, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_dashboard_folder.test", "name", folderName+"-renamed"),
					resource.TestCheckResourceAttrPair(
						"tsuga_dashboard_folder.child", "parent_folder_id",
						"tsuga_dashboard_folder.test", "id",
					),
				),
			},
			// Import
			{
				ResourceName:      "tsuga_dashboard_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
