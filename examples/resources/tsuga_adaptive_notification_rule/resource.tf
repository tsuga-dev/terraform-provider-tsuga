# Adaptive rules only accept trace monitors (or metric monitors over OTel span metrics) with a
# single condition, and SLOs over traces (or OTel span metrics).
resource "tsuga_monitor" "checkout_errors" {
  name        = "Checkout errors"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 1
  cluster_ids = []
  configuration = {
    trace = {
      queries = [{
        filter = "service:checkout status:error"
        aggregate = {
          count = {}
        }
      }]
      conditions = [{
        formula   = "q1"
        operator  = "greater_than"
        threshold = 10
      }]
      timeframe               = 5
      group_by_fields         = []
      no_data_behavior        = "resolve"
      aggregation_alert_logic = "no_aggregation"
    }
  }
}

resource "tsuga_slo" "checkout_availability" {
  name           = "Checkout availability 99.9%"
  owner          = "abc-123-def"
  permissions    = "all"
  target         = 99.9
  timeframe_days = 30
  cluster_ids    = []

  configuration = {
    event = {
      data_source = "traces"
      good_query = {
        formula = "q1"
        queries = [{
          filter    = "service:checkout status:ok"
          aggregate = { count = {} }
        }]
      }
      total_query = {
        formula = "q1"
        queries = [{
          filter    = "service:checkout"
          aggregate = { count = {} }
        }]
      }
      no_data_behavior = "good"
    }
  }

  alerts = [{
    priority = 1
    configuration = {
      burn_rate = 14.4
    }
  }]
}

resource "tsuga_adaptive_notification_rule" "adaptive-notification-rule" {
  name                        = "checkout-errors"
  owner                       = "abc-123-def"
  is_active                   = true
  should_create_investigation = true
  monitor_ids                 = [tsuga_monitor.checkout_errors.id]
  slo_ids                     = [tsuga_slo.checkout_availability.id]
  tags = [
    {
      key   = "env"
      value = "prod"
    },
  ]
  fallback_target = {
    id = "fallback"
    config = {
      email = {
        addresses = [
          "oncall@example.com",
        ]
      }
    }
  }
}
