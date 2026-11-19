// Exercise historical mixed health/task records using only the in-memory fixture.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {spawn} = require('node:child_process');
const {once} = require('node:events');
const {createInterface} = require('node:readline');
const {chromium} = require('/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

async function main() {
  const out = path.resolve(process.argv[2]); fs.mkdirSync(out, {recursive:true});
  const child = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], {stdio:['ignore','pipe','inherit']});
  const exit = once(child, 'exit'); let browser;
  const blocked = [], errors = [], checks = [];
  try {
    const [line] = await Promise.race([once(createInterface({input:child.stdout}), 'line'), exit.then(()=>{throw Error('Fixture failed');})]);
    const fixture = JSON.parse(line), origin = new URL(fixture.url).origin;
    assert.equal(fixture.fixture, 'synthetic-plugin-ui');
    browser = await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless:true});
    const context = await browser.newContext({viewport:{width:1440,height:900}, reducedMotion:'reduce'});
    await context.route('**/*', async route => {
      const req = route.request(), url = new URL(req.url());
      if (url.origin !== origin || req.method() !== 'GET') { blocked.push(req.method()+' '+url.pathname); return route.abort(); }
      if (url.pathname.endsWith('/login/accounts')) {
        const response = await route.fetch(), body = await response.json();
        for (const item of body.items) {
          item.health_reason = 'invalid_callback';
          if (item.id === 'fixture-new') item.health_status = 'unknown';
          if (item.id === 'fixture-active') Object.assign(item, {auth_id:'synthetic-disabled.json', health_status:'disabled'});
          if (item.id === 'fixture-history') item.health_status = 'missing';
          if (item.id === 'fixture-failed') Object.assign(item, {last_job_error_code:'invalid_callback', last_job_error_stage:'callback', last_job_error_reason:''});
        }
        return route.fulfill({response, json:body});
      }
      return route.continue();
    });
    const page = await context.newPage(); page.setDefaultTimeout(7000);
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(fixture.url);
    await page.locator('#login-connect').click();
    await page.locator('#management-key').fill(fixture.management_key);
    await page.locator('#connection-form button[type=submit]').click();
    await page.locator('#account-import-open:enabled').waitFor();
    for (const [id, expected] of [['fixture-new','尚未绑定'],['fixture-failed','尚未绑定'],['fixture-enabled','未记录认证异常'],['fixture-active','人工禁用'],['fixture-history','CPA 凭证缺失']]) {
      const status = page.locator(`[data-account-row="${id}"] [data-label="CPA 状态"]`);
      assert.equal(await status.locator('.account-status-text').innerText(), expected);
      assert.doesNotMatch(await status.innerText(), /OAuth|回调|登录失败|复制完整/);
      assert.doesNotMatch(await status.locator('.account-status').getAttribute('title'), /OAuth|回调/);
      checks.push({id, status:expected, historical_login_error_hidden:true});
    }
    assert.match(await page.locator('[data-account-row="fixture-uncertain"] [data-label="CPA 状态"]').innerText(), /安全操作待核对/);
    const recent = page.locator('[data-account-row="fixture-failed"] [data-label="最近操作"]');
    assert.match(await recent.innerText(), /登录失败[\s\S]*首次授权/);
    assert.doesNotMatch(await recent.innerText(), /OAuth|代理连接/);
    await page.locator('#account-status-filter').selectOption('attention');
    assert(await page.locator('[data-account-row="fixture-failed"]').isVisible());
    assert(await page.locator('[data-account-row="fixture-uncertain"]').isVisible());
    await page.locator('#account-status-filter').selectOption('all');
    for (const theme of ['light','white','dark']) for (const width of [1440,390]) {
      await page.setViewportSize({width,height:900});
      await page.locator('#theme-toggle').selectOption(theme);
      assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      await page.screenshot({path:path.join(out,`account-health-${theme}-${width}.png`)});
    }
    assert.deepEqual(blocked,[]); assert.deepEqual(errors,[]);
    fs.writeFileSync(path.join(out,'account-health.json'),JSON.stringify({ui_version:JSON.parse(await page.locator('#build-info').textContent()).ui_version, checks, safety_warning_preserved:true, recent_operation_compact:true, attention_filter_preserved:true, unexpected_requests:blocked, page_errors:errors, live_writes:0},null,2)+'\n');
    console.log('Historical login errors isolated from CPA status; credential states, safety warnings and concise recent operations verified.');
  } finally { if (browser) await browser.close(); child.kill(); await exit; }
}
main().catch(error=>{console.error(error);process.exitCode=1;});
