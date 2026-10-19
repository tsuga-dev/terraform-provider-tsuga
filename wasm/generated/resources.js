// Generated from the Terraform provider resource registry.
export const resourceTypes = Object.freeze(["tsuga_adaptive_notification_rule","tsuga_cloud_account","tsuga_custom_usage_tag","tsuga_dashboard","tsuga_dashboard_folder","tsuga_ingestion_api_key","tsuga_metric_route","tsuga_monitor","tsuga_monitor_folder","tsuga_notification_integration","tsuga_notification_rule","tsuga_notification_silence","tsuga_operation_api_key","tsuga_retention_policy","tsuga_route","tsuga_rum_public_token","tsuga_slo","tsuga_tag_policy","tsuga_team","tsuga_team_membership"]);
export function bindResources(exportResource) {
  return {
    exportAdaptiveNotificationRule: (resource, options) => exportResource("tsuga_adaptive_notification_rule", resource, options),
    exportCloudAccount: (resource, options) => exportResource("tsuga_cloud_account", resource, options),
    exportCustomUsageTag: (resource, options) => exportResource("tsuga_custom_usage_tag", resource, options),
    exportDashboard: (resource, options) => exportResource("tsuga_dashboard", resource, options),
    exportDashboardFolder: (resource, options) => exportResource("tsuga_dashboard_folder", resource, options),
    exportIngestionApiKey: (resource, options) => exportResource("tsuga_ingestion_api_key", resource, options),
    exportMetricRoute: (resource, options) => exportResource("tsuga_metric_route", resource, options),
    exportMonitor: (resource, options) => exportResource("tsuga_monitor", resource, options),
    exportMonitorFolder: (resource, options) => exportResource("tsuga_monitor_folder", resource, options),
    exportNotificationIntegration: (resource, options) => exportResource("tsuga_notification_integration", resource, options),
    exportNotificationRule: (resource, options) => exportResource("tsuga_notification_rule", resource, options),
    exportNotificationSilence: (resource, options) => exportResource("tsuga_notification_silence", resource, options),
    exportOperationApiKey: (resource, options) => exportResource("tsuga_operation_api_key", resource, options),
    exportRetentionPolicy: (resource, options) => exportResource("tsuga_retention_policy", resource, options),
    exportRoute: (resource, options) => exportResource("tsuga_route", resource, options),
    exportRumPublicToken: (resource, options) => exportResource("tsuga_rum_public_token", resource, options),
    exportSlo: (resource, options) => exportResource("tsuga_slo", resource, options),
    exportTagPolicy: (resource, options) => exportResource("tsuga_tag_policy", resource, options),
    exportTeam: (resource, options) => exportResource("tsuga_team", resource, options),
    exportTeamMembership: (resource, options) => exportResource("tsuga_team_membership", resource, options),
  };
}
