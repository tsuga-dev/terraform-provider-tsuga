# Example: Recurring silence for weekly maintenance windows
resource "tsuga_notification_silence" "maintenance-window" {
  name      = "weekly-maintenance"
  reason    = "Silence alerts during weekly maintenance window"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    recurring = {
      # Maintenance window every Sunday from 2 AM to 6 AM
      sunday = [
        {
          start_time = "02:00:00"
          end_time   = "06:00:00"
        }
      ]
    }
  }

  teams_filter = {
    type  = "specific-teams"
    teams = ["ghi-456-jkl"]
  }

  priorities_filter       = [1, 2, 3]
  transition_types_filter = ["triggered", "resolved"]

  tags = [
    {
      key   = "env"
      value = "prod"
    },
    {
      key   = "type"
      value = "maintenance"
    },
  ]
}

# Example: Recurring silence for off-hours
resource "tsuga_notification_silence" "off-hours" {
  name      = "off-hours-silence"
  reason    = "Reduce noise during off-business hours"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    recurring = {
      # Silence low-priority alerts overnight on weekdays
      monday = [
        {
          start_time = "00:00:00"
          end_time   = "07:00:00"
        },
        {
          start_time = "19:00:00"
          end_time   = "23:59:00"
        }
      ]
      tuesday = [
        {
          start_time = "00:00:00"
          end_time   = "07:00:00"
        },
        {
          start_time = "19:00:00"
          end_time   = "23:59:00"
        }
      ]
      wednesday = [
        {
          start_time = "00:00:00"
          end_time   = "07:00:00"
        },
        {
          start_time = "19:00:00"
          end_time   = "23:59:00"
        }
      ]
      thursday = [
        {
          start_time = "00:00:00"
          end_time   = "07:00:00"
        },
        {
          start_time = "19:00:00"
          end_time   = "23:59:00"
        }
      ]
      friday = [
        {
          start_time = "00:00:00"
          end_time   = "07:00:00"
        },
        {
          start_time = "19:00:00"
          end_time   = "23:59:00"
        }
      ]
      # Silence all day on weekends
      saturday = [
        {
          start_time = "00:00:00"
          end_time   = "23:59:00"
        }
      ]
      sunday = [
        {
          start_time = "00:00:00"
          end_time   = "23:59:00"
        }
      ]
    }
  }

  teams_filter = {
    type = "all-public-teams"
  }

  # Only silence low-priority alerts (P4, P5)
  priorities_filter       = [4, 5]
  transition_types_filter = ["triggered"]

  tags = [
    {
      key   = "type"
      value = "off-hours"
    },
  ]
}

# Example: One-time silence for a planned incident
resource "tsuga_notification_silence" "incident-silence" {
  name      = "incident-2024-03"
  reason    = "Silence alerts during planned database migration"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    one_time = {
      start_time = "2035-03-15T02:00:00"
      end_time   = "2035-03-15T06:00:00"
      time_zone  = "America/New_York"
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  priorities_filter       = [1, 2, 3, 4, 5]
  transition_types_filter = ["triggered", "resolved", "no-data"]
}

# Example: Recurring silence with timezone
resource "tsuga_notification_silence" "tokyo-maintenance" {
  name      = "tokyo-maintenance"
  reason    = "Silence during Tokyo office maintenance window"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    recurring = {
      wednesday = [
        {
          start_time = "02:00:00"
          end_time   = "04:00:00"
        }
      ]
      time_zone = "Asia/Tokyo"
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  priorities_filter       = [3, 4, 5]
  transition_types_filter = ["triggered"]
}

# Example: Silence specific notification rules with query filter
resource "tsuga_notification_silence" "deployment-silence" {
  name      = "deployment-silence"
  reason    = "Silence deployment-related alerts during typical deployment windows"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    recurring = {
      # Typical deployment windows on Tuesday and Thursday mornings
      tuesday = [
        {
          start_time = "10:00:00"
          end_time   = "12:00:00"
        }
      ]
      thursday = [
        {
          start_time = "10:00:00"
          end_time   = "12:00:00"
        }
      ]
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  # Filter alerts by query string
  query_string = "service:api AND env:staging"

  priorities_filter       = [1, 2, 3, 4, 5]
  transition_types_filter = ["triggered", "resolved", "no-data"]
}
