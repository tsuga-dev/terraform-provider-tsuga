# Tag policy for telemetry data
resource "tsuga_tag_policy" "telemetry-policy" {
  name        = "environment-tag-policy"
  description = "Enforces environment tag on all telemetry data"
  is_active   = true
  tag_key     = "environment"
  allowed_tag_values = [
    "production",
    "staging",
    "development",
  ]
  is_required = true
  owner       = "abc-123-def"

  configuration = {
    telemetry = {
      asset_types           = ["logs", "metrics", "traces"]
      should_insert_warning = true
      drop_sample           = 25.0
    }
  }
}

# Tag policy for Tsuga assets with team scope
resource "tsuga_tag_policy" "asset-policy" {
  name        = "cost-center-tag-policy"
  description = "Enforces cost-center tag on dashboards and monitors"
  is_active   = true
  tag_key     = "cost-center"
  allowed_tag_values = [
    "engineering",
    "platform",
    "infrastructure",
  ]
  is_required = false
  owner       = "abc-123-def"

  team_scope = {
    team_ids = ["ghi-456-jkl"]
    mode     = "include"
  }

  configuration = {
    tsuga_asset = {
      asset_types = [
        "dashboard",
        "monitor",
        "notification-rule",
      ]
    }
  }
}

# Tag policy excluding specific teams
resource "tsuga_tag_policy" "exclude-policy" {
  name        = "service-tag-policy"
  description = "Enforces service tag on all teams except excluded ones"
  is_active   = true
  tag_key     = "service"
  allowed_tag_values = [
    "api",
    "web",
    "worker",
    "scheduler",
  ]
  is_required = true
  owner       = "abc-123-def"

  team_scope = {
    team_ids = ["ghi-456-jkl"]
    mode     = "exclude"
  }

  configuration = {
    tsuga_asset = {
      asset_types = [
        "ingestion-api-key",
        "operation-api-key",
        "log-route",
      ]
    }
  }
}
