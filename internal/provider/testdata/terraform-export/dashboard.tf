resource "tsuga_dashboard" "dashboard" {
  filters = [{
    key    = "env"
    values = []
  }]
  folder_id = "folder-123"
  graphs = [{
    id = "note-1"
    layout = {
      h = 3
      w = 6
      x = 0
      y = 0
    }
    name = "Description"
    visualization = {
      note = {
        note = "$${literal} %%{ literal }\nhello 🌲"
      }
    }
    }, {
    id = "chart-1"
    visualization = {
      timeseries = {
        aliases = {
          formula = "Errors"
          queries = {
            "q1" = "Error count"
          }
        }
        queries = [{
          aggregate = {
            count = {}
          }
          filter = "level:ERROR"
        }]
        source         = "logs"
        visible_series = [true]
      }
    }
  }]
  name        = "Example"
  owner       = "team-123"
  time_preset = "past-1-hour"
}

import {
  to = tsuga_dashboard.dashboard
  id = "resource-123"
}
