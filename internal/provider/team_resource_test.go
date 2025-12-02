package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name = "%s"
  visibility = "public"
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_team.test", "name", teamName),
					resource.TestCheckResourceAttr("tsuga_team.test", "visibility", "public"),
				),
			},
			// Update
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name = "%s"
  visibility = "private"
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_team.test", "name", teamName),
					resource.TestCheckResourceAttr("tsuga_team.test", "visibility", "private"),
				),
			},
		},
	})
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
