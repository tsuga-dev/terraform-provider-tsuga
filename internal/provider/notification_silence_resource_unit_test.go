package provider

import (
	"context"
	"terraform-provider-tsuga/internal/resource_notification_silence"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateTimeWindow_DateTimes(t *testing.T) {
	p := path.Root("schedule").AtName("one_time")

	cases := []struct {
		name       string
		start, end string
		wantError  bool
	}{
		{"end after start", "2035-03-15T02:00:00", "2035-03-15T06:00:00", false},
		{"end on a later day", "2035-03-15T23:00:00", "2035-03-16T01:00:00", false},
		{"end equals start", "2035-03-15T02:00:00", "2035-03-15T02:00:00", true},
		{"end before start", "2035-03-15T06:00:00", "2035-03-15T02:00:00", true},
		// Rejected shapes: toISOString() output and other offset-bearing forms.
		{"milliseconds and Z suffix", "2026-08-04T16:22:11.123Z", "2026-08-05T16:22:11", true},
		{"Z suffix", "2026-08-04T16:22:11Z", "2026-08-05T16:22:11", true},
		{"UTC offset", "2026-08-04T16:22:11+02:00", "2026-08-05T16:22:11", true},
		{"space separator", "2026-08-04 16:22:11", "2026-08-05T16:22:11", true},
		{"missing seconds", "2026-08-04T16:22", "2026-08-05T16:22:11", true},
		{"date only", "2026-08-04", "2026-08-05T16:22:11", true},
		// Calendar-invalid values pass a shape check but not a real parse.
		{"day out of range", "2026-02-30T12:00:00", "2026-03-01T12:00:00", true},
		{"every component out of range", "2026-13-40T99:99:99", "2027-01-01T12:00:00", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateTimeWindow(types.StringValue(c.start), types.StringValue(c.end), simpleDateTimeLayout, p)
			if diags.HasError() != c.wantError {
				t.Fatalf("wantError=%v, got diagnostics: %v", c.wantError, diags)
			}
		})
	}
}

func TestValidateTimeWindow_Times(t *testing.T) {
	p := path.Root("schedule").AtName("recurring").AtName("monday").AtListIndex(0)

	cases := []struct {
		name       string
		start, end string
		wantError  bool
	}{
		{"end after start", "09:00:00", "17:00:00", false},
		{"full day", "00:00:00", "23:59:59", false},
		{"end equals start", "09:00:00", "09:00:00", true},
		{"end before start", "17:00:00", "09:00:00", true},
		{"hour out of range", "24:00:00", "23:00:00", true},
		{"minute out of range", "09:60:00", "17:00:00", true},
		{"not zero padded", "9:00:00", "17:00:00", true},
		// The API's simple-time format requires seconds.
		{"missing seconds", "09:00", "17:00", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := validateTimeWindow(types.StringValue(c.start), types.StringValue(c.end), simpleTimeLayout, p)
			if diags.HasError() != c.wantError {
				t.Fatalf("wantError=%v, got diagnostics: %v", c.wantError, diags)
			}
		})
	}
}

// Unknown bounds are only resolved at apply time, and a required attribute
// cannot be null, so neither is worth reporting here.
func TestValidateTimeWindow_SkipsNullAndUnknown(t *testing.T) {
	p := path.Root("schedule").AtName("one_time")

	if diags := validateTimeWindow(types.StringNull(), types.StringValue("17:00:00"), simpleTimeLayout, p); diags.HasError() {
		t.Fatalf("expected a null bound to be skipped, got: %v", diags)
	}
	if diags := validateTimeWindow(types.StringUnknown(), types.StringUnknown(), simpleTimeLayout, p); diags.HasError() {
		t.Fatalf("expected unknown bounds to be skipped, got: %v", diags)
	}
}

func TestValidateRecurringTimeOrder(t *testing.T) {
	ctx := context.Background()
	elemType := types.ObjectType{AttrTypes: resource_notification_silence.TimeRangeAttrTypes(ctx)}

	ranges := func(t *testing.T, start, end string) types.List {
		list, diags := types.ListValueFrom(ctx, elemType, []resource_notification_silence.TimeRangeModel{{
			StartTime: types.StringValue(start),
			EndTime:   types.StringValue(end),
		}})
		if diags.HasError() {
			t.Fatalf("failed to build time ranges: %v", diags)
		}
		return list
	}

	// A bad range on any day is reported, including the last one in the loop.
	recurring := &resource_notification_silence.RecurringScheduleModel{
		Monday:    ranges(t, "09:00:00", "17:00:00"),
		Tuesday:   types.ListNull(elemType),
		Wednesday: types.ListNull(elemType),
		Thursday:  types.ListNull(elemType),
		Friday:    types.ListNull(elemType),
		Saturday:  types.ListNull(elemType),
		Sunday:    ranges(t, "17:00:00", "09:00:00"),
	}
	if diags := validateRecurringTimeOrder(ctx, recurring); !diags.HasError() {
		t.Fatal("expected an error for a Sunday range ending before it starts")
	}

	recurring.Sunday = ranges(t, "09:00:00", "17:00:00")
	if diags := validateRecurringTimeOrder(ctx, recurring); diags.HasError() {
		t.Fatalf("unexpected diagnostics for valid ranges: %v", diags)
	}
}

// An unknown window (e.g. built from a computed value) must not fail decoding.
func TestValidateRecurringTimeOrder_UnknownElement(t *testing.T) {
	ctx := context.Background()
	elemType := types.ObjectType{AttrTypes: resource_notification_silence.TimeRangeAttrTypes(ctx)}

	list, diags := types.ListValue(elemType, []attr.Value{types.ObjectUnknown(elemType.AttrTypes)})
	if diags.HasError() {
		t.Fatalf("failed to build list: %v", diags)
	}

	recurring := &resource_notification_silence.RecurringScheduleModel{
		Monday:    list,
		Tuesday:   types.ListNull(elemType),
		Wednesday: types.ListNull(elemType),
		Thursday:  types.ListNull(elemType),
		Friday:    types.ListNull(elemType),
		Saturday:  types.ListNull(elemType),
		Sunday:    types.ListNull(elemType),
	}
	if diags := validateRecurringTimeOrder(ctx, recurring); diags.HasError() {
		t.Fatalf("expected an unknown window to be skipped, got: %v", diags)
	}
}
