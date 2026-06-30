resource "tsuga_slo" "api_availability" {
  name           = "API availability 99.9%"
  description    = "Overall API gateway availability over the last 28 days"
  owner          = "abc-123-def"
  permissions    = "all"
  target         = 99.9
  timeframe_days = 28
  cluster_ids    = []

  configuration = {
    event = {
      data_source = "logs"
      good_query = {
        formula = "q1"
        queries = [{
          filter    = "status:ok"
          aggregate = { count = {} }
        }]
      }
      total_query = {
        formula = "q1"
        queries = [{
          filter    = "*"
          aggregate = { count = {} }
        }]
      }
      no_data_behavior = "good"
    }
  }

  alerts = [
    {
      priority = 1
      configuration = {
        burn_rate = 14.4
      }
    },
    {
      priority = 3
      configuration = {
        threshold = 99.0
      }
    }
  ]
}

resource "tsuga_slo" "api_latency" {
  name           = "API latency under 300ms"
  description    = "p95 request latency stays under 300ms over the last 30 days"
  owner          = "abc-123-def"
  permissions    = "all"
  target         = 99.0
  timeframe_days = 30

  configuration = {
    time = {
      data_source = "traces"
      query = {
        formula = "q1"
        queries = [{
          filter = "service:api"
          aggregate = {
            percentile = {
              field      = "duration"
              percentile = 95
            }
          }
        }]
      }
      slice_size_minutes = 5
      threshold = {
        operator = "less_than"
        value    = 300
      }
      group_by_fields = [{
        fields = ["endpoint"]
        limit  = 10
      }]
      no_data_behavior = "ignore"
    }
  }

  alerts = [
    {
      priority = 2
      configuration = {
        burn_rate = 6
      }
    }
  ]
}
