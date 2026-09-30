package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMetricRouteResource(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_metric_route" "test" {
  name              = "test-metric-route"
  owner             = tsuga_team.test-team.id
  is_enabled        = true
  metric_name_regex = "^http\\.server\\..*$"

  processors = [
    {
      id   = "mapper-1"
      mapper = {
        map_attributes = [
          {
            origin_attribute = "orig"
            target_attribute = "dest"
            keep_origin      = true
          }
        ]
      }
    },
    {
      id   = "parser-1"
      parse_attribute = {
        grok = {
          attribute_name = "metric.name"
          rules          = ["%%%%{GREEDYDATA:metric}"]
        }
      }
    },
    {
      id   = "creator-1"
      creator = {
        format_string = {
          target_attribute = "formatted"
          format_string    = "val"
        }
      }
    },
    {
      id   = "creator-2"
      creator = {
        category = {
          target_attribute = "severity"
          clauses = [
            {
              query = "status_code:>=500"
              value = "error"
            }
          ]
          default_value = "info"
        }
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "name", "test-metric-route"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "is_enabled", "true"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.#", "4"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.0.mapper.map_attributes.#", "1"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.1.parse_attribute.grok.attribute_name", "metric.name"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.2.creator.format_string.target_attribute", "formatted"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.3.creator.category.target_attribute", "severity"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_metric_route" "test" {
  name              = "test-metric-route-updated"
  owner             = tsuga_team.test-team.id
  is_enabled        = false
  metric_name_regex = "^http\\.client\\..*$"

  processors = []
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "name", "test-metric-route-updated"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "is_enabled", "false"),
					resource.TestCheckResourceAttr("tsuga_metric_route.test", "processors.#", "0"),
				),
			},
		},
	})
}
