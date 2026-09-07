resource "tsuga_cloud_account" "cloud_aws" {
  account_friendly_name = "Production"
  aws = {
    account_id  = "123456789012"
    external_id = var.tsuga_cloud_account_cloud_aws_input_1
    role_arn    = var.tsuga_cloud_account_cloud_aws_input_2
  }
}

variable "tsuga_cloud_account_cloud_aws_input_1" {
  type        = string
  description = "Supply aws.external_id for the exported resource."
  nullable    = false
}

variable "tsuga_cloud_account_cloud_aws_input_2" {
  type        = string
  description = "Supply aws.role_arn for the exported resource."
  nullable    = false
}
