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
