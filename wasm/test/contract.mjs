function assert(condition, message) {
  if (!condition) throw new Error(message);
}

export async function verifyExporter(createTerraformExporter, fixtures, readGolden) {
  const originalGo = globalThis.Go;
  const originalBridge = globalThis.tsugaTerraformExport;
  const [exporter, second] = await Promise.all([createTerraformExporter(), createTerraformExporter()]);
  try {
    for (const fixture of fixtures) {
      const options = {resourceName: fixture.name.replaceAll('-', '_'), includeImport: fixture.resourceType !== 'tsuga_cloud_account'};
      const result = exporter.exportResource(fixture.resourceType, fixture.resource, options);
      const method = 'export'+fixture.resourceType.slice('tsuga_'.length).split('_').map(s => s[0].toUpperCase()+s.slice(1)).join('');
      assert(typeof exporter[method] === 'function', `Missing binding: ${method}`);
      assert(exporter[method](fixture.resource, options).hcl === result.hcl, `${method} disagrees with generic export`);
      assert(result.hcl === await readGolden(fixture.name), `${fixture.name} differs from native Go export`);
      assert(!result.hcl.includes('never-export-this-secret'), 'Ingestion key leaked into export');
    }
    const team = {id:'team-id', name:'${literal} %{literal}\n"quotes" \\ 🌲', visibility:'public'};
    const hcl = exporter.exportTeam(team).hcl;
    assert(hcl.includes('$${literal}') && hcl.includes('%%{literal}'), 'Template escaping changed literal strings');
    assert(!hcl.includes('import {'), 'Imports must be opt-in');
    for (const invalid of [null, [], 4, '{}', '{', '{"name":12}']) {
      let rejected = false;
      try { exporter.exportTeam(invalid); } catch { rejected = true; }
      assert(rejected, `Accepted invalid resource: ${invalid}`);
    }
    assert(exporter.exportTeam(team).hcl === hcl, 'Failed call corrupted the runtime');
    await exporter.close();
    await exporter.close();
    let closedRejected = false;
    try { exporter.exportTeam(team); } catch { closedRejected = true; }
    assert(closedRejected, 'Closed runtime accepted work');
    assert(second.exportTeam(team).hcl === hcl, 'Closing one runtime broke another');
    assert(globalThis.Go === originalGo && globalThis.tsugaTerraformExport === originalBridge, 'Go runtime polluted host globals');
    return fixtures.length;
  } finally {
    await exporter.close();
    await second.close();
  }
}
