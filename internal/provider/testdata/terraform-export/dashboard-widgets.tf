resource "tsuga_dashboard" "dashboard_widgets" {
  graphs = [{
    id = "timeseries"
    visualization = {
      timeseries = {
        custom_link = {
          label = "Open service"
          url   = "https://example.com/services/{{group.service.name}}"
        }
        group_by = [{
          fields = ["service.name"]
          limit  = 10
        }]
        group_by_mode = "relative"
        line_style_options = {
          "1" = {
            group_id   = "a"
            line_style = "dashed"
          }
        }
        queries = [{
          aggregate = {
            count = {}
          }
          fill = {
            mode = {
              type = "zero"
            }
          }
          filter = "events.type:view"
          functions = [{
            type = "cumsum"
          }]
          }, {
          aggregate = {
            sum = {
              field = "http.requests"
            }
          }
          metric_kind = "delta-counter"
        }]
        source = "rum"
        thresholds = [{
          level = "alert"
          value = 100
        }]
      }
    }
    }, {
    id = "bar"
    visualization = {
      bar = {
        custom_link = {
          url = "https://example.com"
        }
        group_by_mode = "absolute"
        queries = [{
          aggregate = {
            count = {}
          }
        }]
        source = "logs"
        thresholds = [{
          level = "warning"
          value = 5
        }]
      }
    }
    }, {
    id = "pie"
    visualization = {
      pie = {
        custom_link = {
          url = "https://example.com/pie"
        }
        group_by_mode = "relative"
        queries = [{
          aggregate = {
            count = {}
          }
        }]
        source = "traces"
      }
    }
    }, {
    id = "top-list"
    visualization = {
      top_list = {
        custom_link = {
          url = "https://example.com/top"
        }
        group_by_mode = "absolute"
        legend_mode   = "table"
        queries = [{
          aggregate = {
            count = {}
          }
        }]
        source = "logs"
      }
    }
    }, {
    id = "table"
    visualization = {
      table = {
        column_sizes = {
          "col-0" = 120
          "label" = 200
        }
        columns = [{
          name = "Errors"
          queries = [{
            aggregate = {
              count = {}
            }
            fill = {
              mode = {
                type = "null"
              }
            }
          }]
          source = "rum"
        }]
        custom_link = {
          url = "https://example.com/table"
        }
        default_sorting = [{
          desc = true
          id   = "col-0"
        }]
        group_by = [{
          fields = ["service.name"]
          limit  = 5
        }]
        group_by_mode = "relative"
      }
    }
    }, {
    id = "heatmap"
    visualization = {
      heatmap = {
        group = "trace_id"
        queries = [{
          aggregate = {
            average = {
              field = "duration"
            }
          }
        }]
        source = "traces"
        y_axis_settings = {
          always_include_zero = false
          max = {
            type  = "number"
            value = 10
          }
          min = {
            type = "auto"
          }
          scale = {
            type = "log"
          }
        }
      }
    }
    }, {
    id = "distribution"
    visualization = {
      distribution = {
        group = "trace_id"
        queries = [{
          aggregate = {
            average = {
              field = "duration"
            }
          }
        }]
        source = "traces"
      }
    }
    }, {
    id = "timeseries-connection"
    visualization = {
      timeseries_connection = {
        connection_id = "connection-123"
        line_style_options = {
          "1" = {
            group_id   = "a"
            line_style = "regular"
          }
        }
        queries = ["SELECT 1"]
      }
    }
    }, {
    id = "timeseries-promql"
    visualization = {
      timeseries_promql = {
        aliases = {
          queries = {
            "0" = "Requests"
          }
        }
        legend_mode = "legend-only"
        line_style_options = {
          "1" = {
            group_id   = "a"
            line_style = "dashed"
          }
        }
        normalizer = {
          type = "percent"
        }
        precision = 2
        queries   = ["sum(rate(http_requests_total[5m]))"]
        smoothing = true
        thresholds = [{
          level = "success"
          value = 1
        }]
        time_bucket = {
          metric = "min"
          time   = 5
        }
        y_axis_settings = {
          always_include_zero = true
          max = {
            type = "auto"
          }
          min = {
            type = "auto"
          }
          scale = {
            exponent = 2
            type     = "pow"
          }
        }
      }
    }
    }, {
    id = "bar-promql"
    visualization = {
      bar_promql = {
        legend_mode = "no-legend"
        queries     = ["sum(up)"]
        thresholds = [{
          level = "alert"
          value = 3
        }]
        time_bucket = {
          metric = "hour"
          time   = 1
        }
      }
    }
    }, {
    id = "pie-promql"
    visualization = {
      pie_promql = {
        legend_mode = "table"
        queries     = ["sum by (job) (up)"]
      }
    }
    }, {
    id = "query-value-promql"
    visualization = {
      query_value_promql = {
        background_mode = "background"
        conditions = [{
          color    = "alert"
          operator = "less_than"
          value    = 1
        }]
        queries = ["sum(up)"]
      }
    }
    }, {
    id = "top-list-promql"
    visualization = {
      top_list_promql = {
        conditions = [{
          color    = "success"
          operator = "greater_than"
          value    = 0
        }]
        queries = ["topk(5, up)"]
      }
    }
    }, {
    id = "list-rum"
    visualization = {
      list_rum = {
        default_sorting = [{
          desc = false
          id   = "view.name"
        }]
        is_cell_wrapped = true
        list_columns = [{
          attribute = "view.name"
        }]
        query = "events.type:error"
      }
    }
    }, {
    id = "slo"
    visualization = {
      slo = {
        display = ["summary", "table"]
        filters = {
          owners = {
            values = ["team-123"]
          }
          statuses = {
            exclude = true
            values  = ["breached", "no_data"]
          }
          tags = {
            values = [{
              key   = "env"
              value = "prod"
            }]
          }
        }
        organize_by = {
          key  = "env"
          type = "key"
        }
      }
    }
    }, {
    id = "slo-over-time"
    visualization = {
      slo_over_time = {
        slo_id = "slo-123"
      }
    }
    }, {
    id = "slo-uptime"
    visualization = {
      slo_uptime = {
        slo_id = "slo-123"
      }
    }
  }]
  name        = "Widgets"
  owner       = "team-123"
  time_preset = "past-15-days"
}

import {
  to = tsuga_dashboard.dashboard_widgets
  id = "resource-456"
}
