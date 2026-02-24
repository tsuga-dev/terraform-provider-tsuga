resource "tsuga_ingestion_api_key" "example" {
  name  = "production-logs-ingestion"
  owner = "abc-123-def"

  tags = [
    {
      key   = "env"
      value = "prod"
    }
  ]
}
