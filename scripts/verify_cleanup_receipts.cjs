// Drive the actual confirmation dialogs against isolated, in-memory cleanup writes.
const assert = require('node:assert/strict');
const { once } = require('node:events');
const { spawn } = require('node:child_process');
const { createInterface } = require('node:readline');
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

function gate() {
  let release;
  const promise = new Promise(resolve => { release = resolve; });
  return { promise, release };
}

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exited = once(fixture, 'exit');
  let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key } = JSON.parse(line), { origin, hostname, port } = new URL(url);
    assert.equal(hostname, '127.0.0.1'); assert.notEqual(port, '8317');
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const out = process.env.UI_OUTPUT, report = { source: {}, scenarios: {} };
    if (out) await fs.mkdir(out, { recursive: true });
    for (const file of ['app.js', 'index.html', 'client.js', 'view.js', 'style.css']) {
      report.source[file] = createHash('sha256').update(await fs.readFile(path.join(__dirname, '../plugin/ui', file))).digest('hex');
    }
    const observed = process.argv.includes('--observe');
    const prefix = '/v0/management/plugins/model-quality', a = 'rule-a', b = 'rule-b';
    async function scenario(name, interact, { width = 1440, height = 900, theme = 'light' } = {}) {
      const context = await browser.newContext({ viewport: { width, height }, reducedMotion: 'reduce' });
      try {
        const page = await context.newPage(); page.setDefaultTimeout(6000);
        const rules = [a, b].map((id, index) => ({ id, name: ['周一观察', '周二观察'][index], revision: 3, target_scope: 'all_enabled', enabled: true, timer_active: false, action: 'observe_only', models: ['fixture-reasoner'], samples: 1, question_id: 'logic-candy' }));
        const run = (id, rule, status = 'completed') => ({ id, source: rule ? 'scheduled_probe' : 'manual_test', created_at: '2026-11-20T02:00:00Z', status, samples: [], config: { rule_id: rule, auth_id: 'fixture-quality-primary', models: ['fixture-reasoner'], samples: 1, question_id: 'logic-candy' } });
        const runs = new Map([run('manual', ''), run('other-manual', ''), run('first', a), run('second', b), run('in-progress', b, 'running'), run('protected', b)].map(item => [item.id, item]));
        const writes = [], reads = [], errors = [], control = {};
        page.on('pageerror', error => errors.push(error.message));
        await context.route('**/*', async route => {
          const req = route.request(), u = new URL(req.url());
          assert.equal(u.origin, origin);
          const p = u.pathname.slice(prefix.length);
          if (!u.pathname.startsWith(prefix + '/')) return route.continue();
          const slice = rows => {
            const offset = Number(u.searchParams.get('offset') || 0), limit = Number(u.searchParams.get('limit') || 20);
            return route.fulfill({ json: { items: rows.slice(offset, offset + limit), offset, limit, total: rows.length } });
          };
          if (req.method() === 'GET') {
            reads.push(p + u.search);
            if (p === '/metrics' && control.metricsFail) return route.fulfill({ status: 503, json: { error: 'offline metrics' } });
            if (p === '/runs') return slice([...runs.values()].filter(r => r.source === u.searchParams.get('source') && (!u.searchParams.get('rule_id') || u.searchParams.get('rule_id') === r.config.rule_id)));
            if (p === '/rules') return slice(rules);
            if (p === '/runs/detail') {
              const item = runs.get(u.searchParams.get('id'));
              if (control.holdID && item?.id === control.holdID) {
                control.entered.release();
                await control.resume.promise;
              }
              return item ? route.fulfill({ json: item }) : route.fulfill({ status: 404, json: { error: { code: 'not_found', message: 'missing run' } } });
            }
            return route.continue();
          }
          assert.equal(req.method(), 'DELETE');
          const body = req.postData() ? req.postDataJSON() : null;
          writes.push({ p, body });
          let receipt = { ok: true };
          if (p === '/rules/run-history') {
            const drop = [...runs.values()].filter(r => r.source === 'scheduled_probe' && r.status === 'completed' && r.id !== 'protected');
            for (const r of drop) runs.delete(r.id);
            receipt = { deleted: drop.length, kept: 2 };
            if (writes.length === 1) runs.set('finished-later', run('finished-later', b));
          } else if (p === '/rules') {
            if (control.conflict) return route.fulfill({ status: 409, json: { error: { code: 'conflict', message: '规则仍有关联的运行记录' } } });
            const index = rules.findIndex(rule => rule.id === body.id && rule.revision === body.revision);
            if (index < 0) return route.fulfill({ status: 404, json: { error: { code: 'not_found', message: 'missing rule' } } });
            rules.splice(index, 1);
            for (const [id, r] of runs) if (r.config.rule_id === body.id) runs.delete(id);
          } else if (p === '/runs') runs.delete(body.id);
          else throw Error('Unexpected fixture write ' + p);
          if (control.failReceipt) { control.failReceipt = false; return route.abort('connectionfailed'); }
          if (control.badReceipt) return route.fulfill({ json: { detail: true } });
          return route.fulfill({ json: receipt });
        });
        await page.goto(url); await page.selectOption('#theme-toggle', theme);
        await page.click('#connection-open'); await page.fill('#management-key', management_key);
        await page.click('#connection-form button[type=submit]'); await page.waitForSelector('body.connected');
        await page.click('[data-page=operations]');
        await page.click('[data-rule-select=rule-a]');
        const confirm = async () => {
          await page.click('#action-confirm');
          await page.waitForFunction(() => !document.querySelector('#action-confirm').hasAttribute('aria-busy'));
        };
        const snapshot = () => page.evaluate(() => ({
          dialog: document.querySelector('#action-dialog').open,
          error: document.querySelector('#action-error:not(.hidden)')?.textContent || '',
          notice: document.querySelector('#notice:not(.hidden)')?.textContent || '',
          visibleNotice: document.querySelector('#notice').getBoundingClientRect().bottom > 0 && document.querySelector('#notice').getBoundingClientRect().top < innerHeight && document.querySelector('#notice').getClientRects().length > 0,
          title: document.querySelector('#action-title').textContent,
          scope: document.querySelector('#ops-scope-label').textContent,
          activity: [...document.querySelectorAll('#operations-history [data-run]')].map(n => n.dataset.run),
          history: [...document.querySelectorAll('#history-list [data-run]')].map(n => n.dataset.run),
          focus: document.activeElement?.id || '',
          selected: document.querySelector('#run-frozen-data .run-identifier')?.textContent || '',
          overflow: document.documentElement.scrollWidth > innerWidth,
        }));
        const screenshot = suffix => out && page.screenshot({ path: path.join(out, name + '-' + suffix + '.png'), animations: 'disabled' });
        const result = await interact({ page, control, confirm, snapshot, screenshot, reads });
        assert.deepEqual(errors, []);
        report.scenarios[name] = { ...result, writes, remaining: [...runs.keys()] };
      } finally { await context.close(); }
    }
    await scenario('clear', async ({ page, control, confirm, snapshot, screenshot }) => {
      await page.click('#ops-history-clear');
      const description = await page.locator('#action-description').innerText();
      await screenshot('confirm');
      control.failReceipt = true;
      await confirm(); const lost = await snapshot();
      await screenshot('receipt');
      if (lost.dialog) await confirm();
      return { description, lost, end: await snapshot() };
    });
    await scenario('rule', async ({ page, control, confirm, snapshot, screenshot }) => {
      await page.click('[data-rule-delete=rule-a]'); control.metricsFail = true;
      await confirm(); const acknowledged = await snapshot();
      await screenshot('receipt');
      if (acknowledged.dialog) await confirm();
      return { acknowledged, end: await snapshot() };
    }, { width: 390, height: 844, theme: 'dark' });
    await scenario('manual', async ({ page, control, confirm, snapshot, screenshot }) => {
      await page.click('[data-page=manual]'); await page.click('[data-manual-view=history]');
      await page.click('[data-run=manual]'); await page.waitForSelector('#run-frozen-data .run-identifier', { state: 'attached' });
      await page.click('[data-manual-view=history]'); await page.click('[data-delete-run=manual]');
      control.failReceipt = true;
      await confirm(); const lost = await snapshot();
      await screenshot('receipt');
      if (lost.dialog) await confirm();
      return { lost, end: await snapshot() };
    }, { width: 1440, height: 650, theme: 'white' });
    await scenario('invalid_clear_receipt', async ({ page, control, confirm, snapshot }) => {
      await page.click('#ops-history-clear'); control.badReceipt = true;
      await confirm(); return { end: await snapshot() };
    }, { width: 320, height: 800 });
    await scenario('rule_conflict', async ({ page, control, confirm, snapshot }) => {
      await page.click('[data-rule-delete=rule-a]'); control.conflict = true;
      await confirm(); const rejected = await snapshot();
      await page.click('#action-cancel'); return { rejected, end: await snapshot() };
    });
    await scenario('other_selection', async ({ page, control, confirm, snapshot }) => {
      await page.click('[data-page=manual]'); await page.click('[data-manual-view=history]');
      control.holdID = 'other-manual'; control.entered = gate(); control.resume = gate();
      const late = page.waitForResponse(response => response.url().endsWith('/runs/detail?id=other-manual'));
      try {
        await page.click('[data-run=other-manual]'); await control.entered.promise;
        await page.click('[data-delete-run=manual]'); await confirm();
      } finally { control.resume.release(); }
      await (await late).finished();
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      return { end: await snapshot() };
    });
    if (out) await fs.writeFile(path.join(out, 'cleanup-receipts.json'), JSON.stringify(report, null, 2) + '\n');
    if (observed) return;
    const { clear, rule, manual, invalid_clear_receipt: invalid, rule_conflict: conflict } = report.scenarios;
    assert.match(clear.description, /所有规则/);
    assert.equal(clear.writes.length, 1); assert(!clear.lost.dialog && !clear.lost.overflow);
    assert.deepEqual(clear.lost.activity.sort(), ['finished-later', 'in-progress', 'protected']);
    assert.deepEqual(clear.remaining.sort(), ['finished-later', 'in-progress', 'manual', 'other-manual', 'protected']);
    assert.match(clear.lost.notice, /回执|未确认/);
    assert.equal(rule.writes.length, 1); assert(!rule.acknowledged.dialog && !rule.acknowledged.overflow);
    assert.match(rule.acknowledged.notice, /已删除/); assert.equal(rule.acknowledged.scope, '全部规则'); assert(rule.acknowledged.visibleNotice);
    assert.deepEqual(manual.writes, [{ p: '/runs', body: { id: 'manual' } }]);
    assert(!manual.lost.dialog && !manual.lost.selected && !manual.lost.overflow); assert.deepEqual(manual.lost.history, ['other-manual']);
    assert.match(manual.lost.notice, /回执|未确认|查无|查不到/);
    assert(!invalid.end.dialog && !invalid.end.notice.includes('undefined') && !invalid.end.overflow); assert.match(invalid.end.notice, /回执|未确认/);
    assert(conflict.rejected.dialog && /conflict/.test(conflict.rejected.error) && conflict.remaining.includes('first'));
    assert.equal(report.scenarios.other_selection.end.selected, '任务 other-manual');
    console.log('Cleanup scope, lost and malformed receipts, read failure after confirmed deletion, retained rows and manual selection passed.');
  } finally { await browser?.close(); fixture.kill(); await exited; }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
