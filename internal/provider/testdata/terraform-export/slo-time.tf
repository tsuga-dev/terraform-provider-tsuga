resource "tsuga_slo" "slo_time" {
  alerts = [{
    configuration = {
      burn_rate = 14.4
    }
    priority = 1
  }]
  cluster_ids = []
  configuration = {
    time = {
      data_source      = "logs"
      no_data_behavior = "good"
      query = {
        formula = "q1"
        queries = [{
          aggregate = {
            count = {}
          }
          filter = "level:ERROR"
        }]
      }
      slice_size_minutes = 5
      threshold = {
        operator = "less_than"
        value    = 10
      }
    }
  }
  description    = ""
  name           = "Example"
  owner          = "team-123"
  permissions    = "all"
  target         = 99.9
  timeframe_days = 30
}

import {
  to = tsuga_slo.slo_time
  id = "resource-123"
}
