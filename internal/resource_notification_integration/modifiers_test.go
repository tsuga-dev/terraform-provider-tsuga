package resource_notification_integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func setting(active string) basetypes.ObjectValue {
	blockType := types.ObjectType{AttrTypes: map[string]attr.Type{"key": types.StringType}}
	attrTypes := map[string]attr.Type{"pagerduty": blockType, "webhook": blockType}
	values := map[string]attr.Value{"pagerduty": types.ObjectNull(blockType.AttrTypes), "webhook": types.ObjectNull(blockType.AttrTypes)}
	if active != "" {
		values[active] = types.ObjectValueMust(blockType.AttrTypes, map[string]attr.Value{"key": types.StringValue("secret")})
	}
	return types.ObjectValueMust(attrTypes, values)
}

func TestSettingTypeRequiresReplace(t *testing.T) {
	cases := []struct {
		name      string
		state     basetypes.ObjectValue
		plan      basetypes.ObjectValue
		wantPlace bool
	}{
		{"same variant", setting("pagerduty"), setting("pagerduty"), false},
		{"other variant", setting("pagerduty"), setting("webhook"), true},
		{"created", types.ObjectNull(setting("").AttributeTypes(context.Background())), setting("webhook"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &planmodifier.ObjectResponse{}
			settingTypeRequiresReplaceModifier{}.PlanModifyObject(context.Background(), planmodifier.ObjectRequest{StateValue: tc.state, PlanValue: tc.plan}, resp)
			if resp.RequiresReplace != tc.wantPlace {
				t.Fatalf("RequiresReplace = %t, want %t", resp.RequiresReplace, tc.wantPlace)
			}
		})
	}
}
