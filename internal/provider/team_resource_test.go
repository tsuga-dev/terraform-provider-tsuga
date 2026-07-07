package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/compare"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccTeamResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	// Asserts the team id is identical across every step it's registered in, so
	// an in-place update (e.g. editing the description) never recreates the team.
	teamID := statecheck.CompareValue(compare.ValuesSame())

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
				ConfigStateChecks: []statecheck.StateCheck{
					teamID.AddStateValue("tsuga_team.test", tfjsonpath.New("id")),
				},
			},
			// Update
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name = "%s"
  visibility = "private"
  description = "first"
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_team.test", "name", teamName),
					resource.TestCheckResourceAttr("tsuga_team.test", "visibility", "private"),
				),
			},
			// Editing only the description updates in place and keeps `id` known,
			// so dependent resources don't see spurious diffs. Regression test for
			// the missing UseStateForUnknown plan modifier on `id`.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name = "%s"
  visibility = "private"
  description = "second"
}
`, teamName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("tsuga_team.test", plancheck.ResourceActionUpdate),
						// id must stay known in the plan, not "(known after apply)".
						plancheck.ExpectKnownValue("tsuga_team.test", tfjsonpath.New("id"), knownvalue.NotNull()),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					// Same id as the create step -> the team was not recreated.
					teamID.AddStateValue("tsuga_team.test", tfjsonpath.New("id")),
				},
				Check: resource.TestCheckResourceAttr("tsuga_team.test", "description", "second"),
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
