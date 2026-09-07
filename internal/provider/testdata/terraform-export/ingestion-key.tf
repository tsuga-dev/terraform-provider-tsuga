resource "tsuga_ingestion_api_key" "ingestion_key" {
  name  = "Example"
  owner = "team-123"
  tags  = []
}

import {
  to = tsuga_ingestion_api_key.ingestion_key
  id = "resource-123"
}
