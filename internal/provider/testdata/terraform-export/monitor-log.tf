resource "tsuga_monitor" "monitor_log" {
  cluster_ids = []
  configuration = {
    log = {
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
  message     = "Literal $${service} and %%{ if x }\n\"quoted\" \\ unicode 🌲"
  name        = "Example"
  owner       = "team-123"
  permissions = "all"
  priority    = 1
}

import {
  to = tsuga_monitor.monitor_log
  id = "resource-123"
}
