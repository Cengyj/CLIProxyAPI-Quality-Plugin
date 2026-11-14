// Verify browser interactions against the actual patched host and native library.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { chromium } = require(
  process.env.PLAYWRIGHT_MODULE ||
    '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright',
);
const root = path.resolve(__dirname, '..');
const out = process.env.UI_VERIFY_OUTPUT || path.join(root, 'artifacts/verification');
const demoDir = process.env.MQ_DEMO_DIR || path.join(root, 'artifacts/native-ui-demo');
const python = process.env.PYTHON_BIN || 'python3';
const demoScript = path.join(root, 'scripts/demo.py');
const checks = [];
let browser;
let demo;

function demoCommand(command) {
  const result = spawnSync(python, [demoScript, command], { cwd: root, encoding: 'utf8', timeout: 75000, env: { ...process.env, MQ_DEMO_DIR: demoDir } });
  if (result.status !== 0) throw new Error(`${command}: ${result.stderr || result.stdout || result.error}`);
  return JSON.parse(result.stdout);
}

async function api(route, method = 'GET', body) {
  const response = await fetch(`${demo.host_url}/v8/management/plugins/model-quality${route}`, {
    method,
    headers: { Authorization: `Bearer ${demo.admin_key}`, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(`${method} ${route}: ${response.status} ${JSON.stringify(data)}`);
  return data;
}

async function stats() {
  return (await fetch(`${demo.upstream_url}/stats`)).json();
}
async function mode(value) {
  const response = await fetch(`${demo.upstream_url}/control`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${demo.control_token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode: value }),
  });
  assert.equal(response.status, 200);
}
async function finish(id) {
  for (let attempt = 0; attempt < 100; attempt++) {
    const run = await api(`/runs/detail?id=${encodeURIComponent(id)}`);
    if (!['queued', 'running'].includes(run.status)) return run;
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`run ${id} did not finish`);
}

(async () => {
  fs.mkdirSync(out, { recursive: true });
  demo = demoCommand('start');
  const unauthorized = await fetch(`${demo.host_url}/v8/management/plugins/model-quality/targets`);
  assert.equal(unauthorized.status, 401);
  checks.push('actual v8 management rejects unauthenticated targets');
  const resourceHashes = {};
  for (const name of ['index.html', 'app.js', 'client.js', 'view.js', 'style.css']) {
    const response = await fetch(new URL(name, demo.page_url));
    assert.equal(response.status, 200);
    const returned = Buffer.from(await response.arrayBuffer());
    const source = fs.readFileSync(path.join(root, 'plugin/ui', name));
    const packaged = fs.readFileSync(path.join(root, 'artifacts/ui', name));
    assert.deepEqual(returned, source, `Native host resource ${name} must match source bytes`);
    assert.deepEqual(returned, packaged, `Native host resource ${name} must match static build bytes`);
    resourceHashes[name] = require('node:crypto').createHash('sha256').update(returned).digest('hex');
  }
  checks.push('all five native-host resources are byte-identical to UI source and static build');
  const targetReply = await api('/targets');
  const target = targetReply.items.find((entry) => entry.models.includes('mq-reasoner'));
  assert(target, `mock target missing: ${JSON.stringify(targetReply)}`);
  assert.equal(target.can_pause, true);
  const initialCalls = (await stats()).calls.length;
  browser = await chromium.launch({
    headless: true,
    args: ['--disable-gpu'],
    executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  });
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1080 },
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
  });
  const errors = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto(demo.page_url);
  await page.locator('#run-form').waitFor();
  assert.equal((await stats()).calls.length, initialCalls);
  await page.locator('#connection-top').click();
  await page.locator('#management-key').fill(demo.admin_key);
  await page.locator('#connection-form button[type=submit]').click();
  await page.waitForFunction(() => document.getElementById('connection-state').textContent === '已连接');
  assert.equal((await stats()).calls.length, initialCalls);
  assert.equal(await page.evaluate(() => localStorage.length + sessionStorage.length), 0);
  checks.push('actual public shell and authenticated connection make no model calls');
  await page.locator('#target').selectOption(target.auth_id);
  await page.locator('#models').fill('mq-reasoner');
  await page.locator('#question').selectOption('logic-boxes');
  await page.locator('#samples').fill('2');
  await page.locator('.setup-card .advanced summary').click();
  await page.locator('#reasoning').selectOption('medium');
  const runReply = page.waitForResponse(
    (response) => response.url().endsWith('/runs') && response.request().method() === 'POST',
  );
  await page.locator('#run-submit').click();
  const created = await (await runReply).json();
  assert(created.run_id, JSON.stringify(created));
  const run = await finish(created.run_id);
  assert.equal(run.status, 'completed', JSON.stringify(run));
  assert.equal(run.samples.length, 2);
  for (const secret of [demo.admin_key, demo.business_key, 'mq-fake-upstream-a', 'mq-fake-upstream-b'])
    assert(!JSON.stringify(run).includes(secret), 'Run export must exclude secret values');
  for (const sample of run.samples) {
    assert.equal(sample.execution_status, 'completed', JSON.stringify(sample));
    assert.equal(sample.output, '4', JSON.stringify(sample));
    assert.equal(sample.quality_verdict, 'correct', JSON.stringify(sample));
    assert.equal(sample.comparable, true, JSON.stringify(sample));
  }
  assert(
    (await stats()).calls
      .slice(initialCalls)
      .every((call) => call.model === 'mock-reasoner-v1' && call.stream),
  );
  await page.waitForFunction(
    () =>
      document.querySelectorAll('.sample').length === 2 &&
      document.getElementById('run-meta').textContent.includes('已完成'),
  );
  await page.evaluate(() => scrollTo(0, 0));
  await page.screenshot({
    animations: 'disabled',
    path: path.join(out, 'native-manual-page.png'),
    fullPage: true,
  });
  fs.writeFileSync(path.join(out, 'native-manual-run.json'), JSON.stringify(run, null, 2) + '\n');
  checks.push('native C ABI streaming multi-sample fixed-auth alias reasoning run');

  const settings = await api('/settings');
  await api('/settings', 'PUT', { ...settings, automatic_pause: true, scheduling_enabled: false });
  await page.locator('[data-page=operations]').click();
  await page.locator('[data-ops-view=rules]').click();
  await page.locator('#rule-new').click();
  await page.locator('#rule-name').fill('离线原生链路 · 凭证质量守卫');
  await page.locator('#rule-target').selectOption(target.auth_id);
  await page.locator('#rule-models').fill('mq-reasoner');
  await page.locator('#rule-question').selectOption('logic-boxes');
  await page.locator('#rule-samples').fill('1');
  await page.locator('#rule-action').selectOption('pause_credential');
  await page.locator('#rule-auto-pause').check();
  const beforeSave = (await stats()).calls.length;
  const saveReply = page.waitForResponse(
    (response) => response.url().endsWith('/rules') && response.request().method() === 'POST',
  );
  await page.locator('#rule-form button[type=submit]').click();
  const rule = await (await saveReply).json();
  assert(rule.id, JSON.stringify(rule));
  assert.equal((await stats()).calls.length, beforeSave);
  await page.locator('#rules-list strong').filter({ hasText: '离线原生链路' }).waitFor();
  checks.push('actual rule save makes no model calls');

  await mode('incorrect');
  const failureRuns = [];
  for (let i = 0; i < 3; i++) {
    const queued = await api('/rules/run', 'POST', { id: rule.id });
    failureRuns.push(await finish(queued.run_id));
  }
  const paused = (await api('/actions')).items;
  assert.equal(paused.length, 1, JSON.stringify({ failureRuns, paused }));
  assert.equal(paused[0].auth_id, target.auth_id);
  assert.equal(
    (await api('/targets')).items.find((entry) => entry.auth_id === target.auth_id).quality_paused,
    true,
  );
  checks.push('actual three complete wrong rounds persist credential pause');

  await mode('correct');
  const beforeBlocked = (await stats()).calls.length;
  const heldRunTotal = (await api('/runs')).total;
  const heldBudgetCalls = (await api('/metrics')).budget.calls;
  const manualConfig = {
    auth_id: target.auth_id,
    provider: target.provider,
    models: ['mq-reasoner'],
    question_id: 'logic-boxes',
    samples: 1,
    parameters: { max_tokens: 128, stream: true },
    judge: { mode: 'deterministic' },
    idempotency_key: 'native-demo-paused-manual',
  };
  const expectBlocked = async (route, body) => {
    const response = await fetch(`${demo.host_url}/v8/management/plugins/model-quality${route}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${demo.admin_key}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    assert.equal(response.status, 409, `${route}: ${await response.text()}`);
  };
  await expectBlocked('/runs', manualConfig);
  await expectBlocked('/rules/run', { id: rule.id });
  const heldRule = (await api('/rules')).items.find((entry) => entry.id === rule.id);
  assert.equal(heldRule.status, 'awaiting_release');
  assert.equal(heldRule.enabled, false);
  await expectBlocked('/rules/pause', { id: rule.id, revision: heldRule.revision, paused: false });
  const peer = targetReply.items.find((entry) => entry.auth_id !== target.auth_id);
  assert(peer);
  await expectBlocked('/runs', {
    ...manualConfig,
    auth_id: peer.auth_id,
    provider: peer.provider,
    question_id: 'random-linear',
    control_auth_id: target.auth_id,
    control_provider: target.provider,
    idempotency_key: 'paused-control',
  });
  await expectBlocked('/runs', {
    ...manualConfig,
    auth_id: peer.auth_id,
    provider: peer.provider,
    judge: { mode: 'llm', auth_id: target.auth_id, provider: target.provider, model: 'mq-reasoner' },
    idempotency_key: 'paused-judge',
  });
  assert.equal((await stats()).calls.length, beforeBlocked);
  assert.equal((await api('/runs')).total, heldRunTotal);
  assert.equal((await api('/metrics')).budget.calls, heldBudgetCalls);
  checks.push('paused target, rule, control and judge reject before enqueue or budget consumption');
  checks.push('paused credential forbids all retests and rule resume');
  await page.locator('[data-ops-view=attention]').click();
  await page.locator('#refresh').click();
  await page.locator('[data-action-preview]').waitFor();
  assert.equal(await page.locator('[data-rule-run]').isDisabled(), true);
  assert.equal(await page.locator('[data-rule-pause]').isDisabled(), true);
  assert.equal(await page.locator('#run-submit').isDisabled(), true);
  assert.equal(await page.locator('#run-rerun').isDisabled(), true);
  await page.evaluate(() => scrollTo(0, 0));
  await page.screenshot({
    animations: 'disabled',
    path: path.join(out, 'native-operations-page.png'),
    fullPage: true,
  });
  await page.locator('[data-action-preview]').click();
  await page.locator('#release-dialog').waitFor({ state: 'visible' });
  await page.evaluate(() => scrollTo(0, 0));
  await page.screenshot({
    animations: 'disabled',
    path: path.join(out, 'native-release-preview.png'),
    fullPage: true,
  });
  await page.locator('#release-confirm').click();
  await page.waitForFunction(() => document.getElementById('release-feedback').textContent.includes('已无此暂停归属'));
  assert.equal((await api('/actions')).items.length, 0);
  const releasedRule = (await api('/rules')).items.find((entry) => entry.id === rule.id);
  assert.equal(releasedRule.enabled, false);
  assert.equal(releasedRule.status, 'paused');
  assert.equal(releasedRule.fail_count, 0);
  await expectBlocked('/rules/run', { id: rule.id });
  assert.equal((await stats()).calls.length, beforeBlocked);
  checks.push('browser CAS release preserves stopped detection and clears prior failure evidence');
  await page.locator('[data-close=release-dialog]').first().click();
  await page.locator('[data-ops-view=rules]').click();
  await page.locator('[data-rule-pause]').filter({ hasText: '恢复检测规则' }).click();
  await page.locator('[data-rule-run]').waitFor();
  await page.waitForFunction(() => !document.querySelector('[data-rule-run]').disabled);
  const resumed = await api('/rules/run', 'POST', { id: rule.id });
  const freshRun = await finish(resumed.run_id);
  assert.equal(freshRun.samples[0].quality_verdict, 'correct', JSON.stringify(freshRun));
  assert.equal((await api('/actions')).items.length, 0);
  assert.equal(
    (await stats()).calls.length,
    initialCalls + 6,
    'Only explicit manual and active rule runs may reach the upstream',
  );
  checks.push('only explicit administrator resume permits a fresh detection after release');
  assert.deepEqual(errors, []);
  const report = {
    result: 'passed',
    verified_at: new Date().toISOString(),
    source_sha256: Object.fromEntries(['index.html', 'app.js', 'client.js', 'view.js', 'style.css'].map((name) => [name, require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(root, 'plugin/ui', name))).digest('hex')])),
    binary_sha256: Object.fromEntries(['cli-proxy-api-quality', process.platform === 'darwin' ? 'model-quality.dylib' : 'model-quality.so'].map((name) => [name, require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(root, 'artifacts', name))).digest('hex')])),
    platform: `${process.platform}/${process.arch}`,
    browser: await browser.version(),
    host_binary: 'artifacts/cli-proxy-api-quality',
    library: process.platform === 'darwin' ? 'artifacts/model-quality.dylib' : 'artifacts/model-quality.so',
    session_dir: demo.session_dir,
    resource_sha256: resourceHashes,
    checks,
    upstream_calls: (await stats()).calls.length,
    cleanup: 'demo stopped by finally handler',
    screenshots: ['native-manual-page.png', 'native-operations-page.png', 'native-release-preview.png'],
    screenshot_sha256: Object.fromEntries(['native-manual-page.png', 'native-operations-page.png', 'native-release-preview.png'].map((name) => [name, require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(out, name))).digest('hex')])),
  };
  if (fs.existsSync(path.join(out, 'native-ui-failure.txt')))
    fs.unlinkSync(path.join(out, 'native-ui-failure.txt'));
  fs.writeFileSync(path.join(out, 'native-ui-results.json'), JSON.stringify(report, null, 2) + '\n');
  console.log(JSON.stringify(report, null, 2));
})()
  .catch((error) => {
    fs.writeFileSync(path.join(out, 'native-ui-failure.txt'), error.stack + '\n');
    console.error(error);
    process.exitCode = 1;
  })
  .finally(async () => {
    if (browser) await browser.close();
    if (demo) demoCommand('stop');
  });
