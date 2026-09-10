// Generated from the Terraform provider resource registry.
export const resourceTypes = Object.freeze(["tsuga_cloud_account","tsuga_custom_usage_tag","tsuga_dashboard","tsuga_dashboard_folder","tsuga_ingestion_api_key","tsuga_monitor","tsuga_notification_integration","tsuga_notification_rule","tsuga_notification_silence","tsuga_retention_policy","tsuga_route","tsuga_slo","tsuga_tag_policy","tsuga_team","tsuga_team_membership"]);
export function bindResources(exportResource) {
  return {
    exportCloudAccount: (resource, options) => exportResource("tsuga_cloud_account", resource, options),
    exportCustomUsageTag: (resource, options) => exportResource("tsuga_custom_usage_tag", resource, options),
    exportDashboard: (resource, options) => exportResource("tsuga_dashboard", resource, options),
    exportDashboardFolder: (resource, options) => exportResource("tsuga_dashboard_folder", resource, options),
    exportIngestionApiKey: (resource, options) => exportResource("tsuga_ingestion_api_key", resource, options),
    exportMonitor: (resource, options) => exportResource("tsuga_monitor", resource, options),
    exportNotificationIntegration: (resource, options) => exportResource("tsuga_notification_integration", resource, options),
    exportNotificationRule: (resource, options) => exportResource("tsuga_notification_rule", resource, options),
    exportNotificationSilence: (resource, options) => exportResource("tsuga_notification_silence", resource, options),
    exportRetentionPolicy: (resource, options) => exportResource("tsuga_retention_policy", resource, options),
    exportRoute: (resource, options) => exportResource("tsuga_route", resource, options),
    exportSlo: (resource, options) => exportResource("tsuga_slo", resource, options),
    exportTagPolicy: (resource, options) => exportResource("tsuga_tag_policy", resource, options),
    exportTeam: (resource, options) => exportResource("tsuga_team", resource, options),
    exportTeamMembership: (resource, options) => exportResource("tsuga_team_membership", resource, options),
  };
}
