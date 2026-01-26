resource "tsuga_notification_rule" "notification-rule" {
  name              = "notification-rule"
  owner             = "abc-123-def"
  priorities_filter = [1, 2, 3]
  teams_filter = {
    type  = "specific-teams"
    teams = ["ghi-456-jkl"]
  }
  transition_types_filter = ["triggered", "resolved"]
  is_active               = true
  tags = [
    {
      key   = "env"
      value = "prod"
    },
  ]
  targets = [
    {
      id = "123"
      config = {
        email = {
          addresses = [
            "test@example.com",
          ]
        }
      }
    },
    {
      id = "456"
      config = {
        slack = {
          channel        = "{{Your Slack Channel ID}}"
          integration_id = "{{Your Slack Integration ID}}"
        }
      }
      rate_limit = {
        max_messages = 5
        minutes      = 10
      }
      renotify_config = {
        mode                      = "each"
        renotification_states     = ["alert"]
        renotify_interval_minutes = 120
      }
    },
  ]
}
