resource "tsuga_notification_integration" "notification_integration_webhook" {
  name = "Example"
  setting = {
    webhook = {
      authentication = {
        bearer = {
          token = var.tsuga_notification_integration_notification_integration_webhook_input_1
        }
      }
      custom_headers = {
        "X-Env" = "prod"
      }
      event_name_mapping = {
        alert = "firing"
        ok    = "resolved"
      }
      method = "POST"
      payload_template = {
        type  = "json"
        value = "{\"status\":\"{{ alert_status }}\",\"title\":\"{{ alert_title }}\"}"
      }
      url = var.tsuga_notification_integration_notification_integration_webhook_input_2
    }
  }
}

variable "tsuga_notification_integration_notification_integration_webhook_input_1" {
  type        = string
  description = "Supply setting.webhook.authentication.bearer.token for the exported resource."
  nullable    = false
  sensitive   = true
}

variable "tsuga_notification_integration_notification_integration_webhook_input_2" {
  type        = string
  description = "Supply setting.webhook.url for the exported resource."
  nullable    = false
  sensitive   = true
}

import {
  to = tsuga_notification_integration.notification_integration_webhook
  id = "resource-123"
}
