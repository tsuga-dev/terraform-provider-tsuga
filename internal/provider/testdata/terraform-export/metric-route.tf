resource "tsuga_metric_route" "metric_route" {
  is_enabled        = true
  metric_name_regex = "^http\\.server\\..*$"
  name              = "Example"
  owner             = "team-123"
  processors        = []
}

import {
  to = tsuga_metric_route.metric_route
  id = "resource-123"
}
