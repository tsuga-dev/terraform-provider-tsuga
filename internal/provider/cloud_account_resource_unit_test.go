package provider

import (
	"testing"

	"terraform-provider-tsuga/internal/resource_cloud_account"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildConnectionSettings_Aws(t *testing.T) {
	plan := resource_cloud_account.CloudAccountModel{
		Aws: &resource_cloud_account.AwsSettingsModel{
			AccountId:  types.StringValue("123456789012"),
			ExternalId: types.StringValue("ext-id"),
			RoleArn:    types.StringValue("arn:aws:iam::123456789012:role/tsuga"),
		},
	}

	settings, cloudType, cloudAccountId := expandConnectionSettings(plan)

	if cloudType != "aws" {
		t.Fatalf("cloudType = %q, want aws", cloudType)
	}
	// cloud_account_id is derived from the block's primary identifier.
	if cloudAccountId != "123456789012" {
		t.Fatalf("cloudAccountId = %q, want 123456789012", cloudAccountId)
	}
	if settings["type"] != "aws" || settings["roleArn"] != "arn:aws:iam::123456789012:role/tsuga" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestBuildConnectionSettings_Gcp(t *testing.T) {
	plan := resource_cloud_account.CloudAccountModel{
		Gcp: &resource_cloud_account.GcpSettingsModel{
			ProjectId:                types.StringValue("my-project"),
			ServiceAccountId:         types.StringValue("sa@my-project.iam"),
			WorkloadIdentityProvider: types.StringValue("projects/1/providers/tsuga"),
		},
	}

	settings, cloudType, cloudAccountId := expandConnectionSettings(plan)

	if cloudType != "gcp" || cloudAccountId != "my-project" {
		t.Fatalf("cloudType=%q cloudAccountId=%q", cloudType, cloudAccountId)
	}
	if settings["projectId"] != "my-project" || settings["workloadIdentityProvider"] != "projects/1/providers/tsuga" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestBuildConnectionSettings_Azure(t *testing.T) {
	plan := resource_cloud_account.CloudAccountModel{
		Azure: &resource_cloud_account.AzureSettingsModel{
			ClientId:       types.StringValue("11111111-1111-1111-1111-111111111111"),
			SubscriptionId: types.StringValue("22222222-2222-2222-2222-222222222222"),
			TenantId:       types.StringValue("33333333-3333-3333-3333-333333333333"),
		},
	}

	settings, cloudType, cloudAccountId := expandConnectionSettings(plan)

	if cloudType != "azure" || cloudAccountId != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("cloudType=%q cloudAccountId=%q", cloudType, cloudAccountId)
	}
	if settings["type"] != "azure" || settings["clientId"] != "11111111-1111-1111-1111-111111111111" || settings["tenantId"] != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}
