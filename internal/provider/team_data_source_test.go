package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccTeamDataSourceLookupByName covers the real-world pattern raised in user feedback:
// teams are provisioned outside Terraform, so operators look them up by name and pass
// the resolved id to dependent resources (e.g. tsuga_ingestion_api_key).
func TestAccTeamDataSourceLookupByName(t *testing.T) {
	teamNameA := fmt.Sprintf("test-%s", randomString(10))
	teamNameB := fmt.Sprintf("test-%s", randomString(10))
	keyNameA := fmt.Sprintf("key-%s", randomString(10))
	keyNameB := fmt.Sprintf("key-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
# Simulate teams that exist prior to this Terraform run (managed by another mechanism).
resource "tsuga_team" "owner_a" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_team" "owner_b" {
  name       = "%s"
  visibility = "public"
}

# Look up those teams by name
data "tsuga_team" "owner_a" {
  name       = "%s"
  depends_on = [tsuga_team.owner_a]
}

data "tsuga_team" "owner_b" {
  name       = "%s"
  depends_on = [tsuga_team.owner_b]
}

resource "tsuga_ingestion_api_key" "key_a" {
  name  = "%s"
  owner = data.tsuga_team.owner_a.id
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}

resource "tsuga_ingestion_api_key" "key_b" {
  name  = "%s"
  owner = data.tsuga_team.owner_b.id
  tags = [
    {
      key   = "env"
      value = "dev"
    }
  ]
}
`, teamNameA, teamNameB, teamNameA, teamNameB, keyNameA, keyNameB),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.tsuga_team.owner_a", "name", teamNameA),
					resource.TestCheckResourceAttrSet("data.tsuga_team.owner_a", "id"),
					resource.TestCheckResourceAttr("data.tsuga_team.owner_b", "name", teamNameB),
					resource.TestCheckResourceAttrSet("data.tsuga_team.owner_b", "id"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.key_a", "name", keyNameA),
					resource.TestCheckResourceAttrPair("tsuga_ingestion_api_key.key_a", "owner", "data.tsuga_team.owner_a", "id"),
					resource.TestCheckResourceAttr("tsuga_ingestion_api_key.key_b", "name", keyNameB),
					resource.TestCheckResourceAttrPair("tsuga_ingestion_api_key.key_b", "owner", "data.tsuga_team.owner_b", "id"),
				),
			},
		},
	})
}

func TestAccTeamDataSource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read by id
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
			// Read by name
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

data "tsuga_team" "test" {
  name = "%s"
}
`, teamName, teamName),
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
