// Control the order of synthetic writes and reads through the real UI.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

function gate() {
  let release;
  const promise = new Promise(resolve => { release = resolve; });
  return { promise, release };
}

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exit = once(fixture, 'exit');
  let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key } = JSON.parse(line);
    const { origin } = new URL(url);
    assert.notEqual(new URL(url).port, '8317');
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const results = {};
    const out = process.env.UI_OUTPUT;
    if (out) await fs.mkdir(out, { recursive: true });
    const screenshot = (page, name) => out && page.screenshot({ path: path.join(out, name + '.png'), animations: 'disabled' });
    async function scenario(name, interact) {
      const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' });
      const page = await context.newPage();
      page.setDefaultTimeout(6000);
      const tasks = new Map(), reads = [], posts = [], cancels = [], errors = [];
      const control = {};
      page.on('pageerror', e => errors.push(e.message));
      await context.route('**/*', async route => {
        const req = route.request(), u = new URL(req.url());
        assert.equal(u.origin, origin);
        const p = u.pathname;
        if (p.endsWith('/runs') && req.method() === 'POST') {
          const config = req.postDataJSON();
          posts.push(config.idempotency_key);
          if (control.dropBeforeAdmission) { control.dropBeforeAdmission = false; return route.abort('connectionfailed'); }
          let run = tasks.get(config.idempotency_key);
          if (!run) {
            run = { id: 'manual-' + (tasks.size + 1), source: 'manual_test', status: 'running', config, samples: [], created_at: '2026-11-20T02:00:00Z' };
            tasks.set(config.idempotency_key, run);
          }
          if (control.ack) await control.ack.promise;
          if (control.loseAck) { control.loseAck = false; return route.abort('connectionfailed'); }
          return route.fulfill({ status: 202, json: { run_id: run.id } });
        }
        if (p.endsWith('/runs/cancel') && req.method() === 'POST') {
          cancels.push(req.postDataJSON().id);
          return route.fulfill({ json: {} });
        }
        assert.equal(req.method(), 'GET');
        if (p.endsWith('/runs/detail')) {
          const id = u.searchParams.get('id'), key = u.searchParams.get('idempotency_key');
          reads.push(id || 'by-receipt');
          if (control.hold?.id === id) {
            const { entered, wait } = control.hold;
            entered.release();
            await wait.promise;
          }
          if (control.otherActive === id && id) {
            const run = await (await route.fetch()).json();
            return route.fulfill({ json: { ...run, status: 'running' } });
          }
          if (key) {
            if (control.failReceiptReads) {
              control.failReceiptReads--;
              return route.fulfill({ status: 503, json: { error: { code: 'unavailable', message: 'Synthetic read failure' } } });
            }
            const found = tasks.get(key);
            return route.fulfill(found ? { json: found } : { status: 404, json: { error: { code: 'not_found', message: 'No synthetic run' } } });
          }
          const manual = [...tasks.values()].find(run => run.id === id);
          if (manual) return route.fulfill({ json: manual });
          if (id === 'fixture-run-observation' || id === 'fixture-run-observation-error') {
            const run = await (await route.fetch()).json();
            return route.fulfill({ json: { ...run, id, status: 'running', action_status: 'disable_pending' } });
          }
        }
        if (p.endsWith('/runs') && u.searchParams.get('source') === 'manual_test') {
          const history = await (await route.fetch()).json();
          return route.fulfill({ json: { ...history, items: [...tasks.values(), ...history.items].slice(0, 10), total: history.total + tasks.size } });
        }
        return route.continue();
      });
      try {
        await page.goto(url);
        await page.click('#login-connect');
        await page.fill('#management-key', management_key);
        await page.click('#connection-form button[type=submit]');
        await page.waitForSelector('body.connected');
        await page.click('[data-page=manual]');
        async function configure() {
          await page.selectOption('#target', 'fixture-quality-primary');
          await page.selectOption('#models', 'fixture-reasoner');
          await page.selectOption('#question', 'logic-candy');
        }
        await configure();
        results[name] = await interact({ page, tasks, reads, posts, cancels, control, configure });
        assert.deepEqual(errors, []);
      } finally { await context.close(); }
    }
    const selected = page => page.locator('#run-frozen-data .run-identifier').textContent({ timeout: 1500 }).catch(() => '');
    const submit = page => page.locator('#run-submit');
    const download = async (page, button) => {
      const pending = page.waitForEvent('download');
      await page.click(button);
      const file = await pending;
      return JSON.parse(await fs.readFile(await file.path(), 'utf8')).id;
    };
    await scenario('history_poll_after_selection', async ({ page, control }) => {
      control.otherActive = 'fixture-run-complete';
      await page.click('[data-manual-view=history]');
      await page.click('[data-run=fixture-run-complete]');
      await page.waitForFunction(() => document.querySelector('#run-frozen-data').textContent.includes('fixture-run-complete'));
      control.hold = { id: control.otherActive, entered: gate(), wait: gate() };
      await control.hold.entered.promise;
      await page.click('[data-manual-view=history]');
      await page.click('[data-run=fixture-run-visual]');
      await page.waitForFunction(() => document.querySelector('#run-frozen-data').textContent.includes('fixture-run-visual'));
      const late = page.waitForResponse(r => r.url().endsWith('/runs/detail?id=fixture-run-complete'));
      control.hold.wait.release();
      await (await late).finished();
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      return { afterLatePoll: await selected(page), exported: await download(page, '#run-export') };
    });
    await scenario('delayed_ack_and_dialog', async ({ page, reads, control, cancels, posts }) => {
      control.ack = gate();
      const receipt = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/runs'));
      await submit(page).click();
      await page.locator('#run-submit-label').filter({ hasText: '提交中' }).waitFor();
      await page.click('[data-manual-view=history]');
      await page.click('[data-run=fixture-run-complete]');
      await page.waitForFunction(() => document.querySelector('#run-frozen-data')?.textContent.includes('fixture-run-complete'));
      control.ack.release();
      await receipt;
      await page.locator('#run-submit-label').filter({ hasText: '测试中' }).waitFor();
      const readAfterAck = await selected(page);
      await screenshot(page, 'slow-receipt');
      await page.click('[data-manual-view=history]');
      control.hold = { id: 'fixture-run-visual', entered: gate(), wait: gate() };
      await page.click('[data-run=fixture-run-visual]');
      await control.hold.entered.promise;
      const pollDuringHistoryRead = page.waitForRequest(r => r.url().endsWith('/runs/detail?id=manual-1'), { timeout: 3500 }).then(() => true, () => false);
      await page.click('[data-page=operations]');
      await page.click('#operations-history [data-run=fixture-run-observation]');
      await page.waitForSelector('#quality-detail-export:visible');
      const initialDownload = await download(page, '#quality-detail-export');
      const trackedDuringHistory = await pollDuringHistoryRead;
      await page.keyboard.press('Escape');
      await page.click('#operations-history [data-run=fixture-run-observation-error]');
      await page.waitForSelector('#quality-detail-export:visible');
      const flushed = page.waitForResponse(r => r.url().endsWith('/runs/detail?id=fixture-run-visual'));
      control.hold.wait.release();
      await flushed;
      await page.click('#quality-detail-cancel');
      const qualityDownload = await download(page, '#quality-detail-export');
      await page.keyboard.press('Escape');
      const trigger = await page.evaluate(() => document.activeElement?.dataset.run);
      await page.click('[data-page=manual]');
      const manualDownload = await download(page, '#run-export');
      return { posts: posts.length, readAfterAck, reads, trackedDuringHistory, initialDownload, qualityDownload, manualDownload, trigger, cancels };
    });
    await scenario('lost_receipt', async ({ page, reads, posts, tasks, control, configure }) => {
      control.loseAck = true;
      const lost = page.waitForEvent('requestfailed', { predicate: request => request.method() === 'POST' && request.url().endsWith('/runs') });
      await submit(page).click();
      await lost;
      await page.waitForFunction(() => !document.querySelector('#run-submit').disabled || document.querySelector('#run-frozen-data').textContent.includes('manual-1'));
      const recovered = await selected(page);
      await screenshot(page, 'lost-receipt');
      if (await submit(page).isEnabled()) await submit(page).click();
      await page.reload();
      await page.waitForSelector('body.connected');
      await page.click('[data-page=manual]');
      await configure();
      await page.waitForFunction(() => !document.querySelector('#run-submit').disabled || document.querySelector('#run-frozen-data').textContent.includes('manual-1'));
      const afterReconnect = await selected(page);
      return { posts: posts.length, distinctKeys: new Set(posts).size, admitted: tasks.size, recovered, afterReconnect, reads };
    });
    await scenario('missing_receipt', async ({ page, reads, posts, tasks, control }) => {
      control.dropBeforeAdmission = true;
      const lost = page.waitForEvent('requestfailed', { predicate: request => request.method() === 'POST' && request.url().endsWith('/runs') });
      await submit(page).click();
      await lost;
      await page.waitForSelector('#run-submit:enabled');
      const beforeRetry = [...reads];
      await submit(page).click();
      await page.waitForFunction(() => document.querySelector('#run-frozen-data').textContent.includes('manual-1'));
      return { beforeRetry, posts: posts.length, distinctKeys: new Set(posts).size, admitted: tasks.size };
    });
    await scenario('interrupted_receipt_read', async ({ page, reads, posts, tasks, control }) => {
      control.loseAck = true;
      control.failReceiptReads = 1;
      const failedRead = page.waitForResponse(response => response.status() === 503 && response.url().includes('idempotency_key='), { timeout: 3500 }).then(() => true, () => false);
      await submit(page).click();
      const checked = await failedRead;
      if (checked) {
        await page.locator('#run-submit-label').filter({ hasText: '核对中' }).waitFor();
        assert(await submit(page).isDisabled());
        await page.locator('#run-form').evaluate(form => form.requestSubmit());
        await page.reload();
        await page.waitForSelector('body.connected');
        await page.click('[data-page=manual]');
        await page.waitForFunction(() => document.querySelector('#run-frozen-data').textContent.includes('manual-1'));
      }
      return { checked, reads, admitted: tasks.size, posts: posts.length, afterReconnect: await selected(page) };
    });
    if (out) await fs.writeFile(path.join(out, 'receipts.json'), JSON.stringify(results, null, 2) + '\n');
    const delayed = results.delayed_ack_and_dialog;
    assert.equal(delayed.posts, 1);
    assert.equal(delayed.readAfterAck, '任务 fixture-run-complete');
    assert(delayed.trackedDuringHistory, 'A slow history read must not pause the pending manual poll');
    assert.equal(delayed.initialDownload, 'fixture-run-observation');
    assert.equal(delayed.qualityDownload, 'fixture-run-observation-error');
    assert.equal(delayed.manualDownload, 'fixture-run-visual');
    assert.equal(delayed.trigger, 'fixture-run-observation-error');
    assert.equal(results.history_poll_after_selection.exported, 'fixture-run-visual');
    assert.deepEqual(delayed.cancels, ['fixture-run-observation-error']);
    assert.equal(results.lost_receipt.posts, 1);
    assert.equal(results.lost_receipt.admitted, 1);
    assert.equal(results.lost_receipt.recovered, '任务 manual-1');
    assert.equal(results.lost_receipt.afterReconnect, '任务 manual-1');
    assert(results.missing_receipt.beforeRetry.includes('by-receipt'));
    assert.equal(results.missing_receipt.posts, 2);
    assert.equal(results.missing_receipt.distinctKeys, 1);
    assert.equal(results.missing_receipt.admitted, 1);
    assert.equal(results.interrupted_receipt_read.posts, 1);
    assert.equal(results.interrupted_receipt_read.admitted, 1);
    assert.equal(results.interrupted_receipt_read.afterReconnect, '任务 manual-1');
    console.log('Delayed, lost and missing manual receipts, reconnect, reversed detail reads, dialog focus, cancellation and exports passed.');
  } finally {
    await browser?.close();
    fixture.kill();
    await exit;
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
