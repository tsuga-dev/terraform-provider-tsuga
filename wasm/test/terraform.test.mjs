import {test} from 'node:test';
import {execFileSync} from 'node:child_process';
import {mkdtemp, mkdir, readFile, rm, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {createTerraformExporter} from '../dist/browser.js';

test('Terraform validates every exported resource against the built provider', {timeout:120000}, async () => {
  const root = fileURLToPath(new URL('../../', import.meta.url));
  let directory;
  let exporter;
  try {
    directory = await mkdtemp(path.join(tmpdir(), 'terraform-export-validation-'));
    exporter = await createTerraformExporter(
      await readFile(new URL('../dist/terraform-export.wasm', import.meta.url)),
    );
    const platform = JSON.parse(execFileSync('go', ['env','-json','GOOS','GOARCH'], {encoding:'utf8'}));
    const mirror = path.join(directory, 'mirror');
    const providerPath = path.join(mirror, 'registry.terraform.io/tsuga-dev/tsuga/0.0.0', `${platform.GOOS}_${platform.GOARCH}`);
    await mkdir(providerPath, {recursive:true});
    execFileSync('go', ['build','-o',path.join(providerPath,'terraform-provider-tsuga_v0.0.0'),'.'], {cwd:root, stdio:'pipe'});
    const fixtures = JSON.parse(await readFile(path.join(root,'internal/provider/testdata/terraform-export/fixtures.json'),'utf8'));
    let configuration = 'terraform {\n required_providers {\n tsuga = { source = "tsuga-dev/tsuga", version = "0.0.0" }\n }\n}\n';
    for (const fixture of fixtures) {
      configuration += exporter.exportResource(fixture.resourceType, fixture.resource, {resourceName:fixture.name.replaceAll('-','_'), includeImport:fixture.resourceType !== 'tsuga_cloud_account'}).hcl;
    }
    await writeFile(path.join(directory,'main.tf'), configuration);
    const cliConfig = path.join(directory,'terraform.rc');
    await writeFile(cliConfig, `provider_installation {\n dev_overrides { "registry.terraform.io/tsuga-dev/tsuga" = ${JSON.stringify(providerPath)} }\n}\n`);
    const options = {cwd:directory, encoding:'utf8', env:{...process.env, TF_CLI_CONFIG_FILE:cliConfig, CHECKPOINT_DISABLE:'1', TF_IN_AUTOMATION:'1'}};
    execFileSync('terraform',['validate','-no-color'], options);
  } catch(error) {
    throw new Error(`${error.message}\n${error.stdout ?? ''}\n${error.stderr ?? ''}`, {cause:error});
  } finally {
    await exporter?.close();
    if (directory) await rm(directory, {recursive:true,force:true});
  }
});
