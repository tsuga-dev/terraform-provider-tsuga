resource "tsuga_team" "team" {
  name        = "my-team-name"
  description = "My team description"
  visibility  = "public"
  tags = [
    {
      key   = "billing_center"
      value = "product"
    }
  ]
}
