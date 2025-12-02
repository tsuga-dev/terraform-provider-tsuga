package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNotificationResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_rule" "test-notification-rule" {
  name = "test-notification-rule"
  owner = tsuga_team.test-team.id
  is_active = true
  priorities_filter = [1, 2, 3]
  teams_filter = []
  transition_types_filter = []
  targets = [
    {
      id = "email"
      config = {
        email = {
          addresses = ["test@example.com"]
        }
      }
    },
    {
      id = "slack"
      config = {
        slack = {
		    integration_id = "T06T0BAKV35"
          channel = "C09GTS5RNGM"
        }
      }
    },
    {
      id = "pagerduty"
      config = {
        pagerduty = {
		    integration_id = "yvsr-cmgt3-tyya"
        }
      }
    },
    {
      id = "grafana-irm"
      config = {
        grafana_irm = {
		    integration_id = "yjs8-d0rg8-j7tr"
        }
      }
    },
    {
      id = "incident-io"
      config = {
        incident_io = {
		    integration_id = "h4s8-fcb5e-cd14"
        }
      }
    }
  ]
  tags = [
	  {
	    key = "test-key"
	    value = "test-value"
	  }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "name", "test-notification-rule"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "is_active", "true"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "priorities_filter.#", "3"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "priorities_filter.0", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "priorities_filter.1", "2"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "priorities_filter.2", "3"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "teams_filter.#", "0"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "transition_types_filter.#", "0"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.0.id", "email"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.0.config.email.type", "email"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.0.config.email.addresses.#", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.0.config.email.addresses.0", "test@example.com"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.1.id", "slack"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.1.config.slack.type", "slack"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.1.config.slack.integration_id", "T06T0BAKV35"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.1.config.slack.channel", "C09GTS5RNGM"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "tags.0.key", "test-key"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "tags.0.value", "test-value"),
				),
			},
			// Update
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_rule" "test-notification-rule" {
  name = "test-notification-rule"
  owner = tsuga_team.test-team.id
  is_active = true
  priorities_filter = [1, 2, 3]
  teams_filter = []
  transition_types_filter = []
  targets = [
    {
      id = "email"
      config = {
        email = {
          addresses = ["test@example.com"]
        }
      }
    }
  ]
  tags = [
	{
	  key = "test-key-updated"
	  value = "test-value-updated"
	}
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.#", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "targets.0.id", "email"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "tags.0.key", "test-key-updated"),
					resource.TestCheckResourceAttr("tsuga_notification_rule.test-notification-rule", "tags.0.value", "test-value-updated"),
				),
			},
		},
	})
}
