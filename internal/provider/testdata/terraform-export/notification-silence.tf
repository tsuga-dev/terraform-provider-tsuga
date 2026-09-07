resource "tsuga_notification_silence" "notification_silence" {
  is_active         = true
  name              = "Example"
  owner             = "team-123"
  priorities_filter = []
  schedule = {
    one_time = {
      end_time   = "2026-09-06T13:00:00"
      start_time = "2026-09-06T12:00:00"
    }
  }
  teams_filter = {
    type = "all-teams"
  }
  transition_types_filter = []
}

import {
  to = tsuga_notification_silence.notification_silence
  id = "resource-123"
}
