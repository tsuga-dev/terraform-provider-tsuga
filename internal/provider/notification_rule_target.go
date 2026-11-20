package provider

import (
	"context"
	"fmt"

	"terraform-provider-tsuga/internal/apiclient"
	"terraform-provider-tsuga/internal/resource_notification_rule"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Mapping between the generated API types (internal/apiclient) and the generated Terraform values
// (internal/resource_notification_rule). Both sides are written as struct literals, so a field the
// schema or the API gains, loses or renames fails to compile here.

// Method sets of the discriminated unions of the request bodies (notification rule create and
// update, adaptive rule create and update) and of the responses (notification rule, adaptive rule).
type inputConfig interface {
	FromRuleTargetInputEmail(apiclient.RuleTargetInputEmail) error
	FromRuleTargetInputGoogleChat(apiclient.RuleTargetInputGoogleChat) error
	FromRuleTargetInputGrafanaIrm(apiclient.RuleTargetInputGrafanaIrm) error
	FromRuleTargetInputIncidentIo(apiclient.RuleTargetInputIncidentIo) error
	FromRuleTargetInputJira(apiclient.RuleTargetInputJira) error
	FromRuleTargetInputMicrosoftTeams(apiclient.RuleTargetInputMicrosoftTeams) error
	FromRuleTargetInputPagerDuty(apiclient.RuleTargetInputPagerDuty) error
	FromRuleTargetInputServiceNow(apiclient.RuleTargetInputServiceNow) error
	FromRuleTargetInputSlack(apiclient.RuleTargetInputSlack) error
	FromRuleTargetInputSquadcast(apiclient.RuleTargetInputSquadcast) error
	FromRuleTargetInputWebhook(apiclient.RuleTargetInputWebhook) error
}

type outputConfig interface {
	ValueByDiscriminator() (any, error)
}

// Every request and response spells the rate limit with the same fields and tags.
type apiRateLimit = struct {
	MaxMessages int `json:"maxMessages"`
	Minutes     int `json:"minutes"`
}

// Tags are spelled the same in every response.
func tagsFromAPI(tags *[]struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}) []apiTag {
	if tags == nil {
		return nil
	}
	out := make([]apiTag, len(*tags))
	for i, t := range *tags {
		out[i] = apiTag{Key: t.Key, Value: t.Value}
	}
	return out
}

// The generated Terraform values report null whatever their fields hold, and the object conversion
// reads the fields by their tfsdk tags: these method-less copies convert to objects field by field.
type (
	configFields    resource_notification_rule.ConfigValue
	targetFields    resource_notification_rule.TargetsValue
	rateLimitFields resource_notification_rule.RateLimitValue
	renotifyFields  resource_notification_rule.RenotifyValue

	emailFields          resource_notification_rule.EmailValue
	googleChatFields     resource_notification_rule.GoogleChatValue
	grafanaIrmFields     resource_notification_rule.GrafanaIrmValue
	incidentIoFields     resource_notification_rule.IncidentIoValue
	jiraFields           resource_notification_rule.JiraValue
	microsoftTeamsFields resource_notification_rule.MicrosoftTeamsValue
	pagerdutyFields      resource_notification_rule.PagerdutyValue
	servicenowFields     resource_notification_rule.ServicenowValue
	slackFields          resource_notification_rule.SlackValue
	squadcastFields      resource_notification_rule.SquadcastValue
	webhookFields        resource_notification_rule.WebhookValue
)

func grow[T any](dst *[]T) *T {
	var zero T
	*dst = append(*dst, zero)
	return &(*dst)[len(*dst)-1]
}

func convertStrings[To, From ~string](in []From) []To {
	out := make([]To, len(in))
	for i, s := range in {
		out[i] = To(s)
	}
	return out
}

func known(o basetypes.ObjectValue) bool {
	return !o.IsNull() && !o.IsUnknown()
}

func valueOf[V any](o basetypes.ObjectValue, ctor func(map[string]attr.Type, map[string]attr.Value) (V, diag.Diagnostics)) (V, diag.Diagnostics) {
	return ctor(o.AttributeTypes(context.Background()), o.Attributes())
}

func expandRenotify(ctx context.Context, o basetypes.ObjectValue, diags *diag.Diagnostics) *apiclient.InputRenotifyConfig {
	if !known(o) {
		return nil
	}
	value, d := valueOf(o, resource_notification_rule.NewRenotifyValue)
	diags.Append(d...)
	var states []string
	diags.Append(value.RenotificationStates.ElementsAs(ctx, &states, false)...)
	return &apiclient.InputRenotifyConfig{
		Mode:                    apiclient.InputRenotifyConfigMode(value.Mode.ValueString()),
		RenotificationStates:    convertStrings[apiclient.InputRenotifyConfigRenotificationStates](states),
		RenotifyIntervalMinutes: int(value.RenotifyIntervalMinutes.ValueInt64()),
	}
}

