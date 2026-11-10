package provider

import (
	"context"
	"testing"

	"terraform-provider-tsuga/internal/resource_cloud_account"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildConnectionSettings_Aws(t *testing.T) {
	plan := resource_cloud_account.CloudAccountModel{
		Aws: resource_cloud_account.NewAwsValueMust(resource_cloud_account.AwsValue{}.AttributeTypes(context.Background()), map[string]attr.Value{
			"account_id":  types.StringValue("123456789012"),
			"external_id": types.StringValue("ext-id"),
			"role_arn":    types.StringValue("arn:aws:iam::123456789012:role/tsuga"),
		}),
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
		Gcp: resource_cloud_account.NewGcpValueMust(resource_cloud_account.GcpValue{}.AttributeTypes(context.Background()), map[string]attr.Value{
			"project_id":                 types.StringValue("my-project-1"),
			"service_account_id":         types.StringValue("sa@my-project.iam.gserviceaccount.com"),
			"workload_identity_provider": types.StringValue("//iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/tsuga"),
		}),
	}

	settings, cloudType, cloudAccountId := expandConnectionSettings(plan)

	if cloudType != "gcp" || cloudAccountId != "my-project-1" {
		t.Fatalf("cloudType=%q cloudAccountId=%q", cloudType, cloudAccountId)
	}
	if settings["projectId"] != "my-project-1" || settings["workloadIdentityProvider"] != "//iam.googleapis.com/projects/1/locations/global/workloadIdentityPools/p/providers/tsuga" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestBuildConnectionSettings_Azure(t *testing.T) {
	plan := resource_cloud_account.CloudAccountModel{
		Azure: resource_cloud_account.NewAzureValueMust(resource_cloud_account.AzureValue{}.AttributeTypes(context.Background()), map[string]attr.Value{
			"client_id":       types.StringValue("11111111-1111-1111-1111-111111111111"),
			"subscription_id": types.StringValue("22222222-2222-2222-2222-222222222222"),
			"tenant_id":       types.StringValue("33333333-3333-3333-3333-333333333333"),
		}),
	}

	settings, cloudType, cloudAccountId := expandConnectionSettings(plan)

	if cloudType != "azure" || cloudAccountId != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("cloudType=%q cloudAccountId=%q", cloudType, cloudAccountId)
	}
	if settings["type"] != "azure" || settings["clientId"] != "11111111-1111-1111-1111-111111111111" || settings["tenantId"] != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}
