package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"terraform-provider-tsuga/internal/resource_notification_silence"
	"terraform-provider-tsuga/internal/teamsfilter"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// recurringWith builds a recurring block from the schedule's own attribute types, with the given
// days set (as already-built lists) and the others null.
func recurringWith(t *testing.T, days map[string]attr.Value) basetypes.ObjectValue {
	t.Helper()

	ctx := context.Background()
	recurringTypes := resource_notification_silence.ScheduleValue{}.AttributeTypes(ctx)["recurring"].(basetypes.ObjectType).AttrTypes
	weeklyTypes := recurringTypes["weekly_schedule"].(basetypes.ObjectType).AttrTypes
	values := map[string]attr.Value{}
	for _, day := range weekdays {
		if list, ok := days[day]; ok {
			values[day] = list
		} else {
			values[day] = types.ListNull(weeklyTypes[day].(basetypes.ListType).ElemType)
		}
	}
	return types.ObjectValueMust(recurringTypes, map[string]attr.Value{
		"time_zone":       types.StringNull(),
		"weekly_schedule": types.ObjectValueMust(weeklyTypes, values),
	})
}

// ranges builds a day's list with one window; an empty start makes it an unknown window.
func ranges(t *testing.T, day, start, end string) attr.Value {
	t.Helper()

	ctx := context.Background()
	elemType := resource_notification_silence.ScheduleValue{}.AttributeTypes(ctx)["recurring"].(basetypes.ObjectType).AttrTypes["weekly_schedule"].(basetypes.ObjectType).AttrTypes[day].(basetypes.ListType).ElemType
	rangeTypes := elemType.(attr.TypeWithAttributeTypes).AttributeTypes()

	var element attr.Value
	if start == "" {
		var err error
		element, err = elemType.ValueFromTerraform(ctx, tftypes.NewValue(elemType.TerraformType(ctx), tftypes.UnknownValue))
		if err != nil {
			t.Fatalf("failed to build an unknown time range: %v", err)
		}
	} else {
		var diags diag.Diagnostics
		object := types.ObjectValueMust(rangeTypes, map[string]attr.Value{"start_time": types.StringValue(start), "end_time": types.StringValue(end)})
		element, diags = elemType.(basetypes.ObjectTypable).ValueFromObject(ctx, object)
		if diags.HasError() {
			t.Fatalf("failed to build a time range: %v", diags)
		}
	}
	list, diags := types.ListValue(elemType, []attr.Value{element})
	if diags.HasError() {
		t.Fatalf("failed to build time ranges: %v", diags)
	}
	return list
}

func TestValidateRecurringTimeOrder(t *testing.T) {
	ctx := context.Background()

	// A bad range on any day is reported, including the last one in the loop.
	recurring := recurringWith(t, map[string]attr.Value{
		"monday": ranges(t, "monday", "09:00:00", "17:00:00"),
		"sunday": ranges(t, "sunday", "17:00:00", "09:00:00"),
	})
	if diags := validateRecurringTimeOrder(ctx, recurring); !diags.HasError() {
		t.Fatal("expected an error for a Sunday range ending before it starts")
	}

	recurring = recurringWith(t, map[string]attr.Value{
		"monday": ranges(t, "monday", "09:00:00", "17:00:00"),
		"sunday": ranges(t, "sunday", "09:00:00", "17:00:00"),
	})
	if diags := validateRecurringTimeOrder(ctx, recurring); diags.HasError() {
		t.Fatalf("unexpected diagnostics for valid ranges: %v", diags)
	}
}

// An unknown window (e.g. built from a computed value) must not fail decoding.
func TestValidateRecurringTimeOrder_UnknownElement(t *testing.T) {
	recurring := recurringWith(t, map[string]attr.Value{"monday": ranges(t, "monday", "", "")})
	if diags := validateRecurringTimeOrder(context.Background(), recurring); diags.HasError() {
		t.Fatalf("expected an unknown window to be skipped, got: %v", diags)
	}
}

func TestExpandFlattenSchedule_RoundTrips(t *testing.T) {
	ctx := context.Background()

	cases := map[string]notificationSilenceSchedule{
		"one-time": {Type: "one-time", StartTime: "2035-03-15T02:00:00", EndTime: "2035-03-15T06:00:00", TimeZone: "America/New_York"},
		"recurring": {Type: "recurring", TimeZone: "UTC", WeeklySchedule: map[string][]timeRangeAPIData{
			"monday": {{StartTime: "00:00:00", EndTime: "06:00:00"}, {StartTime: "12:00:00", EndTime: "14:00:00"}},
			"sunday": {{StartTime: "02:00:00", EndTime: "06:00:00"}},
		}},
	}
	for name, schedule := range cases {
		t.Run(name, func(t *testing.T) {
			flattened, diags := flattenSchedule(ctx, schedule)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			expanded, diags := expandSchedule(ctx, flattened)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			raw, _ := json.Marshal(expanded)
			var back notificationSilenceSchedule
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatalf("failed to decode the expanded schedule %s: %v", raw, err)
			}
			if !reflect.DeepEqual(back, schedule) {
				t.Fatalf("got %+v, want %+v", back, schedule)
			}
		})
	}
}

func TestFlattenNotificationSilence_FitsTheGeneratedSchema(t *testing.T) {
	ctx := context.Background()

	state, diags := flattenNotificationSilence(ctx, notificationSilenceAPIData{
		ID:       "silence-1",
		Name:     "silence",
		Reason:   "maintenance",
		Owner:    "team-1",
		IsActive: true,
		Schedule: notificationSilenceSchedule{Type: "recurring", WeeklySchedule: map[string][]timeRangeAPIData{
			"friday": {{StartTime: "18:00:00", EndTime: "23:59:00"}},
		}},
		TeamsFilter:           teamsfilter.APITeamsFilter{Type: "specific-teams", Teams: []string{"team-1"}},
		PrioritiesFilter:      []int64{1},
		TransitionTypesFilter: []string{"triggered"},
		Tags:                  []apiTag{{Key: "k", Value: "v"}},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	got := tfsdk.State{Schema: resource_notification_silence.NotificationSilenceResourceSchema(ctx)}
	if d := got.Set(ctx, &state); d.HasError() {
		t.Fatalf("flattened silence does not fit the schema: %v", d)
	}
}
