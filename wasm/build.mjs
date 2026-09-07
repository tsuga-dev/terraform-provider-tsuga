import {execFileSync} from 'node:child_process';
import {copyFile, mkdir, readFile, rm, writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import path from 'node:path';

const directory = fileURLToPath(new URL('.', import.meta.url));
const root = path.dirname(directory);
const dist = path.join(directory, 'dist');
const generated = path.join(directory, 'generated');
await rm(dist, {recursive: true, force: true});
await mkdir(dist, {recursive: true});
execFileSync('go', ['run', './cmd/terraform-export-bindings', generated], {cwd: root, stdio: 'inherit'});
for (const name of ['resources.js', 'index.d.ts']) await copyFile(path.join(generated, name), path.join(dist, name));
execFileSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', path.join(dist, 'terraform-export.wasm'), './cmd/terraform-export-wasm'], {
  cwd: root,
  stdio: 'inherit',
  env: {...process.env, GOOS: 'js', GOARCH: 'wasm', CGO_ENABLED: '0'},
});
const goRoot = execFileSync('go', ['env', 'GOROOT'], {encoding: 'utf8'}).trim();
const runtime = await readFile(path.join(goRoot, 'lib', 'wasm', 'wasm_exec.js'), 'utf8');
// Give each instance its own Go global and callback registry. This also avoids
// overwriting another package's Go runtime when multiple Go versions coexist.
await writeFile(path.join(dist, 'go-runtime.js'), `const hostGlobal = globalThis;
export function createGoRuntime() {
  const timers = new Set();
  const setTimeout = (callback, delay) => {
    const id = hostGlobal.setTimeout(() => { timers.delete(id); callback(); }, delay);
    timers.add(id);
    return id;
  };
  const clearTimeout = (id) => { timers.delete(id); hostGlobal.clearTimeout(id); };
  const globalThis = new Proxy(Object.create(null), {
    get(target, key) {
      return Object.hasOwn(target, key) ? target[key] : Reflect.get(hostGlobal, key, hostGlobal);
    },
  });
${runtime}
  return {
    go: new globalThis.Go(),
    bridge: globalThis,
    dispose: () => { for (const id of timers) hostGlobal.clearTimeout(id); timers.clear(); },
  };
}
`);
try {
  await copyFile(path.join(goRoot, 'LICENSE'), path.join(dist, 'GO-LICENSE'));
} catch (error) {
  if (error.code !== 'ENOENT') throw error;
  // Homebrew installs the license next to libexec rather than inside GOROOT.
  await copyFile(path.join(goRoot, '..', 'LICENSE'), path.join(dist, 'GO-LICENSE'));
}
await copyFile(path.join(root, 'LICENSE'), path.join(directory, 'LICENSE'));
for (const name of ['runtime.js', 'browser.js']) {
  await copyFile(path.join(directory, 'src', name), path.join(dist, name));
}
const version = execFileSync('go', ['version'], {encoding: 'utf8'}).trim();
await writeFile(path.join(dist, 'build.json'), JSON.stringify({go: version}, null, 2)+'\n');
console.log(`Built ${dist}`);
