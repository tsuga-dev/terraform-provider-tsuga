resource "tsuga_team" "team" {
  description = "A team"
  name        = "Example"
  visibility  = "public"
}

import {
  to = tsuga_team.team
  id = "resource-123"
}
