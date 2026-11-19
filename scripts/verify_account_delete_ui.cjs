// Only the disposable fixture spawned here may receive a delete request.
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exit = once(fixture, 'exit');
  let browser;
  try {
    const [line] = await Promise.race([once(createInterface({ input: fixture.stdout }), 'line'), exit.then(() => { throw Error('Fixture unavailable'); })]);
    const info = JSON.parse(line), url = new URL(info.url), prefix = '/v0/management/plugins/model-quality/login';
    assert.equal(info.fixture, 'synthetic-plugin-ui'); assert.equal(info.pid, fixture.pid);
    assert.equal(url.hostname, '127.0.0.1'); assert.notEqual(url.port, '8317');
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true });
    const context = await browser.newContext({ reducedMotion: 'reduce', viewport: { width: 1440, height: 900 } });
    const writes = [], detailReads = [], unexpected = [], errors = [];
    let mode = '', acceptPending;
    await context.route('**/*', async route => {
      const req = route.request(), u = new URL(req.url());
      if (u.origin !== url.origin) { unexpected.push('external'); return route.abort(); }
      if (req.method() === 'GET') {
        if (u.pathname === prefix + '/accounts/detail') detailReads.push(u.searchParams.get('id'));
        return route.continue();
      }
      if (req.method() !== 'DELETE' || u.pathname !== prefix + '/accounts/entry') { unexpected.push(req.method()); return route.abort(); }
      writes.push({ body: req.postDataJSON(), key: req.headers()['idempotency-key'] });
      if (mode === 'conflict') return route.fulfill({ status: 409, json: { code: 'conflict' } });
      if (mode === 'transport') return route.abort('connectionfailed');
      if (mode === 'no-receipt') return route.fulfill({ json: { deleted: false } });
      if (mode === 'hold') { acceptPending(route); return; }
      unexpected.push('unplanned delete'); return route.abort();
    });
    const page = await context.newPage(); page.setDefaultTimeout(8000); page.on('pageerror', err => errors.push(err.message));
    const el = selector => page.locator(selector), row = id => el(`[data-account-row="${id}"]`);
    const action = (id, name) => row(id).locator(`[data-account-action="${name}"]`);
    const yes = el('#account-delete-yes'), no = el('#account-delete-no'), editor = el('#account-editor-dialog');
    const dialog = page.getByRole('alertdialog', { name: '删除此账号？' });
    const focused = node => node.evaluate(n => n === document.activeElement);
    const open = async id => {
      await action(id, 'delete').click(); await dialog.waitFor();
      assert(await editor.isHidden()); assert(await focused(no));
      assert.equal(await el('#account-delete-identity').textContent(), `${id}@example.test`);
      assert.equal(await dialog.locator('input, textarea, form').count(), 0);
    };
    await page.goto(info.url); await el('#login-connect').click(); await el('#management-key').fill(info.management_key);
    await el('#connection-form button[type=submit]').click(); await el('#account-import-open:enabled').waitFor();
    const { ui_version } = JSON.parse(await el('#build-info').textContent());
    const out = path.resolve(process.env.UI_OUTPUT || path.join(__dirname, `../artifacts/account-delete-${ui_version}/browser`));
    fs.mkdirSync(out, { recursive: true });
    for (const id of ['fixture-active', 'fixture-uncertain']) assert(await action(id, 'delete').isDisabled());
    const layouts = [];
    for (const [width, theme] of [[1440, 'light'], [390, 'dark'], [320, 'white']]) {
      await page.setViewportSize({ width, height: width > 390 ? 900 : 720 }); await el('#theme-toggle').selectOption(theme);
      await open('fixture-enabled');
      assert(await dialog.evaluate(n => n.matches(':modal') && n.scrollWidth <= n.clientWidth));
      for (const button of [no, yes]) {
        const isReachable = await button.evaluate(n => { const r = n.getBoundingClientRect(); return n.contains(document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2)); });
        assert(isReachable); if (width < 600) assert((await button.boundingBox()).height >= 44);
      }
      await dialog.screenshot({ path: path.join(out, `account-delete-${theme}-${width}.png`) }); layouts.push({ width, theme });
      if (width === 1440) await page.keyboard.press('Escape'); else if (width === 390) await page.keyboard.press('Enter'); else await no.click();
      await dialog.waitFor({ state: 'hidden' }); assert(await focused(action('fixture-enabled', 'delete')));
    }
    assert.deepEqual(writes, []); assert.deepEqual(detailReads, []);

    await page.setViewportSize({ width: 1440, height: 900 });
    await action('fixture-new', 'edit').click(); await el('#account-editor-save:enabled').waitFor();
    await el('#account-label').fill('Unsaved fixture note'); await el('#account-delete-open').click(); await dialog.waitFor();
    await page.keyboard.press('Escape'); await dialog.waitFor({ state: 'hidden' });
    assert(await editor.evaluate(n => n.matches(':modal'))); assert(await focused(el('#account-delete-open')));
    assert.equal(await el('#account-label').inputValue(), 'Unsaved fixture note');
    await el('#account-editor-close').click(); assert.equal(await el('#account-password').inputValue(), '');
    assert.deepEqual(writes, []);

    // Advance the displayed revision using the fixture's control plane, then let normal polling observe it.
    await open('fixture-enabled');
    const change = await fetch(info.control_url, { method: 'POST', headers: { 'X-Fixture-Key': info.control_key, 'Content-Type': 'application/json' }, body: JSON.stringify({ action: 'job', id: 'fixture-job-enabled', status: 'committed' }) });
    assert.equal(change.status, 200);
    await el('#account-delete-error').filter({ hasText: '已变化' }).waitFor(); assert(await yes.isDisabled());
    await yes.evaluate(n => n.click()); await no.click(); assert.deepEqual(writes, []);

    for (const fault of ['conflict', 'no-receipt', 'transport']) {
      const before = writes.length; mode = fault; await open('fixture-new'); await yes.click();
      await el('#account-delete-error').filter({ hasText: fault === 'conflict' ? '已变化或已移除' : '未能确认删除结果' }).waitFor();
      assert(await yes.isDisabled()); assert(await row('fixture-new').count());
      assert.doesNotMatch(await el('#account-action-announcement').textContent(), /已删除/);
      await yes.evaluate(n => n.click()); await no.click(); assert.equal(writes.length, before + 1);
      assert.deepEqual(writes.at(-1).body, { id: 'fixture-new', revision: 1 });
    }
    const readState = async () => {
      const r = await fetch(new URL('/__fixture/state', url), { headers: { 'X-Fixture-Key': info.control_key } });
      assert.equal(r.status, 200); return r.json();
    };
    const start = await readState(); mode = 'hold'; const pending = new Promise(resolve => { acceptPending = resolve; });
    await open('fixture-enabled'); await yes.click(); const held = await pending;
    assert(await yes.isDisabled()); assert(await no.isDisabled()); await page.keyboard.press('Escape'); assert(await dialog.isVisible());
    await yes.evaluate(n => n.click()); await held.continue();
    await dialog.waitFor({ state: 'hidden' }); await row('fixture-enabled').waitFor({ state: 'detached' });
    await el('#account-action-announcement').filter({ hasText: '账号资料已删除' }).waitFor({ state: 'attached' });
    assert.equal(await el('#login-notice').count(), 0);
    const end = await readState();
    assert.deepEqual(end.account_ids, start.account_ids.filter(id => id !== 'fixture-enabled')); assert.equal(end.job_count, start.job_count);
    const received = end.calls.filter(c => c.method !== 'GET'); assert.equal(received.length, 1); assert.equal(received[0].method, 'DELETE');
    assert.equal(writes.length, 4); assert.deepEqual(writes.at(-1).body, { id: 'fixture-enabled', revision: 2 });
    assert(writes.every(w => w.key)); assert.deepEqual(detailReads, ['fixture-new']); assert.deepEqual(unexpected, []); assert.deepEqual(errors, []);
    fs.writeFileSync(path.join(out, 'account-delete.json'), JSON.stringify({ ui_version, fixture: 'synthetic, in memory', layouts, standalone_cancel: true, no_row_detail_reads: true, editor_draft_preserved: true, stale_blocked: true, active_and_quarantined_blocked: true, single_submit: true, unknown_not_retried: true, fixture_deletes: received.length, real_writes: 0 }, null, 2) + '\n');
    console.log('Account deletion: standalone dialog, cancel/focus, draft, stale/busy guards, failures and one confirmed synthetic delete passed.');
  } finally { if (browser) await browser.close(); fixture.kill(); await exit; }
}
main().catch(err => { console.error(err); process.exitCode = 1; });
