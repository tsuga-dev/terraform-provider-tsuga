resource "tsuga_dashboard_folder" "folder" {
  name             = "Example"
  owner            = "team-123"
  parent_folder_id = "parent-123"
}

import {
  to = tsuga_dashboard_folder.folder
  id = "resource-123"
}
