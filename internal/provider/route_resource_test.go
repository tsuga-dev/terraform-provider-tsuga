package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRouteResource(t *testing.T) {
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

resource "tsuga_route" "test" {
  name       = "test-route"
  owner      = tsuga_team.test-team.id
  is_enabled = true
  query      = "true"

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
          attribute_name = "message"
          rules          = [""]
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
      id   = "splitter-1"
      split = {
        items = [
          {
            query = "status:ok"
            processors = [
              {
                id   = "mapper-2"
                mapper = {
                  map_level = {
                    attribute_name = "level"
                  }
                }
              },
              {
                id   = "splitter-2"
                split = {
                  items = [
                    {
                      query = "status:deep"
                      processors = [
                        {
                          id   = "mapper-3"
                          mapper = {
                            map_timestamp = {
                              attribute_name = "ts_nested"
                            }
                          }
                        },
                        {
                          id   = "splitter-3"
                          split = {
                            items = [
                              {
                                query = "status:deeper"
                                processors = [
                                  {
                                    id   = "mapper-4"
                                    mapper = {
                                      map_attributes = [
                                        {
                                          origin_attribute = "foo"
                                          target_attribute = "bar"
                                        }
                                      ]
                                    }
                                  }
                                ]
                              }
                            ]
                          }
                        }
                      ]
                    }
                  ]
                }
              }
            ]
          }
        ]
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_route.test", "name", "test-route"),
					resource.TestCheckResourceAttr("tsuga_route.test", "is_enabled", "true"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.#", "4"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.0.mapper.map_attributes.#", "1"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.1.parse_attribute.grok.attribute_name", "message"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.2.creator.format_string.target_attribute", "formatted"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.3.split.items.#", "1"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.3.split.items.0.processors.#", "2"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.3.split.items.0.processors.1.split.items.0.processors.1.split.items.0.processors.0.mapper.map_attributes.#", "1"),
				),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_route" "test" {
  name       = "test-route-updated"
  owner      = tsuga_team.test-team.id
  is_enabled = false
  query      = "severity:info"

  processors = [
    {
      id   = "mapper-1"
      mapper = {
        map_timestamp = {
          attribute_name = "ts"
        }
      }
    },
    {
      id   = "parser-1"
      parse_attribute = {
        key_value = {
          source_attribute      = "msg"
          target_attribute      = "kv"
          key_value_splitter    = "="
          pairs_splitter        = ","
          accept_standalone_key = false
        }
      }
    },
    {
      id   = "creator-1"
      creator = {
        math_formula = {
          target_attribute = "val"
          formula          = "1+1"
        }
      }
    },
    {
      id   = "splitter-1"
      split = {
        items = [
          {
            query = "status:error"
            processors = [
              {
                id   = "parser-2"
                parse_attribute = {
                  url = {
                    source_attribute = "url"
                  }
                }
              },
              {
                id   = "splitter-3"
                split = {
                  items = [
                    {
                      query = "status:deep"
                      processors = [
                        {
                          id   = "parser-3"
                          parse_attribute = {
                            key_value = {
                              source_attribute      = "msg"
                              target_attribute      = "kv_nested"
                              key_value_splitter    = "="
                              pairs_splitter        = ","
                              accept_standalone_key = false
                            }
                          }
                        },
                        {
                          id   = "splitter-4"
                          split = {
                            items = [
                              {
                                query = "status:deeper"
                                processors = [
                                  {
                                    id   = "parser-4"
                                    parse_attribute = {
                                      grok = {
                                        attribute_name = "deep_message"
                                        rules          = [".*"]
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
                  ]
                }
              }
            ]
          }
        ]
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_route.test", "name", "test-route-updated"),
					resource.TestCheckResourceAttr("tsuga_route.test", "is_enabled", "false"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.#", "4"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.1.parse_attribute.key_value.target_attribute", "kv"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.3.split.items.0.processors.0.parse_attribute.url.source_attribute", "url"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.3.split.items.0.processors.1.split.items.0.processors.1.split.items.0.processors.0.parse_attribute.grok.attribute_name", "deep_message"),
				),
			},
		},
	})
}

func TestAccRouteResource_EmptyProcessors(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(8))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with empty processors
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_route" "test" {
  name       = "test-route-empty"
  owner      = tsuga_team.test-team.id
  is_enabled = true
  query      = "true"
  processors = []
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_route.test", "name", "test-route-empty"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.#", "0"),
				),
			},
			// Update: add a processor
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_route" "test" {
  name       = "test-route-with-processor"
  owner      = tsuga_team.test-team.id
  is_enabled = true
  query      = "true"
  processors = [
    {
      id   = "mapper-1"
      mapper = {
        map_level = {
          attribute_name = "level"
        }
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_route.test", "name", "test-route-with-processor"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.#", "1"),
				),
			},
			// Update: back to empty processors
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name = "%s"
  visibility = "public"
}

resource "tsuga_route" "test" {
  name       = "test-route-empty-again"
  owner      = tsuga_team.test-team.id
  is_enabled = true
  query      = "true"
  processors = []
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_route.test", "name", "test-route-empty-again"),
					resource.TestCheckResourceAttr("tsuga_route.test", "processors.#", "0"),
				),
			},
		},
	})
}
