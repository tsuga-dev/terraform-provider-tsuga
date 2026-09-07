import {test} from 'node:test';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {createServer} from 'node:http';
import {mkdtemp, readFile, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

test('all resource bindings run in a real browser', {timeout:60000}, async () => {
  const root = path.resolve(fileURLToPath(new URL('../../', import.meta.url)));
  const profile = await mkdtemp(path.join(tmpdir(), 'terraform-export-browser-'));
  const browserPath = process.env.CHROME_BIN ?? (process.platform === 'darwin' ? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' : 'google-chrome');
  const {promise: result, resolve, reject} = Promise.withResolvers();
  const server = createServer(async (request, response) => {
    try {
      const pathname = new URL(request.url, 'http://localhost').pathname;
      if (pathname === '/result') {
        let body = '';
        for await (const chunk of request) body += chunk;
        response.end('ok');
        resolve(JSON.parse(body));
      } else if (pathname === '/') {
        response.setHeader('Content-Type', 'text/html');
        response.end(`<script type="module">
import {createTerraformExporter} from '/wasm/dist/browser.js';
import {verifyExporter} from '/wasm/test/contract.mjs';
try {
  const prefix='/internal/provider/testdata/terraform-export/';
  const fixtures=await (await fetch(prefix+'fixtures.json')).json();
  const count=await verifyExporter(createTerraformExporter,fixtures,async name=>(await fetch(prefix+name+'.tf')).text());
  await fetch('/result',{method:'POST',body:JSON.stringify({ok:true,count})});
} catch(error) {
  await fetch('/result',{method:'POST',body:JSON.stringify({ok:false,error:String(error.stack)})});
}
</script>`);
      } else {
        const file = path.resolve(root, '.'+pathname);
        if (!file.startsWith(root+path.sep) || !/^\/(wasm\/(dist|test)\/|internal\/provider\/testdata\/terraform-export\/)/.test(pathname)) {
          response.writeHead(404).end(); return;
        }
        response.setHeader('Content-Type', file.endsWith('.wasm') ? 'application/wasm' : /\.(m?js)$/.test(file) ? 'text/javascript' : 'text/plain');
        response.end(await readFile(file));
      }
    } catch(error) { response.writeHead(500).end(); reject(error); }
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve); });
  const browser = spawn(browserPath, ['--headless', '--no-sandbox', '--disable-dev-shm-usage', '--no-first-run', '--no-default-browser-check', '--user-data-dir='+profile, `http://127.0.0.1:${server.address().port}`], {stdio:'ignore'});
  browser.once('error', reject);
  browser.once('exit', code => reject(new Error(`Browser exited before reporting results (${code})`)));
  const timer = setTimeout(() => reject(new Error('Browser test timed out')), 45000);
  try {
    const report = await result;
    assert.equal(report.ok, true, report.error);
    assert.ok(report.count >= 15);
  } finally {
    clearTimeout(timer);
    const stopped = new Promise(resolve => browser.once('exit', resolve));
    if (browser.exitCode === null && browser.pid) { browser.kill(); await stopped; }
    server.closeAllConnections();
    await new Promise(resolve => server.close(resolve));
    await rm(profile, {recursive:true, force:true, maxRetries:5, retryDelay:100});
  }
});
