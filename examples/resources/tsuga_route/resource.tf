resource "tsuga_route" "route" {
  name       = "my-route-name"
  owner      = "abc-123-def"
  is_enabled = true
  query      = "context.team:my-team-name"

  processors = [
    {
      id = "message-standardizer"
      mapper = {
        map_attributes = [
          {
            origin_attribute = "msg"
            target_attribute = "message"
          }
        ]
      }
    },
    {
      id = "nginx-parser"
      parse_attribute = {
        grok = {
          attribute_name = "message"
          rules = [
            "%%{GREEDYDATA:timestamp} \\[%%{WORD:level}\\] %%{GREEDYDATA:error.message}, client: %%{IP:network.source.ip}, server: %%{NOTSPACE:http.server}, request: \"%%{WORD:http.method} %%{NOTSPACE:http.url} HTTP\\/%%{NUMBER:http.version:number}\", host: \"%%{NOTSPACE:http.host}\"",
            "%%{GREEDYDATA:timestamp} \\[%%{WORD:level}\\] %%{GREEDYDATA:error.message}",
          ]
        }
      }
    },
    {
      id = "audit-log-message-creator"
      creator = {
        format_string = {
          target_attribute = "message"
          format_string    = "{{user}} has successfully {{action.outcome}} {{asset.name}} at {{timestamp}}"
        }
      }
    },
    {
      id = "latency-creator"
      creator = {
        math_formula = {
          target_attribute = "latency"
          formula          = "{{intake.latency_s}} * 1000 + {{processing.latency_ms}} + {{store.latency_ms}}"
        }
      }
    },
    {
      id = "otel-splitter"
      split = {
        items = [
          {
            query = "scope.name:*"
            processors = [
              {
                id = "otel-severity-mapper"
                mapper = {
                  map_level = {
                    attribute_name = "severity"
                  }
                }
              }
            ]
          }
        ]
      }
    }
  ]
}
