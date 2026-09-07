resource "tsuga_notification_rule" "notification_rule" {
  is_active         = true
  name              = "Example"
  owner             = "team-123"
  priorities_filter = [1, 2]
  targets = [{
    config = {
      email = {
        addresses = ["oncall@example.com"]
      }
    }
    id = "target-1"
  }]
  teams_filter = {
    type = "all-teams"
  }
  transition_types_filter = ["triggered"]
}

import {
  to = tsuga_notification_rule.notification_rule
  id = "resource-123"
}
