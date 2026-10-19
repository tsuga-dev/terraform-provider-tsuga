resource "tsuga_rum_public_token" "example" {
  name                 = "production-web-app"
  owner                = "abc-123-def"
  application_type     = "web"
  allowed_url_prefixes = ["https://app.example.com"]

  tags = [
    {
      key   = "env"
      value = "prod"
    }
  ]
}
