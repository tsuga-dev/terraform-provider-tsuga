# Top-level dashboard folder
resource "tsuga_dashboard_folder" "platform" {
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
resource "tsuga_dashboard_folder" "platform-ingestion" {
  name             = "Ingestion"
  owner            = "abc-123-def"
  parent_folder_id = tsuga_dashboard_folder.platform.id
}

# Dashboards join a folder through their own folder_id
resource "tsuga_dashboard" "ingestion-overview" {
  name      = "Ingestion Overview"
  owner     = "abc-123-def"
  folder_id = tsuga_dashboard_folder.platform-ingestion.id
  graphs = [
    {
      id   = "ingestion-runbook"
      name = "Runbook"
      layout = {
        x = 0
        y = 0
        w = 6
        h = 3
      }
      visualization = {
        note = {
          note = "Escalate ingestion lag to the platform on-call."
        }
      }
    }
  ]
}
