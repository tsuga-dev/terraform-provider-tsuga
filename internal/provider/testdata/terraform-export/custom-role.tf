resource "tsuga_custom_role" "custom_role" {
  assets = {
    "dashboard"      = "write"
    "monitor-folder" = "read"
    "team"           = "read"
  }
  description = "Edits monitors and dashboards of the platform team."
  name        = "platform-on-call"
  team_scope = {
    team_ids = ["team-123"]
    type     = "specific"
  }
  telemetry = ["log", "metric", "trace"]
  telemetry_query = [{
    key    = "context.env"
    values = ["production"]
  }]
}

import {
  to = tsuga_custom_role.custom_role
  id = "resource-123"
}
