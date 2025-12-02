# The provider can be configured with a base URL and a token
# It can also be configured with the TSUGA_BASE_URL and TSUGA_TOKEN environment variables
provider "tsuga" {
  base_url = "https://api.tsuga.com"
  token    = "your-api-token"
}