func renotifyObject(ctx context.Context, rc *apiclient.RenotifyConfig) basetypes.ObjectValue {
	attrTypes := resource_notification_rule.RenotifyValue{}.AttributeTypes(ctx)
	if rc == nil {
		return types.ObjectNull(attrTypes)
	}
	states, _ := types.ListValueFrom(ctx, types.StringType, stringsOf(rc.RenotificationStates))
	object, _ := types.ObjectValueFrom(ctx, attrTypes, renotifyFields{
		Mode:                    types.StringValue(string(rc.Mode)),
		RenotificationStates:    states,
		RenotifyIntervalMinutes: types.Int64Value(int64(rc.RenotifyIntervalMinutes)),
	})
	return object
}

// expandTargetConfig writes the one destination set in cfg, with its renotification, into the union.
// The computed attributes (type, integration_name) are not read: the API types have no field for
// the name, and the type comes from the destination.
func expandTargetConfig(ctx context.Context, cfg basetypes.ObjectValue, dst inputConfig) diag.Diagnostics {
	config, diags := valueOf(cfg, resource_notification_rule.NewConfigValue)
	if diags.HasError() {
		return diags
	}

	destinations := []struct {
		name  string
		block basetypes.ObjectValue
		set   func(basetypes.ObjectValue) error
	}{
		{"email", config.Email, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewEmailValue)
			diags.Append(d...)
			var addresses []string
			diags.Append(value.Addresses.ElementsAs(ctx, &addresses, false)...)
			v := apiclient.RuleTargetInputEmail{Addresses: addresses, Type: apiclient.RuleTargetInputEmailTypeEmail}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputEmail(v)
		}},
		{"google_chat", config.GoogleChat, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewGoogleChatValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputGoogleChat{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputGoogleChatTypeGoogleChat}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputGoogleChat(v)
		}},
		{"grafana_irm", config.GrafanaIrm, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewGrafanaIrmValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputGrafanaIrm{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputGrafanaIrmTypeGrafanaIrm}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputGrafanaIrm(v)
		}},
		{"incident_io", config.IncidentIo, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewIncidentIoValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputIncidentIo{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputIncidentIoTypeIncidentIo}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputIncidentIo(v)
		}},
		// Jira has no renotification.
		{"jira", config.Jira, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewJiraValue)
			diags.Append(d...)
			return dst.FromRuleTargetInputJira(apiclient.RuleTargetInputJira{
				ClosedStatus:  value.ClosedStatus.ValueStringPointer(),
				IntegrationId: value.IntegrationId.ValueString(),
				IssueType:     value.IssueType.ValueString(),
				OpenStatus:    value.OpenStatus.ValueStringPointer(),
				ProjectKey:    value.ProjectKey.ValueString(),
				Type:          apiclient.RuleTargetInputJiraTypeJira,
			})
		}},
		{"microsoft_teams", config.MicrosoftTeams, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewMicrosoftTeamsValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputMicrosoftTeams{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputMicrosoftTeamsTypeMicrosoftTeams}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputMicrosoftTeams(v)
		}},
		{"pagerduty", config.Pagerduty, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewPagerdutyValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputPagerDuty{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputPagerDutyTypePagerduty}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputPagerDuty(v)
		}},
		{"servicenow", config.Servicenow, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewServicenowValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputServiceNow{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputServiceNowTypeServicenow}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputServiceNow(v)
		}},
		{"slack", config.Slack, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewSlackValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputSlack{Channel: value.Channel.ValueString(), IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputSlackTypeSlack}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputSlack(v)
		}},
		{"squadcast", config.Squadcast, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewSquadcastValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputSquadcast{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputSquadcastTypeSquadcast}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputSquadcast(v)
		}},
		{"webhook", config.Webhook, func(o basetypes.ObjectValue) error {
			value, d := valueOf(o, resource_notification_rule.NewWebhookValue)
			diags.Append(d...)
			v := apiclient.RuleTargetInputWebhook{IntegrationId: value.IntegrationId.ValueString(), Type: apiclient.RuleTargetInputWebhookTypeWebhook}
			v.RenotifyConfig = expandRenotify(ctx, value.Renotify, &diags)
			return dst.FromRuleTargetInputWebhook(v)
		}},
	}

	var set []string
	for _, d := range destinations {
		if known(d.block) {
			set = append(set, d.name)
		}
	}
	if len(set) != 1 {
		diags.AddError("Invalid target config", fmt.Sprintf("Exactly one destination must be set in config, got %d.", len(set)))
		return diags
	}
	for _, d := range destinations {
		if known(d.block) {
			if err := d.set(d.block); err != nil {
				diags.AddError("Invalid target config", err.Error())
			}
		}
	}
	return diags
}

