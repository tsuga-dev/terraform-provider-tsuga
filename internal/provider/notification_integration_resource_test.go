package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNotificationIntegrationResource(t *testing.T) {
	name := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_notification_integration" "pagerduty" {
  name = "%[1]s"
  setting = {
    pagerduty = {
      integration_key = "abcdef1234567890abcdef1234567890"
    }
  }
  tags = [{ key = "env", value = "dev" }]
}

resource "tsuga_notification_integration" "webhook" {
  name = "%[1]s-webhook"
  setting = {
    webhook = {
      url    = "https://example.com/hooks/tsuga"
      method = "POST"
      authentication = {
        bearer = { token = "shhh" }
      }
      custom_headers = { "X-Env" = "dev" }
      payload_template = {
        type  = "json"
        value = jsonencode({ status = "{{ alert_status }}" })
      }
      event_name_mapping = {
        ok    = "resolved"
        alert = "firing"
      }
    }
  }
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("tsuga_notification_integration.pagerduty", "id"),
					resource.TestCheckResourceAttr("tsuga_notification_integration.pagerduty", "name", name),
					resource.TestCheckResourceAttr("tsuga_notification_integration.pagerduty", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_notification_integration.webhook", "setting.webhook.method", "POST"),
					resource.TestCheckNoResourceAttr("tsuga_notification_integration.webhook", "setting.webhook.authentication.bearer.token"),
					resource.TestCheckResourceAttr("tsuga_notification_integration.webhook", "setting.webhook.custom_headers.X-Env", "dev"),
					resource.TestCheckResourceAttr("tsuga_notification_integration.webhook", "setting.webhook.event_name_mapping.ok", "resolved"),
				),
			},
			{
				ResourceName:      "tsuga_notification_integration.pagerduty",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Rename, rotate the key through secrets_version, and clear the event name mapping.
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_notification_integration" "pagerduty" {
  name            = "%[1]s-updated"
  secrets_version = "2"
  setting = {
    pagerduty = {
      integration_key = "0123456789abcdef0123456789abcdef"
    }
  }
  tags = [{ key = "env", value = "dev" }]
}

resource "tsuga_notification_integration" "webhook" {
  name = "%[1]s-webhook"
  setting = {
    webhook = {
      url    = "https://example.com/hooks/tsuga"
      method = "POST"
      authentication = {
        bearer = { token = "shhh" }
      }
      custom_headers = { "X-Env" = "dev" }
      payload_template = {
        type  = "json"
        value = jsonencode({ status = "{{ alert_status }}" })
      }
    }
  }
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_integration.pagerduty", "name", name+"-updated"),
					resource.TestCheckResourceAttr("tsuga_notification_integration.pagerduty", "secrets_version", "2"),
					resource.TestCheckNoResourceAttr("tsuga_notification_integration.pagerduty", "setting.pagerduty.integration_key"),
					resource.TestCheckNoResourceAttr("tsuga_notification_integration.webhook", "setting.webhook.event_name_mapping.ok"),
				),
			},
		},
	})
}
