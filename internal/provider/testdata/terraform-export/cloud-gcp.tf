resource "tsuga_cloud_account" "cloud_gcp" {
  account_friendly_name = "Production"
  gcp = {
    project_id                 = "example-project"
    service_account_id         = var.tsuga_cloud_account_cloud_gcp_input_1
    workload_identity_provider = var.tsuga_cloud_account_cloud_gcp_input_2
  }
}

variable "tsuga_cloud_account_cloud_gcp_input_1" {
  type        = string
  description = "Supply gcp.service_account_id for the exported resource."
  nullable    = false
}

variable "tsuga_cloud_account_cloud_gcp_input_2" {
  type        = string
  description = "Supply gcp.workload_identity_provider for the exported resource."
  nullable    = false
}
