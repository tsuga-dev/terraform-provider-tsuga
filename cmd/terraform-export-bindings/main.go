// Command terraform-export-bindings generates the resource methods from the provider registry.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"terraform-provider-tsuga/internal/provider"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: terraform-export-bindings OUTPUT_DIRECTORY")
	}
	directory := os.Args[1]
	resources, err := provider.TerraformResourceTypes(context.Background())
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(resources)
	if err != nil {
		return err
	}
	var js, ts strings.Builder
	js.WriteString("// Generated from the Terraform provider resource registry.\n")
	fmt.Fprintf(&js, "export const resourceTypes = Object.freeze(%s);\n", encoded)
	js.WriteString("export function bindResources(exportResource) {\n  return {\n")
	ts.WriteString("// Generated from the Terraform provider resource registry.\n")
	ts.WriteString("export type ResourceType = ")
	for i, name := range resources {
		if i > 0 {
			ts.WriteString(" | ")
		}
		fmt.Fprintf(&ts, "%q", name)
	}
	ts.WriteString(";\nexport declare const resourceTypes: readonly ResourceType[];\n")
	ts.WriteString(`
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
`)
	for _, name := range resources {
		parts := strings.Split(strings.TrimPrefix(name, "tsuga_"), "_")
		method := "export"
		for _, part := range parts {
			method += strings.ToUpper(part[:1]) + part[1:]
		}
		fmt.Fprintf(&js, "    %s: (resource, options) => exportResource(%q, resource, options),\n", method, name)
		fmt.Fprintf(&ts, "  %s(resource: object | string, options?: ExportOptions): ExportResult;\n", method)
	}
	js.WriteString("  };\n}\n")
	ts.WriteString("}\n/** Loads an isolated Go runtime. Supply bytes/module to override the default WASM asset. */\nexport declare function createTerraformExporter(source?: WasmSource): Promise<TerraformExporter>;\n")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, "resources.js"), []byte(js.String()), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "index.d.ts"), []byte(ts.String()), 0644)
}
