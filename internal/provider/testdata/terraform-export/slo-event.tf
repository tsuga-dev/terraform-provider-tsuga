resource "tsuga_slo" "slo_event" {
  alerts = [{
    configuration = {
      burn_rate = 14.4
    }
    priority = 1
  }]
  cluster_ids = []
  configuration = {
    event = {
      data_source = "logs"
      good_query = {
        formula = "q1"
        queries = [{
          aggregate = {
            count = {}
          }
          filter = "level:ERROR"
        }]
      }
      no_data_behavior = "good"
      total_query = {
        formula = "q1"
        queries = [{
          aggregate = {
            count = {}
          }
          filter = "level:ERROR"
        }]
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
  to = tsuga_slo.slo_event
  id = "resource-123"
}
