# Example: PagerDuty integration
# Secrets are write-only: they are sent to Tsuga but never stored in the plan or state.
# Bump `secrets_version` after rotating a secret so Terraform sends the new value.
resource "tsuga_notification_integration" "pagerduty" {
  name            = "primary-on-call"
  secrets_version = "1"

  setting = {
    pagerduty = {
      integration_key = "abcdef1234567890abcdef1234567890"
    }
  }
}

# Example: Webhook integration with bearer auth and a JSON payload template
resource "tsuga_notification_integration" "webhook" {
  name            = "internal-webhook"
  secrets_version = "1"

  setting = {
    webhook = {
      url    = "https://example.com/hooks/tsuga"
      method = "POST"

      authentication = {
        bearer = { token = "shhh" }
      }

      payload_template = {
        type = "json"
        value = jsonencode({
          title  = "{{ alert_title }}"
          status = "{{ alert_status }}"
        })
      }
    }
  }

  tags = [
    {
      key   = "team"
      value = "platform"
    },
  ]
}

# Example: Jira integration
resource "tsuga_notification_integration" "jira" {
  name            = "jira-ops"
  secrets_version = "1"

  setting = {
    jira = {
      site_url  = "https://your-company.atlassian.net"
      email     = "ops@your-company.com"
      api_token = "your-atlassian-api-token"
    }
  }
}
