package resource_notification_integration

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"terraform-provider-tsuga/internal/resource_team"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NotificationIntegrationResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Notification destinations (PagerDuty, webhook, Jira, etc.) used as targets by notification rules. Slack is not supported here because it is set up through an OAuth install flow in the Tsuga app.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier of the integration",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the integration",
			},
			"tags": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of key/value tags applied to the resource",
				Validators: []validator.List{
					listvalidator.SizeAtMost(50),
				},
				NestedObject: schema.NestedAttributeObject{
					CustomType: resource_team.TagsType{
						ObjectType: types.ObjectType{
							AttrTypes: resource_team.TagsValue{}.AttributeTypes(ctx),
						},
					},
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(128),
							},
						},
						"value": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(256),
							},
						},
					},
				},
			},
			"secrets_version": schema.StringAttribute{
				Optional:    true,
				Description: "Secrets are write-only and never stored in state, so Terraform cannot detect when one changes. Set this to any new value after rotating a secret to have it sent again.",
			},
			"setting": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Destination configuration. Exactly one of google_chat, grafana_irm, incident_io, jira, microsoft_teams, pagerduty, servicenow, squadcast, or webhook must be set. The integration type cannot be changed after creation; changing which one is set replaces the resource.",
				PlanModifiers: []planmodifier.Object{
					settingTypeRequiresReplaceModifier{},
				},
				Attributes: map[string]schema.Attribute{
					"google_chat": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"webhook_url": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Google Chat space webhook URL",
							},
						},
					},
					"grafana_irm": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"webhook_url": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Grafana IRM integration webhook URL",
							},
						},
					},
					"incident_io": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"alert_source_config_id": schema.StringAttribute{
								Required:    true,
								Description: "incident.io alert source config ID",
							},
							"token": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "incident.io alert source token",
							},
						},
					},
					"jira": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"site_url": schema.StringAttribute{
								Required:    true,
								Description: "Base URL of the Jira Cloud site, like \"https://your-company.atlassian.net\"",
							},
							"email": schema.StringAttribute{
								Required:    true,
								Description: "Atlassian account email used together with the API token for Basic authentication",
							},
							"api_token": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Atlassian API token minted for the account",
							},
						},
					},
					"microsoft_teams": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"webhook_url": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Microsoft Teams incoming webhook URL",
							},
						},
					},
					"pagerduty": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"integration_key": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "PagerDuty Events API v2 integration key",
							},
						},
					},
					"servicenow": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"instance_name": schema.StringAttribute{
								Required:    true,
								Description: "Instance name of the ServiceNow instance, i.e. the subdomain part of the instance URL, like \"my-instance\" in \"my-instance.service-now.com\"",
							},
							"username": schema.StringAttribute{
								Required:    true,
								Description: "ServiceNow username",
							},
							"password": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "ServiceNow password",
							},
						},
					},
					"squadcast": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"webhook_url": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Squadcast webhook URL",
							},
						},
					},
					"webhook": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								Required:    true,
								Sensitive:   true,
								WriteOnly:   true,
								Description: "Destination URL",
							},
							"method": schema.StringAttribute{
								Required:    true,
								Description: "HTTP method used to call the URL",
								Validators: []validator.String{
									stringvalidator.OneOf("POST", "PUT"),
								},
							},
							"authentication": schema.SingleNestedAttribute{
								Required:    true,
								Description: "Authentication applied to the webhook request. Exactly one of bearer, basic, custom_header, or none must be set.",
								Attributes: map[string]schema.Attribute{
									"none": schema.SingleNestedAttribute{
										Optional:    true,
										Description: "Send the request unauthenticated. Set as an empty object: `none = {}`.",
										Attributes:  map[string]schema.Attribute{},
									},
									"bearer": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"token": schema.StringAttribute{
												Required:    true,
												Sensitive:   true,
												WriteOnly:   true,
												Description: "Bearer token",
											},
										},
									},
									"basic": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"username": schema.StringAttribute{
												Required:    true,
												Description: "Basic auth username",
											},
											"password": schema.StringAttribute{
												Required:    true,
												Sensitive:   true,
												WriteOnly:   true,
												Description: "Basic auth password",
											},
										},
									},
									"custom_header": schema.SingleNestedAttribute{
										Optional: true,
										Attributes: map[string]schema.Attribute{
											"key": schema.StringAttribute{
												Required:    true,
												Description: "Header name",
											},
											"value": schema.StringAttribute{
												Required:    true,
												Sensitive:   true,
												WriteOnly:   true,
												Description: "Header value",
											},
										},
									},
								},
							},
							"custom_headers": schema.MapAttribute{
								Optional:    true,
								ElementType: types.StringType,
								Description: "Additional custom headers sent with every request",
							},
							"payload_template": schema.SingleNestedAttribute{
								Required:    true,
								Description: "Body template sent to the webhook. `type` is json or form; `value` is the template body encoded as a JSON string (a JSON object for `json`, a flat object of strings/numbers/booleans/arrays for `form`).",
								Attributes: map[string]schema.Attribute{
									"type": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf("json", "form"),
										},
									},
									"value": schema.StringAttribute{
										Required:    true,
										Description: "Template body encoded as a JSON string",
									},
								},
							},
							"event_name_mapping": schema.SingleNestedAttribute{
								Optional:    true,
								Description: "Overrides for the event name Tsuga substitutes into the payload template",
								Attributes: map[string]schema.Attribute{
									"ok": schema.StringAttribute{
										Required: true,
									},
									"alert": schema.StringAttribute{
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

type NotificationIntegrationModel struct {
	Id             types.String             `tfsdk:"id"`
	Name           types.String             `tfsdk:"name"`
	Tags           types.List               `tfsdk:"tags"`
	SecretsVersion types.String             `tfsdk:"secrets_version"`
	Setting        *IntegrationSettingModel `tfsdk:"setting"`
}

type IntegrationSettingModel struct {
	GoogleChat     *WebhookUrlSettingModel `tfsdk:"google_chat"`
	GrafanaIrm     *WebhookUrlSettingModel `tfsdk:"grafana_irm"`
	IncidentIo     *IncidentIoSettingModel `tfsdk:"incident_io"`
	Jira           *JiraSettingModel       `tfsdk:"jira"`
	MicrosoftTeams *WebhookUrlSettingModel `tfsdk:"microsoft_teams"`
	Pagerduty      *PagerdutySettingModel  `tfsdk:"pagerduty"`
	Servicenow     *ServicenowSettingModel `tfsdk:"servicenow"`
	Squadcast      *WebhookUrlSettingModel `tfsdk:"squadcast"`
	Webhook        *WebhookSettingModel    `tfsdk:"webhook"`
}

type WebhookUrlSettingModel struct {
	WebhookUrl types.String `tfsdk:"webhook_url"`
}

type IncidentIoSettingModel struct {
	AlertSourceConfigId types.String `tfsdk:"alert_source_config_id"`
	Token               types.String `tfsdk:"token"`
}

type JiraSettingModel struct {
	SiteUrl  types.String `tfsdk:"site_url"`
	Email    types.String `tfsdk:"email"`
	ApiToken types.String `tfsdk:"api_token"`
}

type PagerdutySettingModel struct {
	IntegrationKey types.String `tfsdk:"integration_key"`
}

type ServicenowSettingModel struct {
	InstanceName types.String `tfsdk:"instance_name"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
}

type WebhookSettingModel struct {
	Url              types.String                  `tfsdk:"url"`
	Method           types.String                  `tfsdk:"method"`
	Authentication   *WebhookAuthenticationModel   `tfsdk:"authentication"`
	CustomHeaders    types.Map                     `tfsdk:"custom_headers"`
	PayloadTemplate  *WebhookPayloadTemplateModel  `tfsdk:"payload_template"`
	EventNameMapping *WebhookEventNameMappingModel `tfsdk:"event_name_mapping"`
}

type WebhookAuthenticationModel struct {
	Bearer       *WebhookBearerAuthModel       `tfsdk:"bearer"`
	Basic        *WebhookBasicAuthModel        `tfsdk:"basic"`
	CustomHeader *WebhookCustomHeaderAuthModel `tfsdk:"custom_header"`
	None         *WebhookNoneAuthModel         `tfsdk:"none"`
}

type WebhookNoneAuthModel struct{}

type WebhookBearerAuthModel struct {
	Token types.String `tfsdk:"token"`
}

type WebhookBasicAuthModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

type WebhookCustomHeaderAuthModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type WebhookPayloadTemplateModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type WebhookEventNameMappingModel struct {
	Ok    types.String `tfsdk:"ok"`
	Alert types.String `tfsdk:"alert"`
}

// settingTypeRequiresReplaceModifier forces replacement only when the active setting variant
// (which of the 9 sub-blocks is set) changes, not on every change within `setting` — a plain
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

func (m settingTypeRequiresReplaceModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	// Nothing to compare on create (no prior state) or when the plan isn't fully known yet.
	if req.StateValue.IsNull() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	var state, plan IntegrationSettingModel
	resp.Diagnostics.Append(req.StateValue.As(ctx, &state, basetypes.ObjectAsOptions{})...)
	resp.Diagnostics.Append(req.PlanValue.As(ctx, &plan, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	if activeSettingType(state) != activeSettingType(plan) {
		resp.RequiresReplace = true
	}
}

// activeSettingType returns which of the 9 mutually exclusive setting sub-blocks is set, or ""
// if none are (an invalid config, already rejected by ValidateConfig before plan modification
// would see it in practice).
func activeSettingType(setting IntegrationSettingModel) string {
	switch {
	case setting.GoogleChat != nil:
		return "google_chat"
	case setting.GrafanaIrm != nil:
		return "grafana_irm"
	case setting.IncidentIo != nil:
		return "incident_io"
	case setting.Jira != nil:
		return "jira"
	case setting.MicrosoftTeams != nil:
		return "microsoft_teams"
	case setting.Pagerduty != nil:
		return "pagerduty"
	case setting.Servicenow != nil:
		return "servicenow"
	case setting.Squadcast != nil:
		return "squadcast"
	case setting.Webhook != nil:
		return "webhook"
	default:
		return ""
	}
}
