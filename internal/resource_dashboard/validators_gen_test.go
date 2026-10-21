package resource_dashboard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestTimePresetValidatorsAllowPast15Days guards against the generator or the
// spec silently dropping a time_preset value: "past-15-days" is a real API
// value that the hand-maintained schema once omitted (see CHANGELOG).
func TestTimePresetValidatorsAllowPast15Days(t *testing.T) {
	ctx := context.Background()
	req := validator.StringRequest{ConfigValue: types.StringValue("past-15-days")}
	for _, v := range timePresetValidators() {
		var resp validator.StringResponse
		v.ValidateString(ctx, req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected \"past-15-days\" to be a valid time_preset value: %v", resp.Diagnostics)
		}
	}
}

// TestSourceValidatorsAllowRum guards the generated source enum against
// losing the "rum" value the spec added.
func TestSourceValidatorsAllowRum(t *testing.T) {
	ctx := context.Background()
	req := validator.StringRequest{ConfigValue: types.StringValue("rum")}
	for _, v := range sourceValidators() {
		var resp validator.StringResponse
		v.ValidateString(ctx, req, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("expected \"rum\" to be a valid source value: %v", resp.Diagnostics)
		}
	}
}
