import {createGoRuntime} from './go-runtime.js';
import {bindResources, resourceTypes} from './resources.js';

export async function instantiateExporter(source) {
  const {go, bridge, dispose} = createGoRuntime();
  const instantiated = await WebAssembly.instantiate(source, go.importObject);
  const instance = instantiated instanceof WebAssembly.Instance ? instantiated : instantiated.instance;
  let failure;
  let closed = false;
  const completed = go.run(instance).then(
    () => { dispose(); if (!closed) failure = new Error('Terraform export runtime exited unexpectedly'); },
    (error) => { dispose(); failure = error; },
  );
  const api = bridge.tsugaTerraformExport;
  if (!api) {
    await completed;
    throw failure ?? new Error('Terraform export runtime did not initialize');
  }
  function exportResource(resourceType, resource, options = {}) {
    if (closed) throw new Error('Terraform exporter is closed');
    if (failure) throw failure;
    if (!resourceTypes.includes(resourceType)) throw new Error(`Unsupported Terraform resource: ${resourceType}`);
    const resourceJSON = typeof resource === 'string' ? resource : JSON.stringify(resource);
    const parsed = typeof resource === 'string' ? JSON.parse(resource) : resource;
    if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new TypeError('Resource must be a JSON object');
    }
    const request = JSON.stringify({
      resourceType,
      resourceName: options.resourceName,
      includeImport: options.includeImport,
    });
    const response = JSON.parse(api.exportJSON(`${request.slice(0, -1)},"resource":${resourceJSON}}`));
    if (!response.ok) throw new Error(response.error);
    return response.result;
  }
  return {
    exportResource,
    ...bindResources(exportResource),
    async close() {
      if (!closed) {
        closed = true;
        if (!go.exited) api.close();
      }
      await completed;
    },
  };
}
