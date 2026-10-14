package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDashboardFolderResource(t *testing.T) {
	testAccFolderResource(t, dashboardFolderKind)
}

func TestAccMonitorFolderResource(t *testing.T) {
	testAccFolderResource(t, monitorFolderKind)
}

func testAccFolderResource(t *testing.T, kind folderKind) {
	resourceType := "tsuga_" + kind.typeName
	teamName := fmt.Sprintf("test-%s", randomString(8))
	folderName := fmt.Sprintf("test-folder-%s", randomString(8))

	teamConfig := fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}
`, teamName)

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a top-level folder
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "%[1]s" "test" {
  name  = "%[2]s"
  owner = tsuga_team.test-team.id
  tags = [
    {
      key   = "team"
      value = "platform"
    }
  ]
}
`, resourceType, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceType+".test", "name", folderName),
					resource.TestCheckResourceAttr(resourceType+".test", "tags.0.key", "team"),
					resource.TestCheckResourceAttr(resourceType+".test", "tags.0.value", "platform"),
					resource.TestCheckNoResourceAttr(resourceType+".test", "parent_folder_id"),
					resource.TestCheckResourceAttrSet(resourceType+".test", "id"),
				),
			},
			// Rename, and nest a child folder under it
			{
				Config: providerConfig + teamConfig + fmt.Sprintf(`
resource "%[1]s" "test" {
  name  = "%[2]s-renamed"
  owner = tsuga_team.test-team.id
}

resource "%[1]s" "child" {
  name             = "%[2]s-child"
  owner            = tsuga_team.test-team.id
  parent_folder_id = %[1]s.test.id
}
`, resourceType, folderName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceType+".test", "name", folderName+"-renamed"),
					resource.TestCheckResourceAttrPair(
						resourceType+".child", "parent_folder_id",
						resourceType+".test", "id",
					),
				),
			},
			// Import
			{
				ResourceName:      resourceType + ".test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
