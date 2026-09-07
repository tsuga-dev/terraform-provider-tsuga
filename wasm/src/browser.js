import {instantiateExporter} from './runtime.js';
export {resourceTypes} from './resources.js';

export async function createTerraformExporter(source) {
  if (source === undefined) {
    const response = await fetch(new URL('./terraform-export.wasm', import.meta.url));
    if (!response.ok) throw new Error(`Unable to load Terraform exporter: HTTP ${response.status}`);
    source = await response.arrayBuffer();
  }
  return instantiateExporter(source);
}
