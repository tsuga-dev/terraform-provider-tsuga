import {createTerraformExporter, type ExportResult, type ResourceType} from '../dist/index.js';

async function checkBindings() {
  const exporter = await createTerraformExporter();
  const resource = {id:'example', name:'Example', visibility:'public'};
  const result: ExportResult = exporter.exportTeam(resource, {includeImport:true});
  const type: ResourceType = 'tsuga_dashboard';
  exporter.exportResource(type, resource);
  exporter.exportMonitor('{"id":"monitor"}');
  // @ts-expect-error An unknown provider resource must not typecheck.
  exporter.exportResource('tsuga_does_not_exist', resource);
  // @ts-expect-error Options must be typed.
  exporter.exportTeam(resource, {includeImport:'yes'});
  await exporter.close();
  return result;
}

void checkBindings;
