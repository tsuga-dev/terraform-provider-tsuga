package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCustomUsageTagResource(t *testing.T) {
	tagKey := fmt.Sprintf("test-tag-%s", randomString(8))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_custom_usage_tag" "test" {
  tag_key = %q
}
`, tagKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_custom_usage_tag.test", "tag_key", tagKey),
					resource.TestCheckResourceAttrSet("tsuga_custom_usage_tag.test", "id"),
				),
			},
			// ImportState
			{
				ResourceName:      "tsuga_custom_usage_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
