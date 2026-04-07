package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNotificationSilenceResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with recurring schedule
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_silence" "test-silence" {
  name        = "test-notification-silence"
  reason = "A test silence for maintenance"
  owner       = tsuga_team.test-team.id
  is_active   = true

  schedule = {
    recurring = {
      monday = [
        {
          start_time = "00:00:00"
          end_time   = "06:00:00"
        }
      ]
      wednesday = [
        {
          start_time = "12:00:00"
          end_time   = "14:00:00"
        }
      ]
    }
  }

  teams_filter = {
    type  = "specific-teams"
    teams = [tsuga_team.test-team.id]
  }

  priorities_filter       = [1, 2, 3]
  transition_types_filter = ["triggered"]

  tags = [
    {
      key   = "env"
      value = "test"
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "name", "test-notification-silence"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "reason", "A test silence for maintenance"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "is_active", "true"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "priorities_filter.#", "3"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "priorities_filter.0", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "priorities_filter.1", "2"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "priorities_filter.2", "3"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "teams_filter.type", "specific-teams"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "teams_filter.teams.#", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "transition_types_filter.#", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "transition_types_filter.0", "triggered"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.monday.0.start_time", "00:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.monday.0.end_time", "06:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.wednesday.0.start_time", "12:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.wednesday.0.end_time", "14:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "tags.0.key", "env"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "tags.0.value", "test"),
				),
			},
			// Update recurring schedule
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_silence" "test-silence" {
  name        = "test-notification-silence-updated"
  reason = "Updated test silence"
  owner       = tsuga_team.test-team.id
  is_active   = false

  schedule = {
    recurring = {
      monday = [
        {
          start_time = "00:00:00"
          end_time   = "06:00:00"
        }
      ]
      friday = [
        {
          start_time = "18:00:00"
          end_time   = "23:59:00"
        }
      ]
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  priorities_filter       = [1]
  transition_types_filter = ["triggered", "resolved"]

  tags = [
    {
      key   = "env"
      value = "staging"
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "name", "test-notification-silence-updated"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "reason", "Updated test silence"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "is_active", "false"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "teams_filter.type", "all-teams"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "priorities_filter.#", "1"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "transition_types_filter.#", "2"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.monday.0.start_time", "00:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.monday.0.end_time", "06:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.friday.0.start_time", "18:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence", "schedule.recurring.friday.0.end_time", "23:59:00"),
				),
			},
		},
	})
}

func TestAccNotificationSilenceResource_OneTime(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with one-time schedule
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_silence" "test-silence-onetime" {
  name      = "test-onetime-silence"
  reason    = "Planned database migration"
  owner     = tsuga_team.test-team.id
  is_active = true

  schedule = {
    one_time = {
      start_time = "2030-06-15T02:00:00"
      end_time   = "2030-06-15T06:00:00"
      time_zone  = "America/New_York"
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  priorities_filter       = [1, 2, 3]
  transition_types_filter = ["triggered", "resolved"]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "name", "test-onetime-silence"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "reason", "Planned database migration"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "is_active", "true"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.start_time", "2030-06-15T02:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.end_time", "2030-06-15T06:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.time_zone", "America/New_York"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "teams_filter.type", "all-teams"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "priorities_filter.#", "3"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "transition_types_filter.#", "2"),
				),
			},
			// Update one-time schedule times and timezone
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_notification_silence" "test-silence-onetime" {
  name      = "test-onetime-silence-updated"
  reason    = "Extended maintenance window"
  owner     = tsuga_team.test-team.id
  is_active = true

  schedule = {
    one_time = {
      start_time = "2030-06-15T01:00:00"
      end_time   = "2030-06-15T08:00:00"
      time_zone  = "Europe/London"
    }
  }

  teams_filter = {
    type = "all-teams"
  }

  priorities_filter       = [1, 2, 3, 4, 5]
  transition_types_filter = ["triggered", "resolved", "no-data"]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "name", "test-onetime-silence-updated"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "reason", "Extended maintenance window"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.start_time", "2030-06-15T01:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.end_time", "2030-06-15T08:00:00"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "schedule.one_time.time_zone", "Europe/London"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "priorities_filter.#", "5"),
					resource.TestCheckResourceAttr("tsuga_notification_silence.test-silence-onetime", "transition_types_filter.#", "3"),
				),
			},
		},
	})
}
