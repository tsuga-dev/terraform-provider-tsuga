resource "tsuga_retention_policy" "retention" {
  data_source   = "logs"
  duration_days = 30
  env           = "production"
  is_enabled    = true
}

import {
  to = tsuga_retention_policy.retention
  id = "retention-1"
}
