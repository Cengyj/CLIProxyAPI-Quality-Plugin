// Use installed iframe assets, serve plugin data from a private fixture and prohibit all writes.
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const checkRuleLayout = require('./rule_editor_layout.cjs');
const checkWorkspace = require('./quality_workspace_layout.cjs');
const checkManual = require('./manual_workspace_layout.cjs');
const checkAccountSurface = require('./account_surface_layout.cjs');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const origin = 'http://127.0.0.1:8317';
const panel = origin + '/management.html#/plugin-pages/model-quality/0';
const prefix = '/v0/resource/plugins/model-quality/';
const preview = process.argv.includes('--preview');
const release = fs.realpathSync(process.argv[2]);
const capture = preview ? 'candidate' : 'installed';

async function main() {
  const key = fs.readFileSync(0, 'utf8').trim(); assert(key);
  const { ui_version, ui_source_sha256 } = JSON.parse(fs.readFileSync(path.join(release, 'plugin-package/build-manifest.json')));
  const allowed = new Set(['/favicon.ico', '/management.html', '/v8/management/config', '/v8/management/plugins', ...Object.keys(ui_source_sha256).map(name => prefix + name)]);
  const child = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exit = once(child, 'exit'); let browser;
  try {
    const [line] = await Promise.race([once(createInterface({ input: child.stdout }), 'line'), exit.then(() => { throw Error('Fixture failed'); })]);
    const fixture = JSON.parse(line), local = new URL(fixture.url);
    assert.equal(fixture.pid, child.pid); assert.equal(fixture.fixture, 'synthetic-plugin-ui'); assert.equal(local.hostname, '127.0.0.1'); assert.notEqual(local.origin, origin);
    browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, args: ['--disable-background-networking', '--no-proxy-server'] });
    const context = await browser.newContext({ locale: 'zh-CN', reducedMotion: 'reduce', viewport: { width: 1440, height: 900 }, acceptDownloads: false });
    context.setDefaultTimeout(10000);
    let fixtureReads = 0, detailReads = 0; const writes = [], checks = [], manual_checks = [], account_checks = [], served = {};
    await context.route('**/*', async route => {
      const req = route.request(), u = new URL(req.url());
      if (req.method() !== 'GET') { writes.push(req.method()); return route.abort(); }
      if (u.origin !== origin) return route.abort();
      if (u.pathname.startsWith(prefix) && allowed.has(u.pathname)) {
        const resource = preview ? local.origin + u.pathname : u.href;
        const result = await fetch(resource, { redirect: 'error' });
        const body = Buffer.from(await result.arrayBuffer());
        served[u.pathname.slice(prefix.length)] = createHash('sha256').update(body).digest('hex');
        return route.fulfill({status:result.status,contentType:result.headers.get('content-type'),body});
      }
      if (u.pathname.startsWith('/v0/management/plugins/model-quality/')) {
        if (u.pathname.endsWith('/accounts/detail')) detailReads++;
        if (req.headers().authorization !== 'Bearer ' + fixture.management_key) return route.abort();
        const result = await fetch(local.origin + u.pathname + u.search, { headers: { authorization: 'Bearer ' + fixture.management_key }, redirect: 'error' });
        fixtureReads++; return route.fulfill({ status: result.status, contentType: 'application/json', body: Buffer.from(await result.arrayBuffer()) });
      }
      return allowed.has(u.pathname) ? route.continue() : route.abort();
    });
    await context.routeWebSocket('**/*', socket => socket.close());
    const page = await context.newPage();
    assert.equal((await page.goto(panel, { waitUntil: 'domcontentloaded' })).status(), 200);
    await page.locator('input[name=cpa-management-key]').fill(key);
    const remember = page.getByRole('checkbox').first(); if (await remember.isVisible()) await remember.uncheck();
    const submit = page.locator('button[type=submit]').first();
    if (await submit.isVisible()) await submit.click(); else await page.getByRole('button', { name: /^(登\s*录|Sign In|Login|Connect)$/i }).click();
    await page.locator('.main-header, a[href="#/plugin-pages/model-quality/0"], iframe[src*="model-quality"]').first().waitFor({ state: 'attached' });
    if (page.url() !== panel) await page.locator('a[href="#/plugin-pages/model-quality/0"]').first().click();
    const iframe = page.locator('iframe[src*="model-quality"]').first(); await iframe.waitFor();
    const frame = await (await iframe.elementHandle()).contentFrame();
    assert.equal(new URL(frame.url()).pathname, prefix + 'index.html');
    assert.equal(JSON.parse(await frame.locator('#build-info').textContent()).ui_version, ui_version);
    await frame.locator('#login-connect').click(); await frame.locator('#management-key').fill(fixture.management_key);
    await frame.locator('#connection-form button[type=submit]').click();
    await frame.locator('body.connected').waitFor();
    await frame.locator('[data-page=operations]').click();
    assert.equal(await frame.locator('#ops-attention, .quality-disclosure, .capability-note, #targets-list, #actions-list, #operations-audit, #metrics-detail').count(), 0);
    assert(await frame.locator('#operations-main > :last-child').evaluate(node => node.classList.contains('quality-workspace')));
    const opener = frame.locator('#rule-new'), editor = frame.locator('#rule-dialog');
    for (const width of [1440, 390, 320]) {
      await page.setViewportSize({ width, height: width <= 390 ? 844 : 900 });
      if (width < 600) {
        if (await page.locator('.sidebar.open').count()) await page.locator('.mobile-menu-btn').click();
        await page.waitForFunction(() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0);
      }
      await page.locator('.page-transition--animating').waitFor({state:'detached'});
      const workspaceLayout = await checkWorkspace(frame.locator('.quality-workspace'));
      await page.screenshot({path:path.join(release, `${capture}-workspace-${width}.png`)});
      await opener.click(); await editor.waitFor();
      const back = frame.getByRole('button',{name:'关闭规则编辑器'});
      async function visibleInHost(control, label) {
        const rect = await control.boundingBox(); assert(rect, label);
        const clear = await page.evaluate(({x,y,width,height}) => [2,width/2,width-2].every(dx => document.elementFromPoint(x+dx,y+height/2)?.matches('iframe[src*="model-quality"]')),rect);
        if(!clear) { await page.screenshot({path:path.join(release,`${capture}-obscured-${width}.png`)}); console.log(JSON.stringify({width,label,rect,geometry:await editor.evaluate(n=>Object.fromEntries([n,n.querySelector('form'),n.querySelector('.rule-masthead'),n.querySelector('.rule-form-scroll'),n.querySelector('.editor-footer')].map((e,i)=>[i,{rect:e.getBoundingClientRect().toJSON(),scroll:e.scrollTop,scrollHeight:e.scrollHeight,clientHeight:e.clientHeight,margin:getComputedStyle(e).margin}]))),hit:await page.evaluate(({x,y,width,height})=>[2,width/2,width-2].map(dx=>{const n=document.elementFromPoint(x+dx,y+height/2);return n?{tag:n.tagName,class:n.className}:null;}),rect)})); }
        assert(clear, `CPA toolbar or viewport obscures ${label}`);
      }
      await visibleInHost(back, 'the back button');
      assert.equal(await editor.locator('.rule-mode').count(), 0);
      await back.click();await editor.waitFor({state:'hidden'});await opener.click();await editor.waitFor();
      assert(await frame.locator('#rule-scope-all').isChecked()); assert(await frame.locator('#rule-target-field').isHidden());
      assert.equal(await frame.locator('#rule-max-tokens,#settings-open,#settings-dialog').count(), 0);
      assert.match(await frame.locator('#rule-scope-note').textContent(), /暂无匹配凭证/);
      assert.deepEqual(await frame.locator('#rule-model-selection code').allTextContents(),['gpt-6-astra']);
      const radios = frame.getByRole('group',{name:'思考强度'}).getByRole('radio');
      assert.deepEqual(await radios.evaluateAll(o=>o.map(n=>n.value)),['low','medium','high','xhigh','max']);
      assert(await frame.getByRole('radio',{name:'高 high',exact:true}).isChecked());
      assert.match(await frame.locator('#rule-reasoning').getAttribute('aria-describedby'), /rule-effort-note/);
      assert.equal(await editor.locator('[role=tab],[role=tabpanel]').count(), 0);
      assert.equal(await editor.getByLabel('检测周期（五段 Cron，服务端时区）',{exact:true}).inputValue(),'*/30 * * * *');
      assert.equal(await editor.locator('#rule-estimate,#rule-sample-summary,#rule-cron-unit,#rule-cron-hint').count(),0);
      assert.doesNotMatch(await editor.innerText(),/预计|次主测|探测次数/);
      const layout = await checkRuleLayout(editor);
      const timer=editor.getByRole('switch',{name:'启用定时检测'});
      assert.equal(await editor.getByRole('switch').count(), 1);
      const save=editor.getByRole('button',{name:'保存规则',exact:true});
      await visibleInHost(save, 'the save button');
      assert(await save.evaluate(n=>{const r=n.getBoundingClientRect();return n.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2));}),'Save is occluded within the plugin');
      await page.screenshot({path:path.join(release, `${capture}-sheet-${width}.png`)});
      await checkRuleLayout.captureModelCard(page,frame.locator('.rule-instrument'),path.join(release, `${capture}-model-card-${width}.png`));
      await frame.getByRole('button',{name:'增加样本数'}).click();assert.equal(await frame.locator('#rule-samples').inputValue(),'2');
      await frame.getByRole('button',{name:'减少样本数'}).click();assert.equal(await frame.locator('#rule-samples').inputValue(),'1');
      assert(await frame.locator('#rule-target').isDisabled());
      assert.equal(await frame.locator('#rule-max-results').inputValue(),'100');
      assert.equal(await frame.locator('#rule-interval,#rule-timezone,#rule-temperature,#rule-retention').count(),0);
      assert.equal(await frame.locator('#rule-cron').inputValue(),'*/30 * * * *');assert(await frame.locator('#rule-cron').isVisible());assert(!(await timer.isChecked()));
      await timer.check();assert.match(await frame.locator('#rule-schedule-note').innerText(),/下个周期/);
      await frame.locator('[aria-labelledby="rule-plan-title"]').screenshot({path:path.join(release, `${capture}-plan-${width}.png`)});
      await frame.locator('#rule-scope-single').check(); assert(await frame.locator('#rule-target-field').isVisible());
      await frame.locator('#rule-scope-all').check();
      await frame.locator('[data-add-model="gpt-6.1-sol"]').click();
      assert.deepEqual(await frame.locator('#rule-model-selection code').allTextContents(),['gpt-6-astra','gpt-6.1-sol']);
      await frame.getByRole('radio',{name:'最高 max',exact:true}).check();
      assert(await frame.getByRole('radio',{name:'最高 max',exact:true}).isChecked());
      await frame.locator('#rule-preview-toggle').click();await frame.getByText('合成参考答案',{exact:true}).waitFor();
      assert(await editor.evaluate(n => n.scrollWidth <= n.clientWidth + 1 && document.documentElement.scrollWidth <= innerWidth));
      await frame.locator('[aria-labelledby="rule-question-title"]').screenshot({path:path.join(release, `${capture}-question-${width}.png`)});
      assert(await frame.getByRole('radio',{name:'最高 max',exact:true}).isChecked());assert(await timer.isChecked());
      await frame.locator('#rule-name').fill('离线预览');
      assert.match(await editor.locator('.editor-dirty').innerText(), /有未保存更改/);
      await editor.locator('.editor-footer [data-close]').click();
      await frame.locator('#action-dialog').waitFor();
      assert.match(await frame.locator('#action-title').textContent(), /放弃未保存/);
      await frame.locator('#action-confirm').press('Enter'); await editor.waitFor({state:'hidden'});
      assert.equal(await frame.locator('#rule-schedule-toggle,[data-rule-pause]').count(),0);
      checks.push({ width, ...layout, workspace:workspaceLayout, no_editor_estimate:true, exact_cron_label:true, continuous_sheet: true, single_timer: true, header_unobscured: true, save_reachable: true, default_all: true, single_selectable: true, model_chips: true, reasoning: true, single_cron: true, selected_max: true, result_limit: 100, pointer_cancel: true, timer_draft_discarded: true });
    }
    await page.setViewportSize({width:1440,height:900});
    await frame.locator('[data-page=manual]').click();
    await frame.locator('#run-form').waitFor();
    manual_checks.push({state:'empty',...await checkManual(frame.locator('.manual-workbench'))});
    await page.screenshot({path:path.join(release,`${capture}-manual-empty.png`)});
    await frame.locator('#prompt').fill('Preserved offline layout draft');
    await frame.locator('[data-manual-view=history]').click();
    await frame.locator('#history-list [data-run="fixture-run-complete"]').waitFor();
    manual_checks.push({state:'history',...await checkManual(frame.locator('.manual-workbench'))});
    await page.screenshot({path:path.join(release,`${capture}-manual-history.png`)});
    await frame.locator('#history-list [data-run="fixture-run-complete"]').click();
    await frame.locator('#sample-list .answer-text').first().waitFor();
    manual_checks.push({state:'answers',...await checkManual(frame.locator('.manual-workbench'))});
    await page.screenshot({path:path.join(release,`${capture}-manual-answers.png`)});
    await frame.locator('#manual-results').evaluate(node=>node.scrollTop=node.scrollHeight);
    await page.screenshot({path:path.join(release,`${capture}-manual-answer-footers.png`)});
    await frame.locator('#reading-toggle').click();
    manual_checks.push({state:'expanded',...await checkManual(frame.locator('.manual-workbench'))});
    await frame.locator('#reading-toggle').click();
    for (const width of [390,320]) {
      await page.setViewportSize({width,height:844});
      if (await page.locator('.sidebar.open').count()) await page.locator('.mobile-menu-btn').click();
      await page.waitForFunction(() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0);
      await page.locator('.page-transition--animating').waitFor({state:'detached'});
      for (const pane of ['setup','history','results']) {
        await frame.locator(`[data-manual-pane=${pane}]`).click();
        manual_checks.push({state:pane,...await checkManual(frame.locator('.manual-workbench'))});
        await page.screenshot({path:path.join(release,`${capture}-manual-${pane}-${width}.png`)});
      }
      await frame.locator('[data-manual-pane=setup]').click();
      assert.equal(await frame.locator('#prompt').inputValue(),'Preserved offline layout draft');
    }
    await frame.locator('[data-page=login]').click();
    for (const {width,height} of [{width:1440,height:900},{width:1280,height:650},{width:390,height:844},{width:320,height:844}]) {
      await page.setViewportSize({width,height});
      if (width<600) {
        if (await page.locator('.sidebar.open').count()) await page.locator('.mobile-menu-btn').click();
        await page.waitForFunction(() => document.querySelector('.sidebar').getBoundingClientRect().right <= 0);
      }
      await page.locator('.page-transition--animating').waitFor({state:'detached'});
      await frame.locator('#account-status-filter').selectOption('linked');
      assert((await frame.locator('.account-row').count())>0);
      await frame.locator('#account-status-filter').selectOption('all');
      await frame.evaluate(() => window.scrollTo(0, 0));
      await iframe.evaluate(node => { for (let parent = node.parentElement; parent; parent = parent.parentElement) parent.scrollTop = 0; window.scrollTo(0, 0); });
      await page.screenshot({path:path.join(release,`${capture}-accounts-${width}.png`)});
      for (const surface of ['history','settings']) {
        await frame.locator(`#account-${surface}-open`).click();
        const dialog=frame.locator(`#login-${surface}-dialog`);
        if(surface==='history') {
          await frame.locator('#login-task-search').fill('fixture-job-failed');
          await frame.locator('[data-login-job="fixture-job-failed"]').click();
          await frame.locator('.login-task-log-viewport .account-log-entry').first().waitFor();
        }
        account_checks.push({...await checkAccountSurface(dialog),host_width:width,host_height:height});
        for(const control of [dialog.locator('.account-surface-heading button'),dialog.locator('.account-surface-footer button').last()]) {
          const rect=await control.boundingBox();
          assert(await page.evaluate(({x,y,width,height})=>document.elementFromPoint(x+width/2,y+height/2)?.matches('iframe[src*="model-quality"]'),rect),'Account dialog controls must be reachable through the CPA shell');
        }
        await page.screenshot({path:path.join(release,`${capture}-account-${surface}-${width}.png`)});
        await dialog.locator('.account-surface-heading button').click();
        assert(await frame.locator(`#account-${surface}-open`).evaluate(n=>document.activeElement===n));
      }
      const profileOpener = frame.locator('[data-account-row="fixture-enabled"] [data-account-action=more]');
      await profileOpener.click();
      await frame.locator('[data-account-menu-action=edit]').click();
      await frame.locator('#account-editor-save:enabled').waitFor();
      assert.equal(await frame.locator('#account-password').inputValue(),'SYNTHETIC-NOT-A-REAL-PASSWORD');
      const profile=frame.locator('#account-editor-dialog');
      account_checks.push({...await checkAccountSurface(profile),host_width:width,host_height:height});
      for(const control of [profile.locator('#account-editor-close'),profile.locator('#account-editor-save')]){
        const rect=await control.boundingBox();assert(await page.evaluate(({x,y,width,height})=>document.elementFromPoint(x+width/2,y+height/2)?.matches('iframe[src*="model-quality"]'),rect),'Profile controls must be reachable through CPA');
      }
      await page.screenshot({path:path.join(release,`${capture}-account-profile-${width}.png`)});
      await profile.locator('#account-editor-close').click();
      assert(await profileOpener.evaluate(n=>document.activeElement===n));
      assert.equal(await frame.locator('#account-password').inputValue(),'');
    }
    assert(fixtureReads > 0); assert.equal(detailReads, 4); assert.deepEqual(writes, []);
    assert.deepEqual(served,ui_source_sha256,'The iframe assets must match the tested package');
    fs.writeFileSync(path.join(release, preview ? 'candidate-iframe.json' : 'deployed-iframe.json'), JSON.stringify({ verified_at: new Date().toISOString(), ui_version, assets:preview?'candidate':'installed', ui_source_sha256:served, management_url: panel, accounts: 'in-memory fixture only', checks, manual_checks, account_checks, detail_reads: detailReads, write_attempts: writes.length }, null, 2) + '\n', { mode: 0o600 });
    console.log(`${preview?'Candidate':'Installed'} iframe ${ui_version}: scopes, models, reasoning and discard passed at all three widths; zero writes.`);
  } finally { if (browser) await browser.close(); child.kill(); await exit; }
}
main().catch(err => { console.error('Installed iframe check failed:', err.name, err.stack?.match(/verify_rule_sheet_iframe\.cjs:\d+:\d+/)?.[0], String(err.message).replace(/Bearer\s+\S+/gi,'[redacted]').slice(-1300)); process.exitCode = 1; });
