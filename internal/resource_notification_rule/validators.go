package resource_notification_rule

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
