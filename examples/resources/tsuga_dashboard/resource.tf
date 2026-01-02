resource "tsuga_dashboard" "dashboard" {
  name        = "Kubernetes Dashboard"
  owner       = "abc-123-def"
  time_preset = "past-15-minutes"
  filters = [
    "context.k8s.namespace.name",
    "context.k8s.pod.name",
    "context.env"
  ]
  graphs = [
    {
      id   = "node-memory-usage-percentage"
      name = "Node Memory Usage (%)"
      layout = {
        x = 6
        y = 6
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source  = "metrics"
          formula = "(q1 / (q1 + q2)) * 100"

          group_by = [
            {
              limit  = 5
              fields = ["context.k8s.node.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.node.memory.usage"
                }
              }
            },
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.node.memory.available"
                }
              }
            }
          ]

          visible_series = [
            false,
            false,
            true
          ]
        }
      }
    },
    {
      id   = "node-cpu-usage-cores"
      name = "Node CPU Usage (Cores)"
      layout = {
        x = 0
        y = 6
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source = "metrics"
          group_by = [
            {
              limit  = 5
              fields = ["context.k8s.node.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                average = {
                  field = "k8s.node.cpu.usage"
                }
              }
            }
          ]
        }
      }
    },
    {
      id   = "pod-cpu-usage-cores"
      name = "Pod CPU Usage (Cores)"
      layout = {
        x = 0
        y = 12
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source = "metrics"

          group_by = [
            {
              limit  = 5
              fields = ["context.k8s.pod.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                average = {
                  field = "k8s.pod.cpu.usage"
                }
              }
            }
          ]
        }
      }
    },
    {
      id   = "pod-memory-usage-percentage"
      name = "Pod Memory Usage (%)"
      layout = {
        x = 6
        y = 12
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source  = "metrics"
          formula = "(q1 / (q1 + q2)) * 100"

          group_by = [
            {
              limit  = 5
              fields = ["context.k8s.pod.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.pod.memory.usage"
                }
              }
            },
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.pod.memory.available"
                }
              }
            }
          ]

          visible_series = [
            false,
            false,
            true
          ]
        }
      }
    },
    {
      id   = "memory-usage-by-namespace"
      name = "Memory usage by namespace"
      layout = {
        x = 3
        y = 0
        w = 3
        h = 5
      }
      visualization = {
        pie = {
          source = "metrics"
          group_by = [
            {
              limit  = 100
              fields = ["context.k8s.namespace.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                sum = {
                  field = "k8s.pod.memory.usage"
                }
              }
            }
          ]

          normalizer = {
            type = "data"
            unit = "B"
          }
        }
      }
    },
    {
      id   = "vcores-used-over-time"
      name = "vCores used over time"
      layout = {
        x = 6
        y = 0
        w = 6
        h = 5
      }
      visualization = {
        bar = {
          source = "metrics"
          queries = [
            {
              filter = ""
              aggregate = {
                sum = {
                  field = "k8s.node.cpu.usage"
                }
              }
            }
          ]

          time_bucket = {
            time   = 60
            metric = "min"
          }
        }
      }
    },
    {
      id   = "nodes"
      name = "Nodes"
      layout = {
        x = 0
        y = 5
        w = 12
        h = 1
      }
      visualization = {
        note = {
          note                 = "# Nodes"
          note_align           = "center"
          note_color           = "emerald.200"
          note_justify_content = "center"
        }
      }
    },
    {
      id   = "pods"
      name = "Pods"
      layout = {
        x = 0
        y = 11
        w = 12
        h = 1
      }
      visualization = {
        note = {
          note                 = "# Pods"
          note_align           = "center"
          note_color           = "blue.200"
          note_justify_content = "center"
        }
      }
    },
    {
      id   = "vcore-usage-by-namespace"
      name = "vCore usage by namespace"
      layout = {
        x = 0
        y = 0
        w = 3
        h = 5
      }
      visualization = {
        pie = {
          source = "metrics"
          group_by = [
            {
              limit  = 100
              fields = ["context.k8s.namespace.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                sum = {
                  field = "k8s.pod.cpu.usage"
                }
              }
            }
          ]
        }
      }
    },
    {
      id   = "volumes"
      name = "Volumes"
      layout = {
        x = 0
        y = 17
        w = 6
        h = 1
      }
      visualization = {
        note = {
          note                 = "# Volumes"
          note_align           = "center"
          note_color           = "fuchsia.200"
          note_justify_content = "center"
        }
      }
    },
    {
      id   = "volume-usage-percentage"
      name = "Volume usage (%)"
      layout = {
        x = 0
        y = 18
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source  = "metrics"
          formula = "((q2 - q1) / q2) * 100"
          group_by = [
            {
              limit  = 10
              fields = ["context.cluster_id"]
            },
            {
              limit  = 10
              fields = ["context.k8s.volume.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.volume.available"
                }
              }
            },
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.volume.capacity"
                }
              }
            }
          ]

          visible_series = [
            false,
            false,
            true
          ]
        }
      }
    },
    {
      id   = "network"
      name = "Network"
      layout = {
        x = 6
        y = 17
        w = 6
        h = 1
      }
      visualization = {
        note = {
          note                 = "# Network"
          note_align           = "center"
          note_color           = "amber.200"
          note_justify_content = "center"
        }
      }
    },
    {
      id   = "network-io-rate"
      name = "Network IO (rate)"
      layout = {
        x = 6
        y = 18
        w = 6
        h = 5
      }
      visualization = {
        timeseries = {
          source = "metrics"
          group_by = [
            {
              limit  = 10
              fields = ["context.cluster_id"]
            },
            {
              limit  = 100
              fields = ["context.k8s.node.name"]
            }
          ]

          queries = [
            {
              filter = ""
              aggregate = {
                max = {
                  field = "k8s.node.network.io"
                }
              }
              functions = [
                {
                  type = "rate"
                }
              ]
            }
          ]

          normalizer = {
            type = "data"
            unit = "B"
          }
        }
      }
    },
    {
      id   = "events"
      name = "Events"
      layout = {
        x = 0
        y = 23
        w = 12
        h = 1
      }
      visualization = {
        note = {
          note                 = "# Events"
          note_align           = "center"
          note_color           = "lime.200"
          note_justify_content = "center"
        }
      }
    },
    {
      id   = "k8s-events-all"
      name = "K8s Events (All)"
      layout = {
        x = 0
        y = 24
        w = 12
        h = 7
      }
      visualization = {
        list = {
          source = "logs"
          query  = "object.reason:* OR object.note:*"

          list_columns = [
            {
              attribute = "context.cluster_id"
            },
            {
              attribute = "type"
            },
            {
              attribute = "object.reportingController"
            },
            {
              attribute = "object.note"
            },
            {
              attribute = "object.reason"
            }
          ]
        }
      }
    },
    {
      id   = "errors"
      name = "Errors"
      layout = {
        x = 0
        y = 31
        w = 12
        h = 5
      }
      visualization = {
        query_value = {
          source = "logs"
          queries = [
            {
              filter = "level:ERROR"
              aggregate = {
                count = {}
              }
            }
          ]
          conditions = [
            {
              color    = "alert"
              value    = 0
              operator = "greater_than"
            }
          ]
          background_mode = "background"
        }
      }
    }
  ]
  tags = [
    {
      key   = "env"
      value = "prod"
    }
  ]
}
