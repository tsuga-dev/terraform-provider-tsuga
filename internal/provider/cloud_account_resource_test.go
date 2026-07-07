package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCloudAccountResource(t *testing.T) {
	accountId := fmt.Sprintf("1234567%05d", 42)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_cloud_account" "test" {
  account_friendly_name = "Test AWS"

  aws = {
    account_id  = "%s"
    external_id = "test-external-id"
    role_arn    = "arn:aws:iam::%s:role/tsuga-inventory"
  }
}
`, accountId, accountId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_cloud_account.test", "cloud_type", "aws"),
					resource.TestCheckResourceAttr("tsuga_cloud_account.test", "cloud_account_id", accountId),
					resource.TestCheckResourceAttr("tsuga_cloud_account.test", "account_friendly_name", "Test AWS"),
					resource.TestCheckResourceAttr("tsuga_cloud_account.test", "aws.role_arn", fmt.Sprintf("arn:aws:iam::%s:role/tsuga-inventory", accountId)),
					resource.TestCheckResourceAttrSet("tsuga_cloud_account.test", "id"),
				),
			},
			// Update (only the friendly name is mutable)
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_cloud_account" "test" {
  account_friendly_name = "Renamed AWS"

  aws = {
    account_id  = "%s"
    external_id = "test-external-id"
    role_arn    = "arn:aws:iam::%s:role/tsuga-inventory"
  }
}
`, accountId, accountId),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_cloud_account.test", "account_friendly_name", "Renamed AWS"),
				),
			},
		},
	})
}
