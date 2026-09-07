resource "tsuga_tag_policy" "tag_policy" {
  allowed_tag_values = ["prod", "staging"]
  configuration = {
    tsuga_asset = {
      asset_types = ["dashboard"]
    }
  }
  is_active   = true
  is_required = true
  name        = "Example"
  owner       = "team-123"
  tag_key     = "env"
}

import {
  to = tsuga_tag_policy.tag_policy
  id = "resource-123"
}
