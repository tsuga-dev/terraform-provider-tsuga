resource "tsuga_monitor" "monitor_rum" {
  cluster_ids = []
  configuration = {
    rum = {
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
        filter = "exceptions.context.app.name:web-app"
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
  to = tsuga_monitor.monitor_rum
  id = "resource-123"
}
