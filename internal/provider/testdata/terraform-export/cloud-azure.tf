resource "tsuga_cloud_account" "cloud_azure" {
  account_friendly_name = "Production"
  azure = {
    client_id       = var.tsuga_cloud_account_cloud_azure_input_1
    subscription_id = "00000000-0000-0000-0000-000000000001"
    tenant_id       = var.tsuga_cloud_account_cloud_azure_input_2
  }
}

variable "tsuga_cloud_account_cloud_azure_input_1" {
  type        = string
  description = "Supply azure.client_id for the exported resource."
  nullable    = false
}

variable "tsuga_cloud_account_cloud_azure_input_2" {
  type        = string
  description = "Supply azure.tenant_id for the exported resource."
  nullable    = false
}
