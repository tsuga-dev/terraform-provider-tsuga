package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDashboardResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_dashboard" "test" {
  name    = "test-dashboard"
  owner   = tsuga_team.test-team.id
  filters = ["env:test"]

  graphs = [
    {
      id   = "graph-1"
      name = "Graph One"
      layout = {
        x = 0
        y = 0
        w = 6
        h = 4
      }
      visualization = {
        note = {
          note       = "hello world"
          note_align = "center"
          note_color = "blue.200"
        }
      }
    },
    {
      id   = "graph-2"
      name = "Timeseries Graph"
      visualization = {
        timeseries = {
          source = "logs"
          queries = [{
            aggregate = {
              count = {}
            }
          }]
          formula = "a"
        }
      }
    },
    {
      id   = "graph-3"
      name = "List Graph"
      visualization = {
        list = {
          source = "logs"
          query  = "service:api"
          list_columns = [{
            attribute = "attr1"
          }]
        }
      }
    },
    {
      id   = "graph-4"
      name = "Top List Graph"
      visualization = {
        top_list = {
          source = "logs"
          queries = [{
            aggregate = {
              sum = {
                field = "duration"
              }
            }
          }]
        }
      }
    },
    {
      id   = "graph-5"
      name = "Pie Graph"
      visualization = {
        pie = {
          source = "logs"
          queries = [{
            aggregate = {
              average = {
                field = "latency"
              }
            }
          }]
        }
      }
    },
    {
      id   = "graph-6"
      name = "Query Value Graph"
      visualization = {
        query_value = {
          source = "metrics"
          queries = [{
            aggregate = {
              max = {
                field = "value"
              }
            }
          }]
          background_mode = "background"
        }
      }
    },
    {
      id   = "graph-7"
      name = "Bar Graph"
      visualization = {
        bar = {
          source = "logs"
          queries = [{
            aggregate = {
              min = {
                field = "duration"
              }
            }
          }]
          time_bucket = {
            time   = 60
            metric = "sec"
          }
        }
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "name", "test-dashboard"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.0.visualization.note.note", "hello world"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.#", "7"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_dashboard" "test" {
  name    = "test-dashboard-updated"
  owner   = tsuga_team.test-team.id
  filters = ["env:prod"]

  graphs = [
    {
      id   = "graph-1"
      name = "Graph One Updated"
      visualization = {
        note = {
          note                 = "updated"
          note_justify_content = "flex-end"
        }
      }
    },
    {
      id   = "graph-2"
      name = "Timeseries Graph"
      visualization = {
        query_value = {
          source = "metrics"
          queries = [{
            aggregate = {
              sum = {
                field = "duration"
              }
            }
          }]
          background_mode = "no-background"
        }
      }
    },
    {
      id   = "graph-3"
      name = "List Graph"
      visualization = {
        bar = {
          source = "logs"
          queries = [{
            aggregate = {
              count = {}
            }
          }]
        }
      }
    },
    {
      id   = "graph-4"
      name = "Top List Graph Updated"
      visualization = {
        top_list = {
          source = "logs"
          queries = [{
            aggregate = {
              percentile = {
                field      = "pct"
                percentile = 95
              }
            }
          }]
        }
      }
    },
    {
      id   = "graph-5"
      name = "Pie Graph Updated"
      visualization = {
        pie = {
          source = "logs"
          queries = [{
            aggregate = {
              unique_count = {
                field = "user"
              }
            }
          }]
        }
      }
    },
    {
      id   = "graph-6"
      name = "Query Value Graph Updated"
      visualization = {
        timeseries = {
          source = "metrics"
          queries = [{
            aggregate = {
              max = {
                field = "value"
              }
            }
          }]
        }
      }
    },
    {
      id   = "graph-7"
      name = "Bar Graph Updated"
      visualization = {
        list = {
          source = "logs"
          query  = "service:web"
          list_columns = [{
            attribute = "attr2"
          }]
        }
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "name", "test-dashboard-updated"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.0.visualization.note.note", "updated"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.0.name", "Graph One Updated"),
					resource.TestCheckResourceAttr("tsuga_dashboard.test", "graphs.#", "7"),
				),
			},
		},
	})
}
