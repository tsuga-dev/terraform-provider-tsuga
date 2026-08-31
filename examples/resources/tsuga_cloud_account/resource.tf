# AWS cloud account. The cross-account IAM role must already exist and trust Tsuga
# with the given external ID before applying.
resource "tsuga_cloud_account" "aws_prod" {
  account_friendly_name = "Production AWS"

  aws = {
    account_id  = "123456789012"
    external_id = "tsuga-external-id"
    role_arn    = "arn:aws:iam::123456789012:role/tsuga-inventory"
  }
}

# GCP cloud account. The workload identity provider and service account must already
# be configured before applying.
resource "tsuga_cloud_account" "gcp_prod" {
  account_friendly_name = "Production GCP"

  gcp = {
    project_id                 = "my-gcp-project"
    service_account_id         = "tsuga-inventory@my-gcp-project.iam.gserviceaccount.com"
    workload_identity_provider = "projects/123/locations/global/workloadIdentityPools/tsuga/providers/tsuga"
  }
}

# Azure cloud account. The application registration must already carry a federated
# credential trusting Tsuga, and hold Reader on the subscription, before applying.
resource "tsuga_cloud_account" "azure_prod" {
  account_friendly_name = "Production Azure"

  azure = {
    client_id       = "11111111-1111-1111-1111-111111111111"
    subscription_id = "22222222-2222-2222-2222-222222222222"
    tenant_id       = "33333333-3333-3333-3333-333333333333"
  }
}
