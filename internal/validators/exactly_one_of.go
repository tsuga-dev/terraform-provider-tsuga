package validators

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// ExactlyOneOf checks that exactly one attribute of an object is set. The error reads
// "Exactly one <noun> must be set, one of: ... Set: ...." under the given summary.
func ExactlyOneOf(summary, noun string) validator.Object {
	return exactlyOneOf{summary: summary, noun: noun}
}

type exactlyOneOf struct {
	summary string
	noun    string
}

func (v exactlyOneOf) Description(context.Context) string {
	return fmt.Sprintf("Exactly one %s must be set.", v.noun)
}

func (v exactlyOneOf) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v exactlyOneOf) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
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
		v.summary,
		fmt.Sprintf("Exactly one %s must be set, one of: %s. Set: %s.", v.noun, strings.Join(all, ", "), setDescription),
	)
}
