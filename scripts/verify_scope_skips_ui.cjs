// Render real rule and receipt components with frozen synthetic dispatches only.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exit = once(fixture, 'exit');
  let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key } = JSON.parse(line), { origin, port, hostname } = new URL(url);
    assert.equal(hostname, '127.0.0.1'); assert.notEqual(port, '8317');
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce', timezoneId: 'Asia/Shanghai' });
    page.setDefaultTimeout(6500);
    const out = process.env.UI_OUTPUT;
    if (out) await fs.mkdir(out, { recursive: true });
    const writes = [], errors = [], captures = [];
    page.on('pageerror', err => errors.push(err.message));
    const snapshot = [
      { auth_id: 'file-a.json', email: 'shared@example.test', code: 'missing_model', model: 'synthetic-pro', reason: '凭证未注册全部所选模型。' },
      { auth_id: 'file-b.json', email: 'shared@example.test', code: 'quality_paused', reason: '凭证仍受质量暂停保护，禁止检测。' },
      { auth_id: 'file-c.json', code: 'manual_disabled', reason: '凭证由管理员停用，当前规则无权复检。' },
    ];
    const next = [{ auth_id: 'file-d.json', code: 'snapshot_unavailable', reason: '无法从 CPA 核对本轮凭证快照。' }];
    const stamp = '2026-11-20T02:35:00Z';
    const rule = (id, name, last_scope_dispatch) => ({ id, name, revision: 1, enabled: true, timer_active: false, action: 'observe_only', target_scope: 'all_credentials', last_scope_dispatch, models: ['fixture-reasoner'], samples: 1, question_id: 'logic-candy', judge: { mode: 'deterministic' } });
    const rules = [rule('a', '主模型目录', { at: stamp, matched: 0, skipped: 3, details: snapshot }), rule('b', '备用目录'), rule('old', '旧版汇总', { at: stamp, matched: 0, skipped: 7 })];
    let failRules = false;
    await page.route('**/*', async route => {
      const req = route.request(), u = new URL(req.url()); assert.equal(u.origin, origin);
      const api = u.pathname.slice('/v0/management/plugins/model-quality'.length);
      if (req.method() === 'GET') {
        if (api === '/rules') return failRules ? route.fulfill({ status: 503, json: { error: 'temporary rule list failure' } }) : route.fulfill({ json: { items: rules, total: rules.length, offset: 0 } });
        return route.continue();
      }
      assert.equal(req.method(), 'POST'); assert.equal(api, '/rules/run');
      const body = req.postDataJSON(); writes.push(body);
      if (body.id === 'b') {
        // A different, later timer dispatch reaches the rule directory before the readback.
        rules[1].last_scope_dispatch = { at: '2026-11-20T03:40:00Z', matched: 0, skipped: 1, details: next };
        return route.fulfill({ json: { count: 0, skipped: 3, skip_details: snapshot, skip_reasons: { manual_disabled: 1 } } });
      }
      if (body.id === 'a') return route.fulfill({ json: { count: 2, skipped: 3, scope_batch_id: 'frozen-batch', skip_details: snapshot } });
      return route.fulfill({ json: { count: 0, skipped: 7 } });
    });
    const el = selector => page.locator(selector);
    const card = id => el('[data-key="rule:' + id + '"]');
    const dispatch = id => card(id).locator('.scope-dispatch');
    const receipt = el('#ops-dispatch-receipt');
    const execute = async id => {
      await card(id).locator('[data-rule-run]').click();
      await page.click('#action-confirm');
      await el('#action-dialog').waitFor({ state: 'hidden' });
    };
    const checkThree = async node => {
      assert.equal(await node.locator('li').count(), 3);
      const texts = await node.locator('li').allTextContents();
      for (const [i, row] of snapshot.entries()) {
        assert(texts[i].includes(row.auth_id) && texts[i].includes(row.reason));
        if (row.email) assert(texts[i].includes(row.email));
        if (row.model) assert(texts[i].includes(row.model));
        assert(!snapshot.some(other => other !== row && texts[i].includes(other.reason)));
      }
    };
    await page.goto(url); await page.click('#connection-open'); await page.fill('#management-key', management_key);
    await page.click('#connection-form button[type=submit]'); await page.waitForSelector('body.connected'); await page.click('[data-page=operations]');
    assert.equal(await dispatch('a').locator('li').count(), 0);
    await dispatch('a').locator('summary').focus(); await page.keyboard.press('Enter'); await dispatch('a').locator('li').first().waitFor();
    assert.match(await dispatch('a').locator('summary').innerText(), /纳入检测 0 个 · 跳过 3 个/);
    assert.equal(await dispatch('a').locator('time').getAttribute('datetime'), stamp);
    await checkThree(dispatch('a'));
    await page.click('[data-rule-select=a]'); await execute('b');
    assert.match(await receipt.locator('summary').innerText(), /手动执行回执 · 备用目录.*纳入检测 0 个 · 跳过 3 个/s);
    await checkThree(receipt);
    assert.equal(await el('#ops-scope-label').innerText(), '主模型目录');
    await dispatch('b').locator('summary').click(); await dispatch('b').locator('li').filter({ hasText: 'file-d.json' }).waitFor();
    assert(!(await dispatch('b').innerText()).includes('file-a.json')); assert(!(await receipt.innerText()).includes('file-d.json'));
    await dispatch('b').locator('summary').click();
    await el('#rule-search').fill('主模型'); await dispatch('a').locator('summary').focus();
    const before = await el('#rules-list').evaluate(n => n.scrollTop);
    await page.click('#refresh'); await page.waitForFunction(() => !document.querySelector('#refresh').hasAttribute('aria-busy'));
    assert.equal(await el('#rule-search').inputValue(), '主模型'); assert(await dispatch('a').evaluate(n => n.open));
    assert.equal(await el('#rules-list').evaluate(n => n.scrollTop), before); await checkThree(dispatch('a')); await checkThree(receipt);
    for (const [width, height, theme] of [[1440,900,'light'],[390,844,'dark'],[320,710,'white']]) {
      await page.setViewportSize({ width, height }); await page.selectOption('#theme-toggle', theme);
      for (const [kind, surface] of [['rule', dispatch('a')], ['receipt', receipt]]) {
        await surface.scrollIntoViewIfNeeded();
        const geometry = await surface.evaluate(surface => {
          const list = surface.querySelector('ul'), viewport = list.getBoundingClientRect();
          return { noOverflow: document.documentElement.scrollWidth <= innerWidth && list.scrollWidth <= list.clientWidth + 1, allVisible: [...list.children].every(row => {
            const r = row.getBoundingClientRect(); return r.top >= viewport.top - 1 && r.bottom <= viewport.bottom + 1 && r.top >= -1 && r.bottom <= innerHeight + 1;
          }) };
        });
        assert(geometry.noOverflow && geometry.allVisible, JSON.stringify({ width, kind, geometry }));
        captures.push({ width, kind, ...geometry });
        if (out) await page.screenshot({ path: path.join(out, kind + '-' + width + '.png'), animations: 'disabled' });
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 }); await el('#rule-search').fill('');
    await dispatch('old').locator('summary').click(); await dispatch('old').getByText('本轮未记录逐凭证明细。').waitFor();
    assert.equal(await dispatch('old').locator('li').count(), 0); await execute('old');
    assert.match(await receipt.innerText(), /跳过 7 个/); assert.equal(await receipt.locator('li').count(), 0);
    failRules = true; await execute('a');
    assert.match(await receipt.innerText(), /手动执行回执 · 主模型目录.*纳入检测 2 个 · 跳过 3 个/s); await checkThree(receipt);
    assert(await el('#ops-batch-bar').isVisible()); assert(await el('#rules-read-note').isVisible());
    assert.deepEqual(writes.map(w => w.id), ['b', 'old', 'a']); assert.deepEqual(errors, []);
    if (out) await fs.writeFile(path.join(out, 'scope-skips.json'), JSON.stringify({ sample: snapshot, captures, writes, readback: 'Later scheduled dispatch stayed on its own card, original receipt retained.', oldAggregate: 7, confirmedBatch: 'frozen-batch', modelCalls: 0, credentialWrites: 0 }, null, 2) + '\n');
    console.log('Persisted and manual skip details, duplicate emails, filters, scroll, independent schedules, legacy totals and partial reads passed.');
  } finally { await browser?.close(); fixture.kill(); await exit; }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
