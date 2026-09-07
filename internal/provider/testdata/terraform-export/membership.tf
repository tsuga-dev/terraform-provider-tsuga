resource "tsuga_team_membership" "membership" {
  role_key = "editor"
  team_id  = "team-123"
  user_id  = "user-123"
}

import {
  to = tsuga_team_membership.membership
  id = "user-123:team-123"
}
