package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIngestionApiKeyResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	keyName := fmt.Sprintf("test-%s", randomString(10))

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

resource "tsuga_ingestion_api_key" "test" {
  name  = "%s"
  owner = tsuga_team.test.id
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "name", keyName),
					resource.TestCheckResourceAttrSet("tsuga_ingestion_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("tsuga_ingestion_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("tsuga_ingestion_api_key.test", "key_last_characters"),
					resource.TestCheckResourceAttrPair("tsuga_ingestion_api_key.test", "owner", "tsuga_team.test", "id"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "tags.0.value", "dev"),
				),
			},
			// ImportState — key is not returned by GET so it will be empty after import
			{
				ResourceName:            "tsuga_ingestion_api_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
			// Update name and add tags
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_ingestion_api_key" "test" {
  name  = "%s-updated"
  owner = tsuga_team.test.id

  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamName, keyName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "name", keyName+"-updated"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.test", "tags.0.value", "dev"),
					// key must be preserved across updates
					resource.TestCheckResourceAttrSet("tsuga_ingestion_api_key.test", "key"),
				),
			},
		},
	})
}
