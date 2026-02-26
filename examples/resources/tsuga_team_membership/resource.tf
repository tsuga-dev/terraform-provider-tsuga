data "tsuga_user" "jane" {
  id = "usr-abc-123"
}

resource "tsuga_team" "backend" {
  name       = "backend"
  visibility = "public"
}

resource "tsuga_team_membership" "jane_backend" {
  user_id  = data.tsuga_user.jane.id
  team_id  = tsuga_team.backend.id
  role_key = "editor"
}
