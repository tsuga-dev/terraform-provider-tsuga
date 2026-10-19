resource "time_offset" "example" {
  offset_days = 30
}

resource "tsuga_operation_api_key" "example" {
  name       = "terraform-production"
  owner      = "abc-123-def"
  scope      = "owning-team-only"
  expires_at = time_offset.example.rfc3339

  permissions = {
    dashboard = "write"
    monitor   = "write"
    logs      = "read"
  }

  tags = [
    {
      key   = "env"
      value = "prod"
    }
  ]
}
