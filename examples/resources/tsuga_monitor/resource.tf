resource "tsuga_monitor" "monitor" {
  name        = "No logs"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 1
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
      condition = {
        formula   = "q1"
        operator  = "equal"
        threshold = 0
      },
      timeframe = 10,
      group_by_fields = [{
        fields = ["context.env"]
        limit  = 10
      }]
      no_data_behavior        = "alert"
      aggregation_alert_logic = "no_aggregation"
    }
  }
}
