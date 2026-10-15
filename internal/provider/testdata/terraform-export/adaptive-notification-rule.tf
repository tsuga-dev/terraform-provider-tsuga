resource "tsuga_adaptive_notification_rule" "adaptive_notification_rule" {
  fallback_target = {
    config = {
      email = {
        addresses = ["oncall@example.com"]
      }
    }
    id = "target-1"
  }
  is_active                   = true
  monitor_ids                 = ["monitor-123"]
  name                        = "Example"
  owner                       = "team-123"
  should_create_investigation = false
}

import {
  to = tsuga_adaptive_notification_rule.adaptive_notification_rule
  id = "resource-123"
}
