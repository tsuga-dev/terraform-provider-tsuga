package resource_notification_integration

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// settingTypeRequiresReplaceModifier forces replacement only when the active setting variant
// (which of the integration blocks is set) changes, not on every change within `setting` — a plain
// objectplanmodifier.RequiresReplace() on the whole attribute would also force replacement on an
// in-place credential rotation (e.g. a new pagerduty integration_key), which the API supports as
// a normal update.
type settingTypeRequiresReplaceModifier struct{}

func (m settingTypeRequiresReplaceModifier) Description(_ context.Context) string {
	return "Requires replacement if the integration type (which setting sub-block is set) changes."
}

func (m settingTypeRequiresReplaceModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m settingTypeRequiresReplaceModifier) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	// Nothing to compare on create (no prior state) or when the plan isn't fully known yet.
	if req.StateValue.IsNull() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if ActiveSettingType(req.StateValue) != ActiveSettingType(req.PlanValue) {
		resp.RequiresReplace = true
	}
}

// ActiveSettingType returns which of the mutually exclusive setting blocks is set, or "" if none is
// (an invalid config, already rejected by the ExactlyOneOf validator).
func ActiveSettingType(setting basetypes.ObjectValue) string {
	for name, value := range setting.Attributes() {
		if block, ok := value.(basetypes.ObjectValue); ok && !block.IsNull() && !block.IsUnknown() {
			return name
		}
	}
	return ""
}
