package provider

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// sloImportAlertsCheck verifies the imported SLO kept its alerts: the expected count, each with a
// server-assigned id. It is order-independent, so it complements ImportStateVerifyIgnore("alerts")
// (which only skips the order-sensitive comparison) rather than blanket-ignoring alerts.
func sloImportAlertsCheck(want int) resource.ImportStateCheckFunc {
	return func(states []*terraform.InstanceState) error {
		for _, s := range states {
			got, ok := s.Attributes["alerts.#"]
			if !ok {
				continue
			}
			if got != strconv.Itoa(want) {
				return fmt.Errorf("imported alerts.# = %s, want %d", got, want)
			}
			for i := 0; i < want; i++ {
				if id := s.Attributes[fmt.Sprintf("alerts.%d.id", i)]; id == "" {
					return fmt.Errorf("imported alerts.%d.id is empty", i)
				}
			}
			return nil
		}
		return fmt.Errorf("no tsuga_slo instance with alerts found in imported state")
	}
}

// sloCaptureAlertID stores alerts.<idx>.id into dst so a later step can compare against it.
func sloCaptureAlertID(res string, idx int, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[res]
		if !ok || rs == nil {
			return fmt.Errorf("resource %s not found in state", res)
		}
		id := rs.Primary.Attributes[fmt.Sprintf("alerts.%d.id", idx)]
		if id == "" {
			return fmt.Errorf("alerts.%d.id is empty", idx)
		}
		*dst = id
		return nil
	}
}

// sloAssertAlertIDChanged asserts alerts.<idx>.id differs from prev — i.e. the alert was recreated
// (deleted and re-created with a new server id) rather than updated in place.
func sloAssertAlertIDChanged(res string, idx int, prev *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[res]
		if !ok || rs == nil {
			return fmt.Errorf("resource %s not found in state", res)
		}
		id := rs.Primary.Attributes[fmt.Sprintf("alerts.%d.id", idx)]
		if id == "" {
			return fmt.Errorf("alerts.%d.id is empty", idx)
		}
		if id == *prev {
			return fmt.Errorf("expected the edited alert to be recreated with a new id, but it stayed %q", id)
		}
		return nil
	}
}

