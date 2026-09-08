resource "tsuga_monitor" "monitor_log_error_pattern" {
  cluster_ids = []
  configuration = {
    log_error_pattern = {
      aggregation_alert_logic = "each"
      filter = {
        env      = "prod"
        services = ["api"]
        team_ids = ["team-123"]
      }
      no_data_behavior = "keep_last_status"
    }
  }
  message     = "Literal $${service} and %%{ if x }\n\"quoted\" \\ unicode 🌲"
  name        = "Example"
  owner       = "team-123"
  permissions = "all"
  priority    = 1
}

import {
  to = tsuga_monitor.monitor_log_error_pattern
  id = "resource-123"
}
