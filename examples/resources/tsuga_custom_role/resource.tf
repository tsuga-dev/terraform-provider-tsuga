resource "tsuga_custom_role" "example" {
  name        = "platform-on-call"
  description = "Edits monitors and dashboards of the platform team, and reads its production telemetry."

  team_scope = {
    type     = "specific"
    team_ids = ["abc-123-def"]
  }

  assets = {
    team             = "read"
    dashboard        = "write"
    monitor          = "write"
    "monitor-folder" = "read"
  }

  telemetry = ["log", "metric", "trace"]

  telemetry_query = [
    {
      key    = "context.env"
      values = ["production"]
    }
  ]
}
