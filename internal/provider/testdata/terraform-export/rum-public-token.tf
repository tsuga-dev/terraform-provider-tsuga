resource "tsuga_rum_public_token" "rum_public_token" {
  allowed_url_prefixes = ["https://app.example.com"]
  application_type     = "web"
  name                 = "example-web-app"
  owner                = "team-123"
  tags = [{
    key   = "env"
    value = "production"
  }]
  team_override_fields = ["service"]
}

import {
  to = tsuga_rum_public_token.rum_public_token
  id = "resource-123"
}
