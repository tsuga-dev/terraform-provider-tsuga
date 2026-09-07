resource "tsuga_route" "route" {
  is_enabled = true
  name       = "Example"
  owner      = "team-123"
  processors = []
  query      = "level:ERROR"
}

import {
  to = tsuga_route.route
  id = "resource-123"
}