// TestAccSloResource_AlertConfigChangeRecreatesAlert documents that editing an alert's
// configuration recreates it: alerts are reconciled to prior state by content (priority +
// configuration), so a changed alert no longer matches any prior alert, is sent without an id, and
// the API assigns it a new one (deleting the old). The alert's id therefore changes across the edit.
func TestAccSloResource_AlertConfigChangeRecreatesAlert(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))
	var alertID string

	cfg := func(burnRate string) string {
		return providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-alert-recreate-slo"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 99.0
  timeframe_days = 28

  configuration = {
    event = {
      data_source = "logs"
      good_query  = { formula = "q1", queries = [{ filter = "status:ok", aggregate = { count = {} } }] }
      total_query = { formula = "q1", queries = [{ filter = "*", aggregate = { count = {} } }] }
      no_data_behavior = "good"
    }
  }

  alerts = [
    { priority = 1, configuration = { burn_rate = %s } },
  ]
}
`, teamName, burnRate)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("14.4"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "14.4"),
					sloCaptureAlertID("tsuga_slo.test", 0, &alertID),
				),
			},
			{
				// Change only the burn rate. The alert must come back with a new id (recreated).
				Config: cfg("6"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "6"),
					sloAssertAlertIDChanged("tsuga_slo.test", 0, &alertID),
				),
			},
		},
	})
}

// sloDistinctAlertIds asserts that two alert entries have non-empty, distinct server ids. Used to
// prove that two alerts with identical values are tracked as separate resources.
func sloDistinctAlertIds(res string, i, j int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[res]
		if !ok || rs == nil {
			return fmt.Errorf("resource %s not found in state", res)
		}
		a := rs.Primary.Attributes[fmt.Sprintf("alerts.%d.id", i)]
		b := rs.Primary.Attributes[fmt.Sprintf("alerts.%d.id", j)]
		if a == "" || b == "" {
			return fmt.Errorf("alert ids empty: alerts.%d.id=%q alerts.%d.id=%q", i, a, j, b)
		}
		if a == b {
			return fmt.Errorf("expected two identical alerts to have distinct server ids, both = %q", a)
		}
		return nil
	}
}

// TestAccSloResource_DuplicateAndReorderAlerts exercises alert reconciliation by id: two alerts
// with identical values get distinct ids, an unrelated update keeps them stable , removing one
// duplicate drops the count, and reordering distinct alerts must not produce an "inconsistent
// result after apply".
func TestAccSloResource_DuplicateAndReorderAlerts(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	cfg := func(target, alerts string) string {
		return providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-dup-slo"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = %s
  timeframe_days = 28

  configuration = {
    event = {
      data_source = "logs"
      good_query = {
        formula = "q1"
        queries = [{ filter = "status:ok", aggregate = { count = {} } }]
      }
      total_query = {
        formula = "q1"
        queries = [{ filter = "*", aggregate = { count = {} } }]
      }
      no_data_behavior = "good"
    }
  }

  alerts = %s
}
`, teamName, target, alerts)
	}

	twoIdenticalPlusOne := `[
    { priority = 3, configuration = { threshold = 99 } },
    { priority = 3, configuration = { threshold = 99 } },
    { priority = 1, configuration = { burn_rate = 14.4 } },
  ]`
	removedDuplicate := `[
    { priority = 3, configuration = { threshold = 99 } },
    { priority = 1, configuration = { burn_rate = 14.4 } },
  ]`
	reordered := `[
    { priority = 1, configuration = { burn_rate = 14.4 } },
    { priority = 3, configuration = { threshold = 99 } },
  ]`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create with two identical threshold alerts -> they must get distinct server ids.
				Config: cfg("99.9", twoIdenticalPlusOne),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "3"),
					sloDistinctAlertIds("tsuga_slo.test", 0, 1),
				),
			},
			{
				// Unrelated change (target only). Alerts are unchanged, so the implicit post-apply
				// plan must be empty (no churn) and ids must stay distinct.
				Config: cfg("99.5", twoIdenticalPlusOne),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "target", "99.5"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "3"),
					sloDistinctAlertIds("tsuga_slo.test", 0, 1),
				),
			},
			{
				// Remove one of the two identical alerts -> count drops to 2.
				Config: cfg("99.5", removedDuplicate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.threshold", "99"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.configuration.burn_rate", "14.4"),
					resource.TestCheckResourceAttrSet("tsuga_slo.test", "alerts.0.id"),
				),
			},
			{
				// Reorder distinct alerts -> config order wins, with no inconsistent-result error.
				Config: cfg("99.5", reordered),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "14.4"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.configuration.threshold", "99"),
				),
			},
		},
	})
}

func TestAccSloResource_Event(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-event-slo"
  description    = "Availability over the last 28 days"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 99.9
  timeframe_days = 28

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
        threshold = 99
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "name", "test-event-slo"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "target", "99.9"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "timeframe_days", "28"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "permissions", "all"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.event.data_source", "logs"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.event.good_query.formula", "q1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.event.no_data_behavior", "good"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.priority", "1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "14.4"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.priority", "3"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.configuration.threshold", "99"),
					resource.TestCheckResourceAttrSet("tsuga_slo.test", "alerts.0.id"),
				),
			},
			{
				// Change target, and add/remove/reorder alerts (drop the burn-rate, change the
				// threshold, add a new burn-rate at the end).
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-event-slo-updated"
  owner          = tsuga_team.test-team.id
  permissions    = "owning-team-only"
  target         = 99.5
  timeframe_days = 30

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
      no_data_behavior = "bad"
    }
  }

  alerts = [
    {
      priority = 2
      configuration = {
        threshold = 95
      }
    },
    {
      priority = 1
      configuration = {
        burn_rate = 6
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "name", "test-event-slo-updated"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "target", "99.5"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "timeframe_days", "30"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "permissions", "owning-team-only"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.event.no_data_behavior", "bad"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.priority", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.threshold", "95"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.priority", "1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.configuration.burn_rate", "6"),
				),
			},
			{
				ResourceName:      "tsuga_slo.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The SLO API returns alerts unordered, so an imported SLO's alert order need not
				// match the config order from the prior apply; ignore only that order-sensitive
				// comparison. ImportStateCheck still verifies the alerts survived the round trip.
				ImportStateVerifyIgnore: []string{"alerts"},
				ImportStateCheck:        sloImportAlertsCheck(2),
			},
		},
	})
}