// expandTarget writes one target of the plan into the id, config and rate limit of a request.
func expandTarget(ctx context.Context, id types.String, config, rateLimit basetypes.ObjectValue, dstID *string, dstConfig inputConfig, dstRateLimit **apiRateLimit) diag.Diagnostics {
	*dstID = id.ValueString()
	diags := expandTargetConfig(ctx, config, dstConfig)
	if known(rateLimit) {
		value, d := valueOf(rateLimit, resource_notification_rule.NewRateLimitValue)
		diags.Append(d...)
		*dstRateLimit = &apiRateLimit{MaxMessages: int(value.MaxMessages.ValueInt64()), Minutes: int(value.Minutes.ValueInt64())}
	}
	return diags
}

// expandNotificationRuleTargets writes the targets of the plan into the request body.
func expandNotificationRuleTargets(ctx context.Context, targets types.List, body *apiclient.CreateNotificationRuleJSONBody) diag.Diagnostics {
	var diags diag.Diagnostics
	if targets.IsNull() || targets.IsUnknown() {
		return diags
	}

	var values []resource_notification_rule.TargetsValue
	diags.Append(targets.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return diags
	}

	for _, v := range values {
		t := grow(&body.Targets)
		diags.Append(expandTarget(ctx, v.Id, v.Config, v.RateLimit, &t.Id, &t.Config, &t.RateLimit)...)
	}
	return diags
}

func stringsOf[T ~string](in []T) []string {
	return convertStrings[string](in)
}

func nullConfig(ctx context.Context) configFields {
	null := func(attrTypes map[string]attr.Type) basetypes.ObjectValue { return types.ObjectNull(attrTypes) }
	return configFields{
		Email:          null(resource_notification_rule.EmailValue{}.AttributeTypes(ctx)),
		GoogleChat:     null(resource_notification_rule.GoogleChatValue{}.AttributeTypes(ctx)),
		GrafanaIrm:     null(resource_notification_rule.GrafanaIrmValue{}.AttributeTypes(ctx)),
		IncidentIo:     null(resource_notification_rule.IncidentIoValue{}.AttributeTypes(ctx)),
		Jira:           null(resource_notification_rule.JiraValue{}.AttributeTypes(ctx)),
		MicrosoftTeams: null(resource_notification_rule.MicrosoftTeamsValue{}.AttributeTypes(ctx)),
		Pagerduty:      null(resource_notification_rule.PagerdutyValue{}.AttributeTypes(ctx)),
		Servicenow:     null(resource_notification_rule.ServicenowValue{}.AttributeTypes(ctx)),
		Slack:          null(resource_notification_rule.SlackValue{}.AttributeTypes(ctx)),
		Squadcast:      null(resource_notification_rule.SquadcastValue{}.AttributeTypes(ctx)),
		Webhook:        null(resource_notification_rule.WebhookValue{}.AttributeTypes(ctx)),
	}
}

