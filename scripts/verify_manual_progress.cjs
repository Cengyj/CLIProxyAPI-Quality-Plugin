// Drive real UI events against an isolated fixture and synthetic run responses.
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

async function test() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key } = JSON.parse(line);
    const { origin } = new URL(url);
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    let posts = 0, reads = 0, run, acknowledge;
    const deleted = new Set();
    let detailReads = 0;
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.route('**/*', async route => {
      const request = route.request(), u = new URL(request.url());
      assert.equal(u.origin, origin);
      if (u.pathname.endsWith('/targets')) {
        const response = await (await route.fetch()).json();
        for (const target of response.items) {
          if (target.auth_id === 'fixture-quality-primary') target.email = 'current@example.test';
        }
        return route.fulfill({ json: response });
      }
      if (u.pathname.endsWith('/runs/detail')) detailReads++;
      if (u.pathname.endsWith('/runs') && request.method() === 'DELETE') {
        deleted.add(request.postDataJSON().id);
        return route.fulfill({ json: { ok: true } });
      }
      if (u.pathname.endsWith('/runs') && request.method() === 'POST') {
        posts++;
        run = { id: 'test-' + posts, credential_email: 'saved@example.test', config: request.postDataJSON(), created_at: new Date().toISOString(), source: 'manual_test', status: 'queued', samples: [] };
        if (posts === 1) await new Promise(resolve => { acknowledge = resolve; });
        return route.fulfill({ status: 202, json: { run_id: run.id } });
      }
      if (u.pathname.endsWith('/runs/cancel') && request.method() === 'POST') {
        run.status = 'cancelling';
        return route.fulfill({ json: {} });
      }
      assert.equal(request.method(), 'GET');
      if (run && u.pathname.endsWith('/runs/detail') && u.searchParams.get('id') === run.id) {
        if (++reads === 1) return route.fulfill({ status: 503, json: { error: 'temporary offline failure' } });
        return route.fulfill({ json: run });
      }
      if (run && u.pathname.endsWith('/runs') && u.searchParams.get('source') === 'manual_test') {
        const offset = Number(u.searchParams.get('offset'));
        const limit = Number(u.searchParams.get('limit'));
        u.searchParams.set('offset', '0');
        u.searchParams.set('limit', '100');
        const page = await (await route.fetch({ url: u.href })).json();
        const rows = [run, ...page.items].filter(item => !deleted.has(item.id));
        return route.fulfill({ json: { ...page, items: rows.slice(offset, offset + limit), total: rows.length, offset, limit } });
      }
      return route.continue();
    });
    await page.goto(origin);
    await page.click('#login-connect');
    await page.fill('#management-key', management_key);
    await page.click('#connection-form button[type=submit]');
    await page.waitForSelector('#account-import-open:enabled');
    await page.click('[data-page=manual]');
    await page.selectOption('#target', 'fixture-quality-primary');
    assert.equal(await page.locator('#target option:checked').textContent(), 'current@example.test');
    await page.selectOption('#models', 'fixture-reasoner');
    await page.selectOption('#reasoning', 'max');
    await page.selectOption('#question', 'logic-candy');
    await page.selectOption('#samples', '2');
    const button = page.locator('#run-submit');
    const label = text => button.filter({ hasText: text }).waitFor();
    const submitted = page.waitForRequest(req => req.method() === 'POST' && req.url().endsWith('/runs'));
    await button.click();
    const body = (await submitted).postDataJSON();
    assert.equal(body.parameters.reasoning_effort, 'max');
    assert.equal(body.auth_id, 'fixture-quality-primary');
    await label('提交中');
    assert(await button.isDisabled());
    await page.locator('#run-form').evaluate(node => node.requestSubmit());
    acknowledge();
    await label('排队中');
    await page.locator('#run-feedback').filter({ hasText: '自动重试查询' }).waitFor();
    run.status = 'running';
    await label('测试中');
    await page.click('[data-manual-view=history]');
    await page.click('[data-run=fixture-run-complete]');
    await page.waitForFunction(() => document.querySelector('#run-frozen-data').textContent.includes('fixture-run-complete'));
    assert((await page.locator('#run-frozen-data').textContent()).includes('current@example.test'));
    assert(await button.isDisabled());
    assert(await page.locator('#run-rerun').isDisabled());
    await page.locator('#run-form').evaluate(node => node.requestSubmit());
    run.samples = ['correct', 'incorrect'].map((answer_match, index) => ({ model: 'fixture-reasoner', index, execution_status: 'completed', quality_verdict: 'unknown', output: String(index + 1), answer_match }));
    run.status = 'completed';
    await page.waitForSelector('#run-submit:enabled');
    assert.equal(posts, 1);
    assert((await page.locator('#run-frozen-data').textContent()).includes('fixture-run-complete'));
    await page.click('[data-manual-view=history]');
    const row = page.locator('[data-run=test-1]');
    await row.locator('.history-trailing .warn').filter({ hasText: /^部分正确$/ }).waitFor();
    assert.equal(await row.locator('.history-credential').textContent(), 'saved@example.test');
    for (const width of [1440, 320]) {
      await page.setViewportSize({ width, height: 900 });
      if (width === 320) await page.click('[data-manual-pane=history]');
      const verdict = await row.locator('.history-trailing .warn').boundingBox();
      const status = await row.getByText('已完成', { exact: true }).boundingBox();
      const trash = await page.locator('[data-delete-run=test-1]').boundingBox();
      assert(verdict.x + verdict.width <= status.x && status.x + status.width <= trash.x);
      assert(Math.abs(verdict.y - status.y) < 1 && trash.x + trash.width <= width);
      const remove = page.locator('[data-delete-run=test-1]');
      const section = row.locator('..');
      await row.hover();
      const [red, green, blue] = await remove.evaluate(el => getComputedStyle(el).color.match(/\d+/g).map(Number));
      assert(red > green * 1.5 && red > blue * 1.5, 'The available delete icon is red before hovering over it');
      for (const target of [row, remove]) {
        await target.hover();
        const shot = await section.screenshot({ animations: 'disabled' });
        if (process.env.UI_OUTPUT) require('node:fs').writeFileSync(path.join(process.env.UI_OUTPUT, `history-${width}-${target === row ? 'body' : 'delete'}.png`), shot);
        const painted = await section.evaluate(el => ({
          row: getComputedStyle(el, '::before').backgroundColor,
          buttons: Array.from(el.querySelectorAll('button'), button => getComputedStyle(button).backgroundColor),
        }));
        assert.notEqual(painted.row, 'rgba(0, 0, 0, 0)');
        assert(painted.buttons.every(color => color === 'rgba(0, 0, 0, 0)'), 'The row owns the hover background across both controls');
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await row.click();
    await page.locator('#sample-list .good').filter({ hasText: '答案正确' }).waitFor();
    await page.locator('#sample-list .warn').filter({ hasText: '答案错误' }).waitFor();
    const details = page.locator('#run-frozen');
    for (const width of [1440, 320]) {
      await page.setViewportSize({ width, height: 900 });
      if (width === 320) await page.click('[data-manual-pane=results]');
      await details.focus();
      const before = await details.boundingBox();
      const meta = await page.locator('#run-meta').boundingBox();
      await page.keyboard.press('Enter');
      await page.locator('#run-frozen-data').waitFor();
      assert((await page.locator('#run-frozen-data').textContent()).includes('saved@example.test'));
      assert.deepEqual(await details.boundingBox(), before);
      assert.deepEqual(await page.locator('#run-meta').boundingBox(), meta);
      const panel = await page.locator('#run-frozen-data').boundingBox();
      assert(panel.y >= meta.y + meta.height - 1 && panel.width > meta.width);
      await page.keyboard.press('Space');
      assert(await page.locator('#run-frozen-data').isHidden());
      assert(await details.evaluate(el => document.activeElement === el));
      assert.deepEqual(await details.boundingBox(), before);
    }
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.click('#run-rerun');
    await label('排队中');
    await page.click('#run-cancel');
    await label('停止中');
    run.status = 'cancelled';
    await page.waitForSelector('#run-submit:enabled');
    await label('开始测试');
    assert.equal(posts, 2);
    await page.click('[data-manual-view=history]');
    const detailBefore = detailReads;
    const trash = page.locator('[data-delete-run=test-2]');
    await trash.focus();
    await page.keyboard.press('Enter');
    await page.locator('#action-dialog[open]').waitFor();
    await page.click('#action-cancel');
    assert.equal(deleted.size, 0);
    await trash.click();
    await page.click('#action-confirm');
    await trash.waitFor({ state: 'detached' });
    await page.locator('#action-dialog').waitFor({ state: 'hidden' });
    assert(deleted.has('test-2'));
    assert.equal(detailReads, detailBefore);
    assert(await page.locator('#manual-history').isVisible());
    await page.click('[data-manual-view=results]');
    assert(await page.locator('#run-empty').isVisible());
    await page.click('[data-manual-view=history]');
    assert.equal(await trash.count(), 0);
    await page.click('#history-next');
    await page.locator('#history-page').filter({ hasText: '第 2 页' }).waitFor();
    const lastPageIds = await page.locator('#history-list [data-delete-run]').evaluateAll(nodes => nodes.map(node => node.dataset.deleteRun));
    for (const id of lastPageIds) {
      await page.click(`[data-delete-run="${id}"]`);
      await page.click('#action-confirm');
      await page.locator('#action-dialog').waitFor({ state: 'hidden' });
      assert(deleted.has(id));
    }
    assert.equal(await page.locator('#history-page').textContent(), '第 1 页');
    assert.deepEqual(errors, []);
    console.log('Manual run lifecycle, verdict placement, stable detail toggle, delete without detail reads, selection cleanup and last-page fallback passed.');
  } finally {
    await browser?.close();
    fixture.kill();
  }
}
test().catch(error => { console.error(error); process.exitCode = 1; });
