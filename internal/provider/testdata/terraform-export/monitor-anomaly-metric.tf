resource "tsuga_monitor" "monitor_anomaly_metric" {
  cluster_ids = []
  configuration = {
    anomaly_metric = {
      aggregation_alert_logic = "no_aggregation"
      condition = {
        formula = ""
      }
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
  message     = "Literal $${service} and %%{ if x }\n\"quoted\" \\ unicode 🌲"
  name        = "Example"
  owner       = "team-123"
  permissions = "all"
  priority    = 1
}

import {
  to = tsuga_monitor.monitor_anomaly_metric
  id = "resource-123"
}
