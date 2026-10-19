resource "tsuga_operation_api_key" "operation_api_key" {
  expires_at = "2027-01-01T00:00:00.000Z"
  name       = "terraform-production"
  owner      = "team-123"
  permissions = {
    "dashboard"        = "write"
    "dashboard-folder" = "read"
    "logs"             = "read"
  }
  scope = "owning-team-only"
  tags = [{
    key   = "env"
    value = "production"
  }]
}

import {
  to = tsuga_operation_api_key.operation_api_key
  id = "resource-123"
}
