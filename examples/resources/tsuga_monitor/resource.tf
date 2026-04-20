resource "tsuga_monitor" "monitor" {
  name        = "No logs"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 1
  cluster_ids = ["cluster-1", "cluster-2"]
  configuration = {
    log = {
      queries = [
        {
          filter = "context.env:prod"
          aggregate = {
            count = {}
          }
        }
      ]
      conditions = [{
        formula   = "q1"
        operator  = "equal"
        threshold = 0
      }]
      timeframe = 10,
      group_by_fields = [{
        fields = ["context.env"]
        limit  = 10
      }]
      no_data_behavior        = "alert"
      aggregation_alert_logic = "each"
    }
  }
}

resource "tsuga_monitor" "log_error_pattern" {
  name        = "New Error Pattern Detector"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 2
  message     = "A new error pattern was detected in the logs."

  configuration = {
    log_error_pattern = {
      aggregation_alert_logic = "each"
      no_data_behavior        = "keep_last_status"
      filter = {
        team_ids = ["abc-123-def"]
        env      = "production"
        service  = "api-gateway"
      }
    }
  }
}

resource "tsuga_monitor" "certificate_expiry_monitor" {
  name        = "Certificate expiry warning"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 1
  configuration = {
    certificate_expiry = {
      warn_before_in_days     = 30
      cloud_accounts          = ["aws-prod", "gcp-shared"]
      aggregation_alert_logic = "each"
      no_data_behavior        = "resolve"
    }
  }
}
