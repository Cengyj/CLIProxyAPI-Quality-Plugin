// Browser checks use synthetic credentials and intercept every write locally.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const out = path.resolve(process.env.UI_OUTPUT || 'artifacts/credential-conversion-0.20.0/browser');
async function main() {
  fs.mkdirSync(out, { recursive: true });
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore','pipe','inherit'] });
  const exit = once(fixture, 'exit'); let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key: key } = JSON.parse(line), origin = new URL(url).origin;
    browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, acceptDownloads: true, reducedMotion: 'reduce' });
    const errors = [], writes = []; let mode = 'mixed', pendingUpload, failRefresh = false;
    const source = JSON.stringify({ type: 'sub2api-data', version: 1, accounts: [{ platform: 'openai', type: 'oauth', credentials: { access_token: 'SYNTHETIC_ACCESS_CANARY', refresh_token: 'SYNTHETIC_REFRESH_CANARY', chatgpt_account_id: 'fixture-account' } }] });
    const rows = () => [
      { index: 0, name: '合成账号', email: 'converted@example.test', account_id: 'fixture-account', plan_type: 'plus', filename: 'codex-converted-fixture.json', status: 'ready', warnings: [], credential: { type: 'codex', access_token: 'SYNTHETIC_ACCESS_CANARY', refresh_token: 'SYNTHETIC_REFRESH_CANARY', account_id: 'fixture-account', disabled: false } },
      { index: 1, name: '缺少 token', email: '', plan_type: 'unknown', status: 'invalid', error: 'conversion_missing_access_token', warnings: [] },
    ];
    await context.route('**/*', async route => {
      const req = route.request(), u = new URL(req.url());
      if (u.origin !== origin) return route.abort();
      if (req.method() === 'GET') {
        if (failRefresh && u.pathname === '/v0/management/plugins/model-quality/targets') { failRefresh = false; return route.abort('connectionfailed'); }
        return route.continue();
      }
      writes.push(u.pathname);
      assert(req.headers()['idempotency-key']);
      assert.equal(req.postDataJSON().source, source);
      if (u.pathname.endsWith('/conversion/preview')) return route.fulfill({ json: { items: rows(), ready: 1 } });
      if (u.pathname.endsWith('/conversion/upload')) {
        assert.deepEqual(req.postDataJSON().indexes, [0]);
        if (mode === 'pending') { pendingUpload = route; return; }
        if (mode === 'unknown') return route.abort('connectionfailed');
        return route.fulfill({ json: { items: [{ index: 0, filename: 'codex-converted-fixture.json', status: 'uploaded', enabled: true }] } });
      }
      throw Error('Unexpected write');
    });
    const page = await context.newPage(); page.setDefaultTimeout(7000); page.on('pageerror', e => errors.push(e.message));
    await page.goto(origin); await page.locator('#login-connect').click(); await page.locator('#management-key').fill(key); await page.locator('#connection-form button[type=submit]').click();
    const dialog = page.locator('#account-convert-dialog'), get = name => page.locator('#account-convert-' + name);
    const open = async () => { await get('open').click(); await dialog.waitFor(); };
    const preview = async () => { await get('source').fill(source); await get('preview').click(); await get('results').waitFor(); };
    await open(); assert(await get('preview').isDisabled());
    await get('file').setInputFiles({ name: 'synthetic-sub2api.json', mimeType: 'application/json', buffer: Buffer.from(source) });
    await get('preview').waitFor({ state: 'visible' });
    await page.waitForFunction(() => document.querySelector('#account-convert-source').value.length > 0);
    assert.equal(await get('source').inputValue(), source);
    await dialog.screenshot({ path: path.join(out, 'conversion-input-desktop.png') });
    await get('preview').click(); await get('results').waitFor();
    assert.equal(await get('summary').innerText(), '1 个可上传 · 共 2 个');
    assert.equal(await get('upload').innerText(), '上传 1 个凭证');
    assert.doesNotMatch(await dialog.innerText(), /SYNTHETIC_(ACCESS|REFRESH)_CANARY/);
    const downloadEvent = page.waitForEvent('download'); await page.locator('[data-convert-download="0"]').click();
    const download = await downloadEvent; assert.equal(download.suggestedFilename(), 'codex-converted-fixture.json');
    const downloaded = JSON.parse(fs.readFileSync(await download.path(), 'utf8')); assert.equal(downloaded.type, 'codex'); assert.equal(downloaded.access_token, 'SYNTHETIC_ACCESS_CANARY');
    const layouts = [];
    for (const width of [1440, 1064, 620, 390, 320]) {
      await page.setViewportSize({ width, height: width < 500 ? 640 : 900 });
      const size = await dialog.evaluate(n => ({ width: n.clientWidth, scroll: n.scrollWidth, height: n.clientHeight, viewport: innerWidth }));
      assert(size.scroll <= size.width + 1 && size.width <= width, JSON.stringify(size));
      await get('upload').scrollIntoViewIfNeeded(); assert(await get('upload').isVisible());
      await dialog.screenshot({ path: path.join(out, `conversion-result-${width}.png`) }); layouts.push({ width, ...size });
    }
    await page.setViewportSize({ width: 1064, height: 800 });
    mode = 'pending'; await get('upload').click();
    await page.waitForFunction(() => document.querySelector('#account-convert-upload').textContent === '正在上传…');
    await page.keyboard.press('Escape'); assert(await dialog.evaluate(n => n.open));
    for (const name of ['upload','close','cancel','edit']) assert(await get(name).isDisabled());
    failRefresh = true;
    await pendingUpload.fulfill({ json: { items: [{ index: 0, status: 'uploaded', enabled: true }] } });
    await page.getByText('已上传并启用', { exact: true }).waitFor();
    await page.waitForFunction(() => !document.querySelector('#account-convert-cancel').disabled);
    assert.equal(failRefresh, false);
    assert(await get('upload').isDisabled()); await get('cancel').click();
    assert.equal(await get('source').inputValue(), ''); assert.equal(await get('list').textContent(), '');
    await open(); await preview(); mode = 'unknown'; await get('upload').click();
    await get('feedback').filter({ hasText: '上传结果尚未确认' }).waitFor(); assert(await get('upload').isDisabled());
    assert.equal(writes.filter(p => p.endsWith('/conversion/upload')).length, 2);
    await get('cancel').click(); assert(await get('open').evaluate(n => n === document.activeElement));
    assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(out, 'conversion.json'), JSON.stringify({ layouts, file_import: true, converted_download: true, invalid_rows_skipped: true, token_text_hidden: true, upload_busy_guard: true, uncertain_not_replayed: true, closing_clears_material: true, intercepted_writes: writes.length, real_writes: 0 }, null, 2));
    console.log('Credential conversion UI, download, upload and unknown-result checks passed.');
  } finally { if (browser) await browser.close(); fixture.kill(); await exit; }
}
main().catch(e => { console.error(e); process.exitCode = 1; });
