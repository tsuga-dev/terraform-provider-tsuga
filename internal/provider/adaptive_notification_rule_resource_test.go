package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccAdaptiveNotificationRuleConfig(teamName, ruleName, monitorIDs string) string {
	return providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "%s-monitor"
  owner       = tsuga_team.test.id
  permissions = "all"
  priority    = 1
  message     = "Adaptive rule acceptance test"
  cluster_ids = []
  configuration = {
    trace = {
      aggregation_alert_logic = "no_aggregation"
      conditions = [{
        formula   = "q1"
        operator  = "greater_than"
        threshold = 0
      }]
      group_by_fields  = []
      no_data_behavior = "resolve"
      queries = [{
        aggregate = {
          count = {}
        }
        filter = "level:ERROR"
      }]
      timeframe = 5
    }
  }
}

resource "tsuga_adaptive_notification_rule" "test" {
  name        = "%s"
  owner       = tsuga_team.test.id
  is_active   = true
  monitor_ids = %s
  fallback_target = {
    id = "fallback"
    config = {
      email = {
        addresses = ["oncall@example.com"]
      }
    }
  }
}
`, teamName, ruleName, ruleName, monitorIDs)
}

func TestAccAdaptiveNotificationRuleResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(8))
	ruleName := fmt.Sprintf("test-adaptive-rule-%s", randomString(8))
	resourceName := "tsuga_adaptive_notification_rule.test"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAdaptiveNotificationRuleConfig(teamName, ruleName, "[tsuga_monitor.test.id]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", ruleName),
					resource.TestCheckResourceAttr(resourceName, "is_active", "true"),
					resource.TestCheckResourceAttr(resourceName, "should_create_investigation", "false"),
					resource.TestCheckResourceAttr(resourceName, "monitor_ids.#", "1"),
					resource.TestCheckResourceAttrPair(resourceName, "monitor_ids.0", "tsuga_monitor.test", "id"),
					resource.TestCheckNoResourceAttr(resourceName, "slo_ids"),
					resource.TestCheckResourceAttr(resourceName, "fallback_target.id", "fallback"),
					resource.TestCheckResourceAttr(resourceName, "fallback_target.config.email.addresses.0", "oncall@example.com"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAdaptiveNotificationRuleConfig(teamName, ruleName+"-renamed", "[tsuga_monitor.test.id]"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", ruleName+"-renamed"),
					resource.TestCheckResourceAttr(resourceName, "monitor_ids.#", "1"),
				),
			},
		},
	})
}
