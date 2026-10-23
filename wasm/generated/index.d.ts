// Generated from the Terraform provider resource registry.
export type ResourceType = "tsuga_adaptive_notification_rule" | "tsuga_cloud_account" | "tsuga_custom_role" | "tsuga_custom_usage_tag" | "tsuga_dashboard" | "tsuga_dashboard_folder" | "tsuga_ingestion_api_key" | "tsuga_metric_route" | "tsuga_monitor" | "tsuga_monitor_folder" | "tsuga_notification_integration" | "tsuga_notification_rule" | "tsuga_notification_silence" | "tsuga_operation_api_key" | "tsuga_retention_policy" | "tsuga_route" | "tsuga_rum_public_token" | "tsuga_slo" | "tsuga_tag_policy" | "tsuga_team" | "tsuga_team_membership";
export declare const resourceTypes: readonly ResourceType[];

export interface ExportOptions {
  /** Terraform local resource name. Defaults to "exported". */
  resourceName?: string;
  /** Include the existing resource's import block. Defaults to false. */
  includeImport?: boolean;
}
export interface ExportVariable {
  name: string;
  path: string;
  sensitive: boolean;
}
export interface ExportResult {
  hcl: string;
  /** Inputs the API cannot export, declared as variables without defaults. */
  variables: ExportVariable[];
  warnings: string[];
}
export type WasmSource = ArrayBuffer | Uint8Array<ArrayBuffer> | WebAssembly.Module;
export interface TerraformExporter {
  /** Pass the single public API resource object, without its data envelope. */
  exportResource(type: ResourceType, resource: object | string, options?: ExportOptions): ExportResult;
  /** Release the Go runtime. Calling this more than once is safe. */
  close(): Promise<void>;
  exportAdaptiveNotificationRule(resource: object | string, options?: ExportOptions): ExportResult;
  exportCloudAccount(resource: object | string, options?: ExportOptions): ExportResult;
  exportCustomRole(resource: object | string, options?: ExportOptions): ExportResult;
  exportCustomUsageTag(resource: object | string, options?: ExportOptions): ExportResult;
  exportDashboard(resource: object | string, options?: ExportOptions): ExportResult;
  exportDashboardFolder(resource: object | string, options?: ExportOptions): ExportResult;
  exportIngestionApiKey(resource: object | string, options?: ExportOptions): ExportResult;
  exportMetricRoute(resource: object | string, options?: ExportOptions): ExportResult;
  exportMonitor(resource: object | string, options?: ExportOptions): ExportResult;
  exportMonitorFolder(resource: object | string, options?: ExportOptions): ExportResult;
  exportNotificationIntegration(resource: object | string, options?: ExportOptions): ExportResult;
  exportNotificationRule(resource: object | string, options?: ExportOptions): ExportResult;
  exportNotificationSilence(resource: object | string, options?: ExportOptions): ExportResult;
  exportOperationApiKey(resource: object | string, options?: ExportOptions): ExportResult;
  exportRetentionPolicy(resource: object | string, options?: ExportOptions): ExportResult;
  exportRoute(resource: object | string, options?: ExportOptions): ExportResult;
  exportRumPublicToken(resource: object | string, options?: ExportOptions): ExportResult;
  exportSlo(resource: object | string, options?: ExportOptions): ExportResult;
  exportTagPolicy(resource: object | string, options?: ExportOptions): ExportResult;
  exportTeam(resource: object | string, options?: ExportOptions): ExportResult;
  exportTeamMembership(resource: object | string, options?: ExportOptions): ExportResult;
}
/** Loads an isolated Go runtime. Supply bytes/module to override the default WASM asset. */
export declare function createTerraformExporter(source?: WasmSource): Promise<TerraformExporter>;
