---
page_title: "tsuga_notification_silence Resource - tsuga"
subcategory: ""
description: |-
  Silences to suppress notifications based on schedules and filters
---

# tsuga_notification_silence (Resource)

Silences to suppress notifications based on schedules and filters

Exactly one of `recurring` or `one_time` must be set in `schedule`. `recurring` takes its days of the week in `weekly_schedule`, such as `weekly_schedule.monday`.

Times are fixed-width local times without a UTC offset, `HH:MM:SS` (e.g. `09:00:00`) in `recurring` windows, `YYYY-MM-DDTHH:MM:SS` (e.g. `2035-03-15T02:00:00`) in `one_time`.

`teams_filter.type` is one of `specific-teams`, `all-teams` or `all-public-teams`; `teams` is required with `specific-teams`.

## Example Usage

```terraform
# Example: Recurring silence for weekly maintenance windows
resource "tsuga_notification_silence" "maintenance-window" {
  name      = "weekly-maintenance"
  reason    = "Silence alerts during weekly maintenance window"
  owner     = "abc-123-def"
  is_active = true

  schedule = {
    recurring = {
      weekly_schedule = {
        # Maintenance window every Sunday from 2 AM to 6 AM
        sunday = [
          {
            start_time = "02:00:00"
            end_time   = "06:00:00"
          }
        ]
      }
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
      weekly_schedule = {
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
      weekly_schedule = {
        wednesday = [
          {
            start_time = "02:00:00"
            end_time   = "04:00:00"
          }
        ]
      }
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
      weekly_schedule = {
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
  }

  teams_filter = {
    type = "all-teams"
  }

  # Filter alerts by query string
  query_string = "service:api AND env:staging"

  priorities_filter       = [1, 2, 3, 4, 5]
  transition_types_filter = ["triggered", "resolved", "no-data"]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `is_active` (Boolean) Set to true for the silence to suppress matching notifications while its schedule is active. Set to false to keep the silence without suppressing anything.
- `name` (String) Display name of the silence
- `owner` (String) Team ID that owns and manages the silence
- `priorities_filter` (List of Number) Monitor priorities that must match for this silence to apply. An empty array matches every priority.
- `reason` (String) Free-text explanation of why notifications are being silenced, shown to other operators reviewing active silences.
- `schedule` (Attributes) (see [below for nested schema](#nestedatt--schedule))
- `teams_filter` (Attributes) Team visibility filter used when matching alert transitions. Tsuga validates create and update requests against the caller permissions. (see [below for nested schema](#nestedatt--teams_filter))
- `transition_types_filter` (List of String) Alert state transitions that must match for this silence to apply. An empty array matches every transition type.

### Optional

- `notification_rule_ids` (List of String) Notification rule IDs this silence applies to directly, matching any notification produced by one of these rules regardless of priority or transition type. Set only when `priorities_filter`, `transition_types_filter`, and `query_string` are not used to filter alerts.
- `query_string` (String) Query on the alerting monitor tags and group-by dimensions, e.g. `env:prod AND service:bridge`. The monitor itself can be targeted with `monitor_id:<monitor id>`.
- `tags` (Attributes List) Key/value tags to apply to the resource. Tag policies may require specific keys or values. (see [below for nested schema](#nestedatt--tags))

### Read-Only

- `id` (String) Tsuga-generated notification silence ID assigned when the silence is created.

<a id="nestedatt--schedule"></a>
### Nested Schema for `schedule`

Optional:

- `one_time` (Attributes) One-time silence schedule with specific start and end times (see [below for nested schema](#nestedatt--schedule--one_time))
- `recurring` (Attributes) Recurring weekly silence schedule (see [below for nested schema](#nestedatt--schedule--recurring))

<a id="nestedatt--schedule--one_time"></a>
### Nested Schema for `schedule.one_time`

Required:

- `end_time` (String) Local end date and time for the one-time silence. It must be after `start_time` and in the future.
- `start_time` (String) Local start date and time for the one-time silence, interpreted in `time_zone` when provided.

Optional:

- `time_zone` (String) IANA timezone identifier (e.g. "America/New_York"). Defaults to UTC if omitted.


<a id="nestedatt--schedule--recurring"></a>
### Nested Schema for `schedule.recurring`

Required:

- `weekly_schedule` (Attributes) Recurring silence windows keyed by day of week, evaluated in the schedule `time_zone` or in UTC when it is omitted. At least one day must be listed, and a day with no entry is never silenced. Each window covers `start_time` up to but not including `end_time`. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule))

Optional:

- `time_zone` (String) IANA timezone identifier (e.g. "America/New_York"). Defaults to UTC if omitted.

<a id="nestedatt--schedule--recurring--weekly_schedule"></a>
### Nested Schema for `schedule.recurring.weekly_schedule`

Optional:

- `friday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--friday))
- `monday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--monday))
- `saturday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--saturday))
- `sunday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--sunday))
- `thursday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--thursday))
- `tuesday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--tuesday))
- `wednesday` (Attributes List) Silence windows for this day. Each window uses the schedule time zone, and windows on the same day cannot overlap. (see [below for nested schema](#nestedatt--schedule--recurring--weekly_schedule--wednesday))

<a id="nestedatt--schedule--recurring--weekly_schedule--friday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.friday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--monday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.monday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--saturday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.saturday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--sunday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.sunday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--thursday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.thursday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--tuesday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.tuesday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.


<a id="nestedatt--schedule--recurring--weekly_schedule--wednesday"></a>
### Nested Schema for `schedule.recurring.weekly_schedule.wednesday`

Required:

- `end_time` (String) Local end time for this recurring silence window on this day. It must be after `start_time`, and ranges on the same day cannot overlap.
- `start_time` (String) Local start time for this recurring silence window on this day, using the schedule time zone.





<a id="nestedatt--teams_filter"></a>
### Nested Schema for `teams_filter`

Required:

- `type` (String) `specific-teams`: Match alert transitions associated with the listed teams. The caller must be allowed to access every listed team. Requires `teams`. `all-teams`: Match alert transitions for every team in the organization. Public API callers need an API key with `all` scope to create or update this filter. `all-public-teams`: Match alert transitions for all public teams. Public API callers need an API key with `all` or `owning-team-and-public` scope to create or update this filter.

Optional:

- `teams` (List of String) Team IDs selected by this filter.


<a id="nestedatt--tags"></a>
### Nested Schema for `tags`

Required:

- `key` (String) Tag key to attach to the resource.
- `value` (String) Tag value to attach to the resource. Leading or trailing whitespace is rejected.
