resource "tsuga_retention_policy" "retention_policy" {
  env           = "prod"
  team_id       = "123-abc-456"
  data_source   = "logs"
  duration_days = "30-days"
  is_enabled    = true
}
