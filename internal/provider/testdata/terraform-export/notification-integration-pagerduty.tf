resource "tsuga_notification_integration" "notification_integration_pagerduty" {
  name = "Example"
  setting = {
    pagerduty = {
      integration_key = var.tsuga_notification_integration_notification_integration_pagerduty_input_1
    }
  }
  tags = [{
    key   = "team"
    value = "platform"
  }]
}

variable "tsuga_notification_integration_notification_integration_pagerduty_input_1" {
  type        = string
  description = "Supply setting.pagerduty.integration_key for the exported resource."
  nullable    = false
  sensitive   = true
}

import {
  to = tsuga_notification_integration.notification_integration_pagerduty
  id = "resource-123"
}
