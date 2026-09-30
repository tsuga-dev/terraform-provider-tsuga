resource "tsuga_metric_route" "metric_route" {
  name              = "my-metric-route-name"
  owner             = "abc-123-def"
  is_enabled        = true
  metric_name_regex = "^http\\.server\\..*$"

  processors = [
    {
      id = "service-name-mapper"
      mapper = {
        map_attributes = [
          {
            origin_attribute = "svc"
            target_attribute = "service.name"
          }
        ]
      }
    },
    {
      id = "metric-name-parser"
      parse_attribute = {
        grok = {
          attribute_name = "metric.name"
          rules = [
            "%%{WORD:namespace}\\.%%{WORD:subsystem}\\.%%{GREEDYDATA:metric}",
          ]
        }
      }
    },
    {
      id = "unit-creator"
      creator = {
        format_string = {
          target_attribute = "unit"
          format_string    = "{{namespace}}_{{subsystem}}_unit"
        }
      }
    },
    {
      id = "severity-creator"
      creator = {
        category = {
          target_attribute = "severity"
          clauses = [
            {
              query = "status_code:>=500"
              value = "error"
            },
            {
              query = "status_code:>=400"
              value = "warning"
            }
          ]
          default_value = "info"
        }
      }
    }
  ]
}
