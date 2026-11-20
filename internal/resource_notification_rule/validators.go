package resource_notification_rule

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ExactlyOneDestination checks that exactly one block of a target config is set.
func ExactlyOneDestination() validator.Object {
	return exactlyOneDestination{}
}

type exactlyOneDestination struct{}

func (exactlyOneDestination) Description(context.Context) string {
	return "Exactly one destination must be set."
}

func (v exactlyOneDestination) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v exactlyOneDestination) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var all, set []string
	for name, value := range req.ConfigValue.Attributes() {
		all = append(all, name)
		if value.IsUnknown() {
			return
		}
		if !value.IsNull() {
			set = append(set, name)
		}
	}
	if len(set) == 1 {
		return
	}
	slices.Sort(all)
	slices.Sort(set)

	setDescription := "none"
	if len(set) > 0 {
		setDescription = strings.Join(set, ", ")
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid target config",
		fmt.Sprintf("Exactly one destination must be set, one of: %s. Set: %s.", strings.Join(all, ", "), setDescription),
	)
}

// RenotifyConfigValid applies the field validators the generator gives the first renotify block to
// the others, which reuse its type (see tools/patch-provider-code-spec.js).
func RenotifyConfigValid() validator.Object {
	return renotifyConfigValid{}
}

type renotifyConfigValid struct{}

func (renotifyConfigValid) Description(context.Context) string {
	return "mode must be each, renotification_states alert or alert_no_data, renotify_interval_minutes at least 1."
}

func (v renotifyConfigValid) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (renotifyConfigValid) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	attrs := req.ConfigValue.Attributes()

	for _, name := range []string{"mode", "renotification_states", "renotify_interval_minutes"} {
		if attrs[name].IsNull() {
			resp.Diagnostics.AddAttributeError(req.Path.AtName(name), "Missing renotify attribute", name+" must be set.")
		}
	}

	mode := validator.StringResponse{}
	stringvalidator.OneOf("each").ValidateString(ctx, validator.StringRequest{
		Path: req.Path.AtName("mode"), ConfigValue: attrs["mode"].(types.String),
	}, &mode)
	states := validator.ListResponse{}
	listvalidator.ValueStringsAre(stringvalidator.OneOf("alert", "alert_no_data")).ValidateList(ctx, validator.ListRequest{
		Path: req.Path.AtName("renotification_states"), ConfigValue: attrs["renotification_states"].(types.List),
	}, &states)
	minutes := validator.Int64Response{}
	int64validator.AtLeast(1).ValidateInt64(ctx, validator.Int64Request{
		Path: req.Path.AtName("renotify_interval_minutes"), ConfigValue: attrs["renotify_interval_minutes"].(types.Int64),
	}, &minutes)

	resp.Diagnostics.Append(mode.Diagnostics...)
	resp.Diagnostics.Append(states.Diagnostics...)
	resp.Diagnostics.Append(minutes.Diagnostics...)
}