func TestAccSloResource_Time(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-time-slo"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 99.0
  timeframe_days = 7

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
      group_by_fields = [
        {
          fields = ["service"]
          limit  = 10
        },
        {
          fields = ["endpoint"]
          limit  = 5
        }
      ]
      no_data_behavior = "ignore"
    }
  }

  alerts = [
    {
      priority = 2
      configuration = {
        burn_rate = 6
      }
    },
    {
      priority = 4
      configuration = {
        threshold = 90
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "name", "test-time-slo"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "target", "99"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "timeframe_days", "7"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.data_source", "traces"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.slice_size_minutes", "5"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.threshold.operator", "less_than"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.threshold.value", "300"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.no_data_behavior", "ignore"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.group_by_fields.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.group_by_fields.0.fields.0", "service"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.group_by_fields.1.fields.0", "endpoint"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "2"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "6"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.1.configuration.threshold", "90"),
				),
			},
			{
				// Change slice size, threshold, drop an alert.
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-time-slo-updated"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 98.0
  timeframe_days = 90

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
              percentile = 99
            }
          }
        }]
      }
      slice_size_minutes = 10
      threshold = {
        operator = "less_than_or_equal"
        value    = 500
      }
      no_data_behavior = "bad"
    }
  }

  alerts = [
    {
      priority = 1
      configuration = {
        burn_rate = 14.4
      }
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "name", "test-time-slo-updated"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "target", "98"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "timeframe_days", "90"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.slice_size_minutes", "10"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.threshold.operator", "less_than_or_equal"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.threshold.value", "500"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.no_data_behavior", "bad"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.#", "1"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "alerts.0.configuration.burn_rate", "14.4"),
				),
			},
			{
				ResourceName:      "tsuga_slo.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The SLO API returns alerts unordered, so an imported SLO's alert order need not
				// match the config order from the prior apply; ignore only that order-sensitive
				// comparison. ImportStateCheck still verifies the alerts survived the round trip.
				ImportStateVerifyIgnore: []string{"alerts"},
				ImportStateCheck:        sloImportAlertsCheck(1),
			},
		},
	})
}

func TestAccSloResource_EventToTimeSwitch(t *testing.T) {
	teamName := fmt.Sprintf("test-%s", randomString(10))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-switch-slo"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 99.9
  timeframe_days = 28

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
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.event.data_source", "logs"),
				),
			},
			{
				// Switch the same SLO from an event configuration to a time configuration.
				Config: providerConfig + fmt.Sprintf(`
resource "tsuga_team" "test-team" {
  name       = "%s"
  visibility = "public"
}

resource "tsuga_slo" "test" {
  name           = "test-switch-slo"
  owner          = tsuga_team.test-team.id
  permissions    = "all"
  target         = 99.9
  timeframe_days = 28

  configuration = {
    time = {
      data_source = "metrics"
      query = {
        formula = "q1"
        queries = [{
          filter = "service:api"
          aggregate = {
            average = {
              field = "latency"
            }
          }
        }]
      }
      slice_size_minutes = 1
      threshold = {
        operator = "less_than"
        value    = 250
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
    }
  ]
}
`, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.data_source", "metrics"),
					resource.TestCheckResourceAttr("tsuga_slo.test", "configuration.time.threshold.value", "250"),
					resource.TestCheckNoResourceAttr("tsuga_slo.test", "configuration.event.data_source"),
				),
			},
		},
	})
}
