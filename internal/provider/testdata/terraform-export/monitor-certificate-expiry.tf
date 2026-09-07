resource "tsuga_monitor" "monitor_certificate_expiry" {
  cluster_ids = []
  configuration = {
    certificate_expiry = {
      aggregation_alert_logic = "each"
      cloud_accounts          = ["cloud-123"]
      no_data_behavior        = "resolve"
      warn_before_in_days     = 30
    }
  }
  message     = "Literal $${service} and %%{ if x }\n\"quoted\" \\ unicode 🌲"
  name        = "Example"
  owner       = "team-123"
  permissions = "all"
  priority    = 1
}

import {
  to = tsuga_monitor.monitor_certificate_expiry
  id = "resource-123"
}
