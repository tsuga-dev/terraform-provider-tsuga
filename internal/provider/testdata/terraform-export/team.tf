resource "tsuga_team" "team" {
  description = "A team"
  name        = "example"
  visibility  = "public"
}

import {
  to = tsuga_team.team
  id = "resource-123"
}
