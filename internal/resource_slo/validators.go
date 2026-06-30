package resource_slo

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// exclusiveBetweenValidator enforces lower < value < upper. The framework's
// float64validator.Between is inclusive, so it cannot express the SLO API's exclusive
// bounds (e.g. 0 < target < 100) on its own.
type exclusiveBetweenValidator struct {
	lower, upper float64
}

// exclusiveBetween returns a Float64 validator that rejects values outside the open interval
// (lower, upper) — both endpoints are excluded.
func exclusiveBetween(lower, upper float64) validator.Float64 {
	return exclusiveBetweenValidator{lower: lower, upper: upper}
}

func (v exclusiveBetweenValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be greater than %v and less than %v", v.lower, v.upper)
}

func (v exclusiveBetweenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v exclusiveBetweenValidator) ValidateFloat64(_ context.Context, req validator.Float64Request, resp *validator.Float64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueFloat64()
	if value <= v.lower || value >= v.upper {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid value",
			fmt.Sprintf("value must be greater than %v and less than %v, got %v", v.lower, v.upper, value),
		)
	}
}
