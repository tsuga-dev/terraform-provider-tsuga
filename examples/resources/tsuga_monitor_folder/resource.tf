# Top-level monitor folder
resource "tsuga_monitor_folder" "platform" {
  name  = "Platform"
  owner = "abc-123-def"
  tags = [
    {
      key   = "team"
      value = "platform"
    }
  ]
}

# Folders nest one level deep: a child folder points at a top-level folder
resource "tsuga_monitor_folder" "platform-ingestion" {
  name             = "Ingestion"
  owner            = "abc-123-def"
  parent_folder_id = tsuga_monitor_folder.platform.id
}

# Monitors join a folder through their own folder_id
resource "tsuga_monitor" "ingestion-no-logs" {
  name        = "No ingestion logs"
  owner       = "abc-123-def"
  permissions = "all"
  priority    = 2
  folder_id   = tsuga_monitor_folder.platform-ingestion.id
  configuration = {
    log = {
      queries = [
        {
          filter = "service:ingestion"
          aggregate = {
            count = {}
          }
        }
      ]
      conditions = [{
        formula   = "q1"
        operator  = "equal"
        threshold = 0
      }]
      timeframe               = 10
      group_by_fields         = []
      no_data_behavior        = "alert"
      aggregation_alert_logic = "no_aggregation"
    }
  }
}