// flattenTargetConfig reads the destination of a response into its block of the config, and its
// renotification into its renotify block. An unsupported destination is a diagnostic.
func flattenTargetConfig(ctx context.Context, src outputConfig) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	configTypes := resource_notification_rule.ConfigValue{}.AttributeTypes(ctx)

	value, err := src.ValueByDiscriminator()
	if err != nil {
		diags.AddError("Unsupported notification destination", err.Error())
		return types.ObjectNull(configTypes), diags
	}

	config := nullConfig(ctx)
	block := func(attrTypes map[string]attr.Type, fields any) basetypes.ObjectValue {
		object, d := types.ObjectValueFrom(ctx, attrTypes, fields)
		diags.Append(d...)
		return object
	}

	switch v := value.(type) {
	case apiclient.RuleTargetConfigEmail:
		addresses, d := types.ListValueFrom(ctx, types.StringType, v.Addresses)
		diags.Append(d...)
		config.Email = block(resource_notification_rule.EmailValue{}.AttributeTypes(ctx), emailFields{Addresses: addresses, EmailType: types.StringValue(string(v.Type)), Renotify: renotifyObject(ctx, v.RenotifyConfig)})
	case apiclient.RuleTargetConfigGoogleChat:
		config.GoogleChat = block(resource_notification_rule.GoogleChatValue{}.AttributeTypes(ctx), googleChatFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			GoogleChatType:  types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigGrafanaIrm:
		config.GrafanaIrm = block(resource_notification_rule.GrafanaIrmValue{}.AttributeTypes(ctx), grafanaIrmFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			GrafanaIrmType:  types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigIncidentIo:
		config.IncidentIo = block(resource_notification_rule.IncidentIoValue{}.AttributeTypes(ctx), incidentIoFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			IncidentIoType:  types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigJira:
		config.Jira = block(resource_notification_rule.JiraValue{}.AttributeTypes(ctx), jiraFields{
			ClosedStatus:    types.StringPointerValue(v.ClosedStatus),
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			IssueType:       types.StringValue(v.IssueType),
			OpenStatus:      types.StringPointerValue(v.OpenStatus),
			ProjectKey:      types.StringValue(v.ProjectKey),
			JiraType:        types.StringValue(string(v.Type)),
		})
	case apiclient.RuleTargetConfigMicrosoftTeams:
		config.MicrosoftTeams = block(resource_notification_rule.MicrosoftTeamsValue{}.AttributeTypes(ctx), microsoftTeamsFields{
			IntegrationId:      types.StringValue(v.IntegrationId),
			IntegrationName:    types.StringValue(v.IntegrationName),
			MicrosoftTeamsType: types.StringValue(string(v.Type)),
			Renotify:           renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigPagerDuty:
		config.Pagerduty = block(resource_notification_rule.PagerdutyValue{}.AttributeTypes(ctx), pagerdutyFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			PagerdutyType:   types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigServiceNow:
		config.Servicenow = block(resource_notification_rule.ServicenowValue{}.AttributeTypes(ctx), servicenowFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			ServicenowType:  types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigSlack:
		config.Slack = block(resource_notification_rule.SlackValue{}.AttributeTypes(ctx), slackFields{
			Channel:         types.StringValue(v.Channel),
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			SlackType:       types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigSquadcast:
		config.Squadcast = block(resource_notification_rule.SquadcastValue{}.AttributeTypes(ctx), squadcastFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			SquadcastType:   types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	case apiclient.RuleTargetConfigWebhook:
		config.Webhook = block(resource_notification_rule.WebhookValue{}.AttributeTypes(ctx), webhookFields{
			IntegrationId:   types.StringValue(v.IntegrationId),
			IntegrationName: types.StringValue(v.IntegrationName),
			WebhookType:     types.StringValue(string(v.Type)),
			Renotify:        renotifyObject(ctx, v.RenotifyConfig),
		})
	default:
		diags.AddError("Unsupported notification destination", fmt.Sprintf("Unexpected destination type %T.", value))
	}

	return block(configTypes, config), diags
}

// flattenNotificationRuleTarget returns one target as an object of id, config and rate_limit.
func flattenNotificationRuleTarget(ctx context.Context, id string, config outputConfig, rateLimit *apiRateLimit) (basetypes.ObjectValue, diag.Diagnostics) {
	configObject, diags := flattenTargetConfig(ctx, config)

	rateLimitTypes := resource_notification_rule.RateLimitValue{}.AttributeTypes(ctx)
	rateLimitObject := types.ObjectNull(rateLimitTypes)
	if rateLimit != nil {
		object, d := types.ObjectValueFrom(ctx, rateLimitTypes, rateLimitFields{
			MaxMessages: types.Int64Value(int64(rateLimit.MaxMessages)),
			Minutes:     types.Int64Value(int64(rateLimit.Minutes)),
		})
		diags.Append(d...)
		rateLimitObject = object
	}
	if diags.HasError() {
		return types.ObjectNull(resource_notification_rule.TargetsValue{}.AttributeTypes(ctx)), diags
	}

	object, d := types.ObjectValueFrom(ctx, resource_notification_rule.TargetsValue{}.AttributeTypes(ctx), targetFields{
		Id:        types.StringValue(id),
		Config:    configObject,
		RateLimit: rateLimitObject,
	})
	diags.Append(d...)
	return object, diags
}

func flattenNotificationRuleTargets(ctx context.Context, rule apiclient.Rule) (types.List, diag.Diagnostics) {
	targetAttrTypes := resource_notification_rule.TargetsValue{}.AttributeTypes(ctx)
	elemType := resource_notification_rule.TargetsType{ObjectType: types.ObjectType{AttrTypes: targetAttrTypes}}
	if len(rule.Targets) == 0 {
		return types.ListNull(elemType), nil
	}

	var diags diag.Diagnostics
	values := make([]attr.Value, 0, len(rule.Targets))
	for _, t := range rule.Targets {
		object, objectDiags := flattenNotificationRuleTarget(ctx, t.Id, t.Config, t.RateLimit)
		diags.Append(objectDiags...)
		if diags.HasError() {
			return types.ListNull(elemType), diags
		}
		value, valueDiags := resource_notification_rule.NewTargetsValue(targetAttrTypes, object.Attributes())
		diags.Append(valueDiags...)
		values = append(values, value)
	}
	if diags.HasError() {
		return types.ListNull(elemType), diags
	}

	list, listDiags := types.ListValue(elemType, values)
	diags.Append(listDiags...)
	return list, diags
}
