resource "tsuga_monitor_folder" "monitor_folder" {
  name             = "Example"
  owner            = "team-123"
  parent_folder_id = "parent-123"
}

import {
  to = tsuga_monitor_folder.monitor_folder
  id = "resource-123"
}
