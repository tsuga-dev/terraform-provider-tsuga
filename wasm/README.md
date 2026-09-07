# Terraform export bindings

`@tsuga/terraform-export` converts Tsuga API resource objects to Terraform HCL
in a browser. The package contains the Go exporter compiled to WebAssembly and
has no runtime JavaScript dependencies or network calls.

## Build

From the provider repository, with Go and Node.js installed:

```sh
node wasm/build.mjs
```

This regenerates `wasm/generated/` from the provider resource registry and writes
the browser package to `wasm/dist/`. The generated files must stay in sync with
the provider registry.

The provider repository is the source of truth. The infrastructure-as-code
repository builds this package after provider changes merge and synchronizes
`dist/`, `package.json`, this README, and the license to the public
`tsuga-dev/terraform-wasm` repository.

## Use

```ts
import {createTerraformExporter} from '@tsuga/terraform-export'

const exporter = await createTerraformExporter()
const result = exporter.exportResource('tsuga_dashboard', dashboard, {
  resourceName: 'api_overview',
  includeImport: true,
})

// result.hcl, result.variables, and result.warnings
await exporter.close()
```


## Verification

```sh
go test ./...
node wasm/build.mjs
node --test wasm/test/browser.test.mjs
node --test wasm/test/terraform.test.mjs
```
