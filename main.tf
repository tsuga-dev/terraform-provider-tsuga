terraform {
  required_providers {
    tsuga = {
      source = "hashicorp.com/edu/tsuga"
    }
  }
}

provider "tsuga" {
  // Local development
  # base_url = "http://127.0.0.1:3101"
  # Token can also be set via TSUGA_TOKEN environment variable
  # token = "your-bearer-token"
}


resource "tsuga_team" "test-team" {
  name        = "test-team-terraform"
  description = "Test from Terraform"
  visibility  = "public"
  tags = [
    {
      key   = "test"
      value = "test"
    }
  ]
}

resource "tsuga_notification_rule" "test-notification-rule" {
  is_active         = true
  name              = "test-notification-rule"
  owner             = "4efk-hyf69-t2wy"
  priorities_filter = [1, 2, 3, ]
  tags = [
    {
      key   = "test-key"
      value = "test-value"
    },
  ]
  targets = [
    {
      config = {
        email = {
          addresses = [
            "test@example.com",
          ]
        }
      }
      id = "123"
    },
    {
      config = {
        slack = {
          channel        = "C0123456789"
          integration_id = "T06T0BAKV35"
        }
      }
      id = "456"
    },
  ]
  teams_filter            = []
  transition_types_filter = []
}

resource "tsuga_dashboard" "test-dashboard" {
  name  = "Test Terraform Import"
  owner = tsuga_team.test-team.id
  filters = [
    "context.env:staging"
  ]
  graphs = [
    {
      id   = "60c3-p99ke-evyc",
      name = "Test Graph",
      layout = {
        x = 0
        y = 0
        w = 3
        h = 4
      },
      visualization = {
        timeseries = {
          source = "logs",
          queries = [
            {
              aggregate = {
                count = {}
              },
              filter = ""
            }
          ]
        },
      }
    }
  ]
  tags = [
    {
      key   = "mykey"
      value = "myvalue"
    }
  ]
}

resource "tsuga_route" "test-route" {
  name       = "Team Central"
  owner      = tsuga_team.test-team.id
  is_enabled = true
  query      = "context.team:central"

  processors = []
  tags = [
    {
      key   = "team"
      value = "central"
    }
  ]
}
