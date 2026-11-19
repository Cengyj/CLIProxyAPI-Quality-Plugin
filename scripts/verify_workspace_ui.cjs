// Exercise the embedded UI with the in-memory fixture, never a supplied server.
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const checkWorkspace = require('./quality_workspace_layout.cjs');
const checkManual = require('./manual_workspace_layout.cjs');
const checkAccountSurface = require('./account_surface_layout.cjs');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

const html = fs.readFileSync(path.join(__dirname, '../plugin/ui/index.html'), 'utf8');
const { ui_version: version } = JSON.parse(html.match(/<script type="application\/json" id="build-info">(.*?)<\/script>/)[1]);
const out = path.resolve(process.env.UI_OUTPUT || path.join(__dirname, `../artifacts/ui-rework-${version}/browser`));
const workspaces = ['login', 'manual', 'operations'];
const report = { ui_version: version, fixture: 'synthetic, in memory', checks: [], layouts: [], manual_layouts: [], account_surfaces: [], screenshots: [] };

async function run() {
  fs.mkdirSync(out, { recursive: true });
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exit = once(fixture, 'exit');
  let browser;
  try {
    const lines = createInterface({ input: fixture.stdout });
    const [line] = await Promise.race([once(lines, 'line'), exit.then(() => { throw Error('Fixture did not start'); })]);
    const { url, management_key: key } = JSON.parse(line);
    const { origin } = new URL(url);
    browser = await chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' });
    const errors = [], writes = [], external = [], accountDetailReads = [];
    await context.route('**/*', route => {
      const req = route.request(), u = new URL(req.url());
      if (u.origin !== origin) { external.push(u.origin); return route.abort(); }
      if (req.method() !== 'GET') { writes.push(`${req.method()} ${u.pathname}`); return route.abort(); }
      if (u.pathname.endsWith('/login/accounts/detail')) accountDetailReads.push(u.searchParams.get('id'));
      return route.continue();
    });
    const page = await context.newPage();
    page.setDefaultTimeout(7000);
    page.on('pageerror', e => errors.push(e.message));
    const find = selector => page.locator(selector);
    const click = selector => find(selector).click();
    const visible = (selector, yes = true) => find(selector).first().waitFor({ state: yes ? 'visible' : 'hidden' });
    const settle = target => target.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    const widthCheck = async (target, label) => {
      const sizes = await target.evaluate(() => ({ doc: document.documentElement.scrollWidth, client: document.documentElement.clientWidth, viewport: innerWidth }));
      assert(sizes.doc <= sizes.viewport, `${label}: ${JSON.stringify(sizes)}`);
      const nav = await target.locator('.primary-nav button').evaluateAll(buttons => buttons.map(button => {
        const { x, y, right } = button.getBoundingClientRect(); return { x, y, right };
      }));
      assert.equal(nav.length, 3);
      assert(nav.every((item, i) => item.y === nav[0].y && (!i || item.x >= nav[i - 1].right) && item.right <= sizes.client + 1), `Horizontal workspace navigation: ${label}`);
      if (await target.locator('#account-table-wrap').isVisible()) {
        const clipped = await target.locator('.account-action-label').evaluateAll(labels => labels.filter(el => el.scrollWidth > el.clientWidth + 1).map(el => el.textContent));
        assert.deepEqual(clipped, [], `Account action labels: ${label}`);
        const geometry = await target.locator('.account-row').first().evaluate(row => {
          const actions = [...row.querySelectorAll('.account-row-actions button')].map(b => b.getBoundingClientRect());
          return { firstRowY: Math.round(row.getBoundingClientRect().top + scrollY), rowHeight: row.offsetHeight,
            actionHeight: Math.min(...actions.map(b => b.height)), aligned: actions.length === 6 && actions.slice(0,3).every((b,i) => Math.abs(b.x-actions[i+3].x) < 1 && b.bottom <= actions[i+3].y) };
        });
        assert(geometry.aligned, `Two aligned rows of actions: ${label}`);
        if (sizes.viewport <= 600) assert(geometry.actionHeight >= 44, `Phone hit targets: ${label}`);
        if (sizes.viewport > 800) assert(geometry.rowHeight <= 90, `Compact desktop row: ${label}`);
        Object.assign(sizes, geometry);
      }
      if (await target.locator('#metrics').isVisible()) {
        const grid = await target.locator('#metrics').evaluate(el => getComputedStyle(el).gridTemplateColumns.split(' ').length);
        assert.equal(grid, sizes.viewport > 800 ? 4 : 2, `Metric columns: ${label}`);
      }
      report.layouts.push({ label, ...sizes });
    };
    const shot = async (name, surface = page) => {
      await page.evaluate(() => scrollTo(0, 0)); await settle(page);
      await surface.screenshot({ path: path.join(out, `${name}.png`), animations: 'disabled' });
      report.screenshots.push(`${name}.png`);
    };
    await page.goto(origin);
    await visible('#login-disconnected');
    assert.equal(await find('[data-page][aria-current=page]').getAttribute('data-page'), 'login');
    await click('#login-connect'); assert.equal(await find('#connection-build').innerText(), `UI ${version}`);
    await find('#management-key').fill(key);
    await click('#connection-form button[type=submit]'); await visible('#account-import-open:enabled');
    await visible('[data-account-row="fixture-enabled"]');
    const rowAction = (id, action) => `[data-account-row="${id}"] [data-account-action="${action}"]`;
    const more = id => rowAction(id, 'more');
    await shot('accounts-desktop');
    await shot('account-actions-desktop', find('[data-account-row="fixture-enabled"] .account-row-actions'));
    await shot('account-actions-disabled', find('[data-account-row="fixture-uncertain"] .account-row-actions'));
    await page.setViewportSize({ width: 1280, height: 820 }); await shot('accounts-reference-size');
    await page.setViewportSize({ width: 1440, height: 900 });
    const filterBounds = await find('.account-status-filter').boundingBox(), searchBounds = await find('.account-search').boundingBox();
    assert(Math.abs(filterBounds.y - searchBounds.y) < 2 && filterBounds.x + filterBounds.width < searchBounds.x);
    await find('#account-status-filter').selectOption('attention');
    assert.equal(await find('.account-row').count(), Number((await find('#account-status-filter option:checked').innerText()).split('·')[1]));
    assert.equal(await find('#login-workbench [role=tablist],#login-workbench [role=tabpanel]').count(),0);
    await find('#account-search').fill('not-a-fixture'); await visible('#account-no-results');
    await click('#account-clear-filter'); await visible('[data-account-row="fixture-enabled"]');
    for (const action of ['start', 'reset_totp', 'delete']) assert(await find(rowAction('fixture-uncertain', action)).isDisabled(), action);
    await click(more('fixture-uncertain')); assert(await find('#account-logout-all').isDisabled());
    await page.keyboard.press('Escape');
    assert(await find(more('fixture-uncertain')).evaluate(el => document.activeElement === el));
    await click(rowAction('fixture-enabled', 'reset_totp')); await visible('#account-security-dialog');
    await click('#account-security-cancel');
    assert(await find(rowAction('fixture-enabled', 'reset_totp')).evaluate(el => document.activeElement === el));
    await click(rowAction('fixture-enabled', 'delete')); await visible('#account-delete-confirm');
    assert(await find('#account-editor-dialog').isHidden());
    assert(await find('#account-delete-no').evaluate(el => el === document.activeElement));
    await click('#account-delete-no'); await visible('#account-delete-confirm', false);
    assert(await find('#account-editor-dialog').isHidden());
    assert.deepEqual(accountDetailReads, []); assert.deepEqual(writes, []);
    assert(await find(rowAction('fixture-enabled', 'delete')).evaluate(el => document.activeElement === el));
    await click('#account-import-open'); await visible('#account-import-dialog'); await click('#account-import-cancel');
    await click('[data-account-row="fixture-failed"] .account-logs-action'); await visible('.account-log-row');
    await click('[data-account-row="fixture-failed"] .account-logs-action');
    report.checks.push('Directory filters, empty search, safe import dismissal, inline logs, locked security state, standalone delete cancellation with no editor, detail read or write, and keyboard focus restoration');
    await click('#account-history-open'); await find('#login-task-search').fill('fixture-job-failed');
    await click('[data-login-job="fixture-job-failed"]'); await visible('.login-task-log-viewport .account-log-entry');
    await click('[data-task-log-action=errors]'); await visible('.login-task-log-viewport .account-log-entry');
    await shot('task-diagnostics');
    await page.keyboard.press('Escape');
    assert(await find('#account-history-open').evaluate(n=>document.activeElement===n));
    await click('#account-settings-open'); await visible('#account-upload-plans'); await shot('login-settings');
    const savedProxy=await find('#account-default-proxy').inputValue();
    await find('#account-default-proxy').fill('direct');
    await page.keyboard.press('Escape');
    assert(await find('#account-settings-open').evaluate(n=>document.activeElement===n));
    assert(await find('#account-settings-draft').isVisible());
    await click('#account-settings-open'); assert.equal(await find('#account-default-proxy').inputValue(),'direct');
    await find('#account-default-proxy').fill(savedProxy); await page.keyboard.press('Escape');
    report.checks.push('Task history and diagnostic filtering; settings and upload exclusions remain accessible');
    await click('[data-page=manual]'); await visible('#run-form'); assert(await find('#run-submit').isDisabled());
    report.manual_layouts.push({state:'empty',...await checkManual(find('.manual-workbench'))});
    await shot('manual-empty-desktop');
    await find('.skip-link').focus(); await page.keyboard.press('Enter');
    assert.equal(new URL(page.url()).hash, '#manual'); assert.equal(await page.evaluate(() => document.activeElement.id), 'main');
    await click('[data-manual-view=history]');
    report.manual_layouts.push({state:'history',...await checkManual(find('.manual-workbench'))});
    const historyFooter = await find('.answer-footer').boundingBox();
    await find('#manual-history').evaluate(node => node.scrollTop = node.scrollHeight);
    assert(await find('#manual-history').evaluate(node => node.scrollTop > 0));
    assert.deepEqual(await find('.answer-footer').boundingBox(),historyFooter);
    await find('#history-next').click(); await find('#history-page').filter({hasText:'第 2 页'}).waitFor();
    assert.equal(await find('#history-page').innerText(),'第 2 页');
    await find('#history-prev').click(); await find('#history-page').filter({hasText:'第 1 页'}).waitFor();
    await click('#history-list [data-run="fixture-run-complete"]');
    await visible('#sample-list .answer-text'); await shot('manual-desktop');
    report.manual_layouts.push({state:'answers',...await checkManual(find('.manual-workbench'))});
    const compareRows = () => find('#sample-list .sample').evaluateAll(nodes => nodes.slice(0, 2).map(n => n.getBoundingClientRect().top));
    const wide = await compareRows(); assert.equal(wide.length, 2); assert(Math.abs(wide[0] - wide[1]) < 1);
    await page.setViewportSize({ width: 960, height: 850 }); await settle(page);
    const narrow = await compareRows(); assert(narrow[1] > narrow[0] + 100, 'Narrow reading column stacks samples');
    await click('#reading-toggle'); await settle(page);
    report.manual_layouts.push({state:'expanded',...await checkManual(find('.manual-workbench'))});
    const expanded = await compareRows(); assert(Math.abs(expanded[0] - expanded[1]) < 1, 'Expanded reading restores side-by-side comparison');
    await click('#reading-toggle'); await page.setViewportSize({ width: 1440, height: 900 });
    await find('#prompt').fill('draft to preserve'); await click('#reading-toggle'); await visible('#run-form', false);
    await click('#reading-toggle'); assert.equal(await find('#prompt').inputValue(), 'draft to preserve');
    await page.setViewportSize({width:1280,height:650}); await settle(page);
    const setupFooter = await find('.setup-footer').boundingBox();
    await find('#run-form').evaluate(node => node.scrollTop = node.scrollHeight);
    assert(await find('#run-form').evaluate(node => node.scrollTop > 0));
    assert.deepEqual(await find('.setup-footer').boundingBox(),setupFooter);
    report.manual_layouts.push({state:'short-screen',...await checkManual(find('.manual-workbench'))});
    await page.setViewportSize({width:1440,height:900});
    await click('[data-page=operations]'); await visible('.metric');
    assert.equal(await find('#ops-attention, .quality-disclosure, .capability-note, #targets-list, #actions-list, #operations-audit, #metrics-detail').count(), 0);
    assert(await find('#operations-main > :last-child').evaluate(node => node.classList.contains('quality-workspace')), 'Quality workspace ends after rules and run history');
    assert.equal(await find('.metric-value').first().innerText(), '—');
    assert(await find('.metric-value').first().evaluate(el => el.hasAttribute('data-missing')));
    assert.equal(await find('.metric-value').nth(1).innerText(), '0.0%');
    assert.equal(await find('.metric-unit').innerText(), 'token');
    assert.equal(await find('.metric-label').nth(2).innerText(), '结论未知');
    assert.equal(await find('[data-action-preview]').count(), 0); await shot('quality-desktop');
    await checkWorkspace(find('.quality-workspace'));
    await click('#rule-new'); await visible('#rule-dialog');
    assert(await find('#rule-scope-all').isChecked()); assert(await find('#rule-target-field').isHidden());
    assert.equal(await find('#rule-max-tokens, #settings-open, #settings-dialog').count(), 0);
    await find('#rule-scope-single').check(); await visible('#rule-target-field'); await find('#rule-scope-all').check();
    await click('#rule-dialog .dialog-heading [data-close]');
    await visible('#ops-activity');
    assert.equal(await find('#rule-schedule-toggle,[data-rule-pause]').count(),0);

    report.checks.push('Read sample answers and history, preserve draft through expanded reading, render unknown results distinctly, remove quality support sections and footer without changing the main workspace');
    for (const width of [320, 390, 768, 960, 1064, 1440]) {
      await page.setViewportSize({ width, height: 850 });
      for (const theme of ['light', 'white', 'dark']) {
        await find('#theme-toggle').selectOption(theme);
        for (const area of workspaces) {
          await click(`[data-page=${area}]`);
          if (area === 'login') { await visible('.account-row'); }
          await settle(page); await widthCheck(page, `${width}/${theme}/${area}`);
          if (area === 'manual') report.manual_layouts.push({state:'answers',theme,...await checkManual(find('.manual-workbench'))});
          if (area === 'login') for (const surface of ['history','settings']) {
            await click(`#account-${surface}-open`);
            report.account_surfaces.push({theme,...await checkAccountSurface(find(`#login-${surface}-dialog`))});
            if (width===1440 && theme==='light' || width===390 && theme==='dark' || width===320 && theme==='white') await shot(`account-${surface}-${width}-${theme}`);
            await page.keyboard.press('Escape');
          }
          if (width === 390 && theme === 'dark' && area === 'login') {
            await shot('accounts-mobile-dark');
            await shot('account-actions-mobile-dark', find('[data-account-row="fixture-enabled"] .account-row-actions'));
          }
        }
      }
    }
    await page.setViewportSize({ width: 320, height: 750 });
    await click('[data-page=manual]'); await click('[data-manual-pane=setup]'); assert.equal(await find('#prompt').inputValue(), 'draft to preserve');
    await click('[data-manual-pane=history]'); await visible('#manual-history'); await visible('#run-form', false);
    await click('#history-list [data-run="fixture-run-complete"]'); await visible('#manual-results');
    assert.equal(await find('[data-manual-pane=results]').getAttribute('aria-pressed'), 'true');
    await click('#task-actions summary'); await visible('#run-export');
    await page.keyboard.press('Escape'); assert(await find('#run-export').isHidden());
    await shot('manual-answers-320');
    await click('[data-manual-pane=setup]'); await visible('#run-form');
    assert.equal(await find('#prompt').inputValue(), 'draft to preserve');
    report.checks.push('Six viewport widths × three themes × three workspaces, and 320px execution setup/history/answers with draft retention');
    await click('[data-page=login]'); await visible('#account-import-open:enabled');
    await click('#account-import-open'); await visible('#account-import-dialog');
    assert(await find('#account-import-dialog').evaluate(el => el.scrollWidth <= el.clientWidth));
    await click('#account-import-cancel'); await click(more('fixture-enabled')); await click('#account-logout-all');
    await visible('#account-security-dialog'); assert(await find('#account-security-dialog').evaluate(el => el.scrollWidth <= el.clientWidth));
    await click('#account-security-cancel');
    assert(await find(more('fixture-enabled')).evaluate(el => document.activeElement === el));
    await page.route('**/model-quality/login/accounts', route => route.fulfill({ status: 503, json: { code: 'storage_unavailable' } }));
    await click('#refresh'); await visible('#account-list-summary'); assert(await find('.account-row').count() > 0);
    assert.equal(await find('#login-sync-status').count(), 0);
    assert(await find('#account-import-open').isDisabled()); await page.unroute('**/model-quality/login/accounts');
    await click('#refresh'); await visible('#account-import-open:enabled'); await visible('#account-list-summary', false);
    report.checks.push('Keyboard skip preserves the route; 320px import and security dialogs fit; failed reads retain the directory and block writes until explicit refresh succeeds');
    await page.goto(`${origin}/embedded.html?page=login&theme=dark`);
    const frame = page.frameLocator('#fixture-plugin'); await frame.locator('body.connected').waitFor();
    const inner = await (await find('#fixture-plugin').elementHandle()).contentFrame();
    await require('./check_security_dialog.cjs')({ page, frame: inner, origin, out, report });
    await find('#fixture-theme').selectOption('dark'); await frame.locator('html[data-theme=dark]').waitFor();
    for (const width of [320, 390, 768, 1064, 1440]) {
      await page.setViewportSize({ width, height: 850 });
      await frame.locator('.account-row').first().waitFor();
      await widthCheck(inner, `embedded/${width}`);
      assert(await frame.locator('#theme-toggle').isHidden());
      assert.equal(await frame.locator('html').getAttribute('data-theme'), 'dark');
      if (width === 1064) await shot('accounts-embedded-dark');
    }
    for (const theme of ['white', 'light']) {
      await find('#fixture-theme').selectOption(theme); await frame.locator(`html[data-theme=${theme}][data-theme-source=host]`).waitFor();
      assert(await frame.locator('#theme-toggle').isHidden());
    }
    for (const width of [1064, 390, 320]) {
      await page.setViewportSize({ width, height: 850 });
      for (const area of workspaces) {
        await frame.locator(`[data-page=${area}]`).click(); await frame.locator(`#page-${area}`).waitFor();
        assert(await frame.locator('#connection-state').isVisible());
        if (area === 'manual' && width < 960) await frame.locator('[data-manual-pane=setup]').click();
        await inner.evaluate(() => scrollTo(0, 0)); await settle(inner);
        await widthCheck(inner, `embedded/${width}/light/${area}`); await shot(`embedded-${width}-${area}`, find('#fixture-plugin'));
        if (area === 'manual') report.manual_layouts.push({state:'embedded',...await checkManual(frame.locator('.manual-workbench'))});
      }
    }
    report.checks.push('All three workspaces retain horizontal navigation; the embedded shell follows the host theme; account actions are not truncated');
    assert.deepEqual(writes, []); assert.deepEqual(external, []); assert.deepEqual(errors, []);
    report.checks.push('No management write, external application request or uncaught page error');
    fs.writeFileSync(path.join(out, 'verification.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(`${report.layouts.length} layouts; ${report.screenshots.length} screenshots; all checks passed`);
  } finally {
    if (browser) await browser.close(); fixture.kill(); await exit;
  }
}
run().catch(error => { console.error(error); process.exitCode = 1; });
