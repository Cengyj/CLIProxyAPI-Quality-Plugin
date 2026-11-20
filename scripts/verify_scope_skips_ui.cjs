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
    const rule = (id, name, last_scope_dispatch) => ({
      id, name, revision: 1, enabled: true, timer_active: false, action: 'observe_only', target_scope: 'all_credentials',
      object_types: ['oauth'], last_scope_dispatch, models: ['gpt-6.2-sol'], samples: 1, question_id: 'logic-candy', judge: { mode: 'deterministic' },
    });
    const first = rule('a', '主模型巡检', { at: stamp, matched: 0, skipped: 3, details: snapshot });
    first.handling = Array.from({ length: 17 }, (_, i) => ({
      auth_id: 'route-' + i, action_id: 'action-' + i, state: 'disabled',
      upstream_condition: 'unauthorized', email: 'frozen-' + i + '@example.test',
    }));
    const multi = { ...rule('old', '兼容模型巡检', { at: stamp, matched: 0, skipped: 7 }),
      models: ['gpt-6.1-sol', ...Array.from({ length: 7 }, (_, i) => 'secondary-model-' + i)],
      timer_active: true, next_run_local: '2026-11-21T10:30:00+08:00', cron: '*/30 * * * *' };
    const long = { ...rule('long', '较长名称的定向检测规则'.repeat(5)), enabled: false,
      object_types: ['oauth','provider'], target_scope: 'single', auth_id: 'fixture-' + 'target'.repeat(20) + '@example.test',
      models: ['long-model-' + 'suffix'.repeat(28)], action: 'disable_credential', auto_restore: true };
    const rules = [first, rule('b', '备用目录'), multi, long];
    let failRules = false;
    await page.route('**/*', async route => {
      const req = route.request(), u = new URL(req.url()); assert.equal(u.origin, origin);
      const api = u.pathname.slice('/v0/management/plugins/model-quality'.length);
      if (req.method() === 'GET') {
        if (api === '/rules') return failRules ? route.fulfill({ status: 503, json: { error: 'temporary rule list failure' } }) : route.fulfill({ json: { items: rules, total: rules.length, offset: 0 } });
        return route.continue();
      }
      if (api === '/rules/scope-preview' && req.method() === 'POST') return route.continue();
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
    const context = el('#ops-activity #ops-rule-context');
    const dispatch = id => context.locator('details[data-skip-rule="' + id + '"]');
    const selectRule = async id => {
      const title = card(id).locator('[data-rule-select]');
      if (await title.getAttribute('aria-pressed') !== 'true') await title.click();
      await card(id).locator('[data-rule-select][aria-pressed=true]').waitFor();
    };
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
    assert(await context.isHidden());
    await card('a').locator('[data-rule-select]').focus(); await page.keyboard.press('Enter');
    await context.waitFor();
    assert.equal(await el('#ops-scope-label').innerText(), first.name);
    if (out) {
      await page.selectOption('#theme-toggle', 'white');
      await el('#rules-title').click();
      await page.mouse.move(4, 4);
      const clip = await card('a').evaluate(card => {
        const box = card.closest('#ops-rules').getBoundingClientRect(), bottom = card.getBoundingClientRect().bottom;
        return { x: Math.floor(box.left + scrollX), y: Math.floor(box.top + scrollY),
          width: Math.ceil(box.width), height: Math.ceil(bottom - box.top) + 6 };
      });
      assert.equal(await card('a').locator(':focus-visible').count(), 0);
      await page.screenshot({ path: path.join(out, 'rule-card-preview.png'), clip, animations: 'disabled' });
    }
    for (const [width, height, theme] of [[1440,900,'light'],[1064,760,'light'],[390,844,'dark'],[320,710,'white']]) {
      await page.setViewportSize({ width, height }); await page.selectOption('#theme-toggle', theme);
      const list = el('#rules-list');
      assert.equal(await list.locator('details,time[datetime="' + stamp + '"],[data-rule-batch],[data-run],[data-handling-ack]').count(), 0);
      assert.doesNotMatch(await list.textContent(), /最近调度|未执行 3 个|已在 CPA 手动变更|frozen-\d+@example\.test|凭证未注册全部所选模型/);
      assert.doesNotMatch(await card('old').innerText(), /secondary-model/);
      assert.match(await card('old').innerText(), /gpt-6\.1-sol/);
      assert.match(await card('old').innerText(), /等 8 个/);
      assert.match(await card('old').innerText(), /定时开启/);
      assert.match(await card('a').innerText(), /gpt-6\.2-sol/);
      assert.match(await card('a').innerText(), /仅手动/);
      assert.match(await card('long').innerText(), /已停止/);
      assert(await card('long').locator('[data-rule-run]').isDisabled());
      for (const action of ['edit','delete','run']) assert(await card('a').locator('[data-rule-' + action + ']').isEnabled());
      const layout = await list.evaluate(node => {
        const cards = [...node.querySelectorAll('article')];
        const buttonFits = (btn, box) => {
          const r = btn.getBoundingClientRect();
          return r.width > 0 && r.height > 0 && r.left >= box.left - 1 && r.right <= box.right + 1 && r.top >= box.top - 1 && r.bottom <= box.bottom + 1;
        };
        return { heights: cards.map(card => card.getBoundingClientRect().height),
          fits: cards.every(card => card.scrollWidth <= card.clientWidth + 1 &&
            [...card.querySelectorAll('button')].every(btn => buttonFits(btn, card.getBoundingClientRect()))),
          pageFits: document.documentElement.scrollWidth <= innerWidth + 1 };
      });
      assert(layout.fits && layout.pageFits, JSON.stringify({ width, ...layout }));
      assert(Math.abs(layout.heights[0] - layout.heights[1]) < 1, 'History must not change card height');
      captures.push({ width, theme, kind: 'rule-list', ...layout });
      if (out) await list.screenshot({ path: path.join(out, 'rules-' + width + '.png'), animations: 'disabled' });
    }
    const handling = context.locator('[data-rule-handling]');
    await handling.locator(':scope > summary').click();
    const finalRow = handling.locator('.quality-handling-records > div').last();
    await finalRow.getByText(/frozen-16@example\.test/).waitFor();
    await finalRow.scrollIntoViewIfNeeded();
    assert(await finalRow.evaluate(node => {
      const rect = node.getBoundingClientRect(), pane = node.closest('.quality-activity-notes');
      return pane.scrollTop > 0 && pane.scrollHeight > pane.clientHeight && rect.bottom <= pane.getBoundingClientRect().bottom + 1;
    }));
    assert.equal(await card('a').locator('.quality-handling-records').count(), 0);
    await handling.locator(':scope > summary').click();
    await page.setViewportSize({ width: 1440, height: 900 });
    assert.equal(await dispatch('a').locator('li').count(), 0);
    await dispatch('a').locator('summary').focus(); await page.keyboard.press('Enter'); await dispatch('a').locator('li').first().waitFor();
    assert.match(await dispatch('a').innerText(), /检测 0 个 · 未执行 3 个/);
    assert.equal(await dispatch('a').locator('time').getAttribute('datetime'), stamp);
    await checkThree(dispatch('a'));
    await execute('b');
    assert.match(await receipt.locator('summary').innerText(), /手动执行回执 · 备用目录.*检测 0 个 · 未执行 3 个/s);
    await checkThree(receipt);
    assert.equal(await el('#ops-scope-label').innerText(), first.name);
    assert.equal(await dispatch('b').count(), 0); assert(!(await receipt.innerText()).includes('file-d.json'));
    await el('#rule-search').fill('主模型'); await dispatch('a').locator('summary').focus();
    const before = await el('#rules-list').evaluate(n => n.scrollTop);
    await page.click('#refresh'); await page.waitForFunction(() => !document.querySelector('#refresh').hasAttribute('aria-busy'));
    assert.equal(await el('#rule-search').inputValue(), '主模型'); assert(await dispatch('a').evaluate(n => n.open));
    assert.equal(await el('#rules-list').evaluate(n => n.scrollTop), before); await checkThree(dispatch('a')); await checkThree(receipt);
    for (const [width, height, theme] of [[1440,900,'light'],[390,844,'dark'],[320,710,'white']]) {
      await page.setViewportSize({ width, height }); await page.selectOption('#theme-toggle', theme);
      for (const [kind, surface] of [['selected-dispatch', dispatch('a')], ['receipt', receipt]]) {
        await surface.scrollIntoViewIfNeeded();
        const geometry = await surface.evaluate(surface => {
          const list = surface.querySelector('ul'), viewport = list.getBoundingClientRect();
          return { noOverflow: document.documentElement.scrollWidth <= innerWidth && list.scrollWidth <= list.clientWidth + 1, rowsFit: [...list.children].every(row => {
            const r = row.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.left >= viewport.left - 1 && r.right <= viewport.right + 1;
          }) };
        });
        assert(geometry.noOverflow && geometry.rowsFit, JSON.stringify({ width, kind, geometry }));
        for (const row of await surface.locator('li').all()) {
          await row.scrollIntoViewIfNeeded();
          assert(await row.evaluate(node => {
            const rect = node.getBoundingClientRect(), hit = document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
            return hit && node.contains(hit);
          }), 'Every row must be reachable by scrolling the records panel');
        }
        captures.push({ width, kind, ...geometry });
        if (out) await page.screenshot({ path: path.join(out, kind + '-' + width + '.png'), animations: 'disabled' });
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 }); await el('#rule-search').fill('');
    await selectRule('b'); assert.equal(await dispatch('a').count(), 0); assert(await receipt.isHidden());
    await dispatch('b').locator('summary').click(); await dispatch('b').locator('li').filter({ hasText: 'file-d.json' }).waitFor();
    assert(!(await dispatch('b').innerText()).includes('file-a.json'));
    await selectRule('old');
    await dispatch('old').locator('summary').click(); await dispatch('old').getByText('本轮未记录逐个对象的明细。').waitFor();
    assert.equal(await dispatch('old').locator('li').count(), 0); await execute('old');
    assert.match(await receipt.innerText(), /未执行 7 个/); assert.equal(await receipt.locator('li').count(), 0);
    failRules = true; await execute('a');
    assert.match(await receipt.innerText(), /手动执行回执 · 主模型巡检.*检测 2 个 · 未执行 3 个/s); await checkThree(receipt);
    assert(await el('#ops-batch-bar').isVisible()); assert(await el('#rules-read-note').isVisible());
    assert.deepEqual(writes.map(w => w.id), ['b', 'old', 'a']); assert.deepEqual(errors, []);
    if (out) await fs.writeFile(path.join(out, 'scope-skips.json'), JSON.stringify({ sample: snapshot, captures, writes, readback: 'Selected-rule history and frozen manual receipts remain independent.', oldAggregate: 7, confirmedBatch: 'frozen-batch', modelCalls: 0, credentialWrites: 0 }, null, 2) + '\n');
    console.log('Configuration-only rule cards, selected-rule history, retained receipts, scheduled updates and legacy totals passed.');
  } finally { await browser?.close(); fixture.kill(); await exit; }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
