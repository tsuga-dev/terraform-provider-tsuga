package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMonitorResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-monitor"
  owner       = tsuga_team.test-team.id
  priority    = 3
  permissions = "all"
  message     = "Test monitor message"

  configuration = {
    metric = {
      condition = {
        formula   = "q1"
        operator  = "greater_than"
        threshold = 10.0
      }
      no_data_behavior        = "alert"
      timeframe               = 5
      group_by_fields = [{
        fields = ["service"]
        limit  = 10
      }]
      aggregation_alert_logic = "no_aggregation"
      queries = [{
        filter = "service:api"
        aggregate = {
          sum = {
            field = "duration"
          }
        }
      }]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-monitor"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "3"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "all"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "message", "Test monitor message"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.metric.condition.formula", "q1"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.metric.condition.operator", "greater_than"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.metric.condition.threshold", "10"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.metric.timeframe", "5"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.metric.queries.#", "1"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-monitor-updated"
  owner       = tsuga_team.test-team.id
  priority    = 4
  permissions = "owning-team-only"
  message     = "Updated monitor message"

  configuration = {
    log = {
      condition = {
        formula   = "q1 + q2"
        operator  = "less_than"
        threshold = 5.0
      }
      no_data_behavior        = "resolve"
      timeframe               = 10
      group_by_fields = [{
        fields = ["service", "env"]
        limit  = 10
      }]
      aggregation_alert_logic = "no_aggregation"
      queries = [
        {
          filter = "service:web"
          aggregate = {
            count = {}
          }
        },
        {
          filter = "env:prod"
          aggregate = {
            unique_count = {
              field = "user"
            }
          }
        }
      ]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-monitor-updated"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "4"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "owning-team-only"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "message", "Updated monitor message"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.log.condition.formula", "q1 + q2"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.log.condition.operator", "less_than"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.log.condition.threshold", "5"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.log.timeframe", "10"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.log.queries.#", "2"),
				),
			},
		},
	})
}

func TestAccMonitorResource_WithTags(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-monitor-with-tags"
  owner       = tsuga_team.test-team.id
  priority    = 3
  permissions = "all"

  tags = [
    {
      key   = "environment"
      value = "test"
    },
    {
      key   = "team"
      value = "platform"
    }
  ]

  configuration = {
    metric = {
      condition = {
        formula   = "q1"
        operator  = "greater_than"
        threshold = 10.0
      }
      no_data_behavior        = "alert"
      timeframe               = 5
      group_by_fields         = []
      aggregation_alert_logic = "no_aggregation"
      queries = [{
        name   = "q1"
        filter = "service:api"
        aggregate = {
          sum = {
            field = "duration"
          }
        }
      }]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "tags.#", "2"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "tags.0.key", "environment"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "tags.0.value", "test"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "tags.1.key", "team"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "tags.1.value", "platform"),
				),
			},
		},
	})
}

func TestAccMonitorResource_WithDashboardId(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_dashboard" "test-dashboard" {
  name  = "test-dashboard"
  owner = tsuga_team.test-team.id
  graphs = [{
    id   = "graph-1"
    name = "Graph One"
    visualization = {
      note = {
        note = "hello world"
      }
    }
  }]
}

resource "tsuga_monitor" "test" {
  name         = "test-monitor-with-dashboard"
  owner        = tsuga_team.test-team.id
  priority     = 3
  permissions  = "all"
  dashboard_id = tsuga_dashboard.test-dashboard.id

  configuration = {
    metric = {
      condition = {
        formula   = "q1"
        operator  = "greater_than"
        threshold = 10.0
      }
      no_data_behavior        = "alert"
      timeframe               = 5
      group_by_fields         = []
      aggregation_alert_logic = "no_aggregation"
      queries = [{
        name   = "q1"
        filter = "service:api"
        aggregate = {
          sum = {
            field = "duration"
          }
        }
      }]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("tsuga_monitor.test", "dashboard_id"),
				),
			},
		},
	})
}

func TestAccMonitorResource_AnomalyMetric(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-anomaly-metric-monitor"
  owner       = tsuga_team.test-team.id
  priority    = 3
  permissions = "all"
  message     = "Anomaly metric monitor message"

  configuration = {
    anomaly_metric = {
      condition = {
        formula = "q1"
      }
      no_data_behavior        = "alert"
      timeframe               = 5
      group_by_fields = [{
        fields = ["service"]
        limit  = 10
      }]
      aggregation_alert_logic = "no_aggregation"
      queries = [{
        filter = "service:api"
        aggregate = {
          sum = {
            field = "duration"
          }
        }
      }]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-anomaly-metric-monitor"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "3"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "all"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "message", "Anomaly metric monitor message"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.condition.formula", "q1"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.timeframe", "5"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.queries.#", "1"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-anomaly-metric-monitor-updated"
  owner       = tsuga_team.test-team.id
  priority    = 4
  permissions = "owning-team-only"
  message     = "Updated anomaly metric monitor message"

  configuration = {
    anomaly_metric = {
      condition = {
        formula = "q1 + q2"
      }
      no_data_behavior        = "resolve"
      timeframe               = 10
      group_by_fields = [{
        fields = ["service", "env"]
        limit  = 5
      }]
      aggregation_alert_logic = "no_aggregation"
      queries = [
        {
          filter = "service:web"
          aggregate = {
            count = {}
          }
        },
        {
          filter = "env:prod"
          aggregate = {
            average = {
              field = "latency"
            }
          }
        }
      ]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-anomaly-metric-monitor-updated"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "4"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "owning-team-only"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.condition.formula", "q1 + q2"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.timeframe", "10"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_metric.queries.#", "2"),
				),
			},
		},
	})
}

func TestAccMonitorResource_AnomalyLog(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-anomaly-log-monitor"
  owner       = tsuga_team.test-team.id
  priority    = 2
  permissions = "all"
  message     = "Anomaly log monitor message"

  configuration = {
    anomaly_log = {
      condition = {
        formula = "q1"
      }
      no_data_behavior        = "keep_last_status"
      timeframe               = 15
      group_by_fields = [{
        fields = ["host"]
        limit  = 20
      }]
      aggregation_alert_logic = "no_aggregation"
      queries = [{
        filter = "level:error"
        aggregate = {
          count = {}
        }
      }]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-anomaly-log-monitor"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "2"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "all"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "message", "Anomaly log monitor message"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.condition.formula", "q1"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.timeframe", "15"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.no_data_behavior", "keep_last_status"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.queries.#", "1"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_monitor" "test" {
  name        = "test-anomaly-log-monitor-updated"
  owner       = tsuga_team.test-team.id
  priority    = 1
  permissions = "owning-team-and-public"
  message     = "Updated anomaly log monitor message"

  configuration = {
    anomaly_log = {
      condition = {
        formula = "q1 - q2"
      }
      no_data_behavior        = "alert"
      timeframe               = 30
      group_by_fields = []
      aggregation_alert_logic = "no_aggregation"
      queries = [
        {
          filter = "level:error"
          aggregate = {
            count = {}
          }
        },
        {
          filter = "level:warn"
          aggregate = {
            count = {}
          }
        }
      ]
    }
  }
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_monitor.test", "name", "test-anomaly-log-monitor-updated"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "priority", "1"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "permissions", "owning-team-and-public"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.condition.formula", "q1 - q2"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.timeframe", "30"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.aggregation_alert_logic", "no_aggregation"),
					resource.TestCheckResourceAttr("tsuga_monitor.test", "configuration.anomaly_log.queries.#", "2"),
				),
			},
		},
	})
}
