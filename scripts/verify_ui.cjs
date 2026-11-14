// Exercise the embedded UI against a local fake management service.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const { chromium } = require(
  process.env.PLAYWRIGHT_MODULE ||
    '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright',
);
const root = path.resolve(__dirname, '..');
const output = process.env.UI_VERIFY_OUTPUT || path.join(root, 'artifacts/verification');
const uiSource = process.env.UI_SOURCE_DIR || path.join(root, 'plugin/ui');
const sourceHashes = () => Object.fromEntries(['index.html', 'app.js', 'client.js', 'view.js', 'style.css'].map((name) => [name, require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(uiSource, name))).digest('hex')]));
const initialSourceHashes = sourceHashes();
fs.mkdirSync(output, { recursive: true });
const calls = [];
const robustnessChecks = [];
const deferred = () => {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const staticCSP = fs
  .readFileSync(path.join(root, 'plugin/cmd/model-quality/main.go'), 'utf8')
  .match(/"Content-Security-Policy": \{"([^"]+)"\}/)[1];
const models = ['mock-reasoner', 'mock-visual'];
const targets = [
  {
    auth_id: 'mock-credential-a',
    auth_index: 'mock-a',
    provider: 'openai',
    type: 'config_api_key',
    models,
    identity_version: 'identity-a',
    manual_disabled: false,
    quality_paused: false,
    effective_available: true,
    can_pause: true,
    can_retest: false,
    reasons: [],
  },
  {
    auth_id: 'mock-credential-b',
    auth_index: 'mock-b',
    provider: 'openai',
    type: 'config_api_key',
    models,
    identity_version: 'identity-b',
    manual_disabled: false,
    quality_paused: false,
    effective_available: true,
    can_pause: true,
    can_retest: false,
    reasons: [],
  },
];
const questions = [
  {
    id: 'random-linear',
    title: '可复现随机约束题',
    kind: 'random',
    prompt: '提交时生成随机题面',
    version: '1',
  },
  {
    id: 'logic',
    title: '有界组合推理',
    kind: 'logic',
    prompt: '在 1 到 9 中，所有偶数的和是多少？只输出整数。',
    version: '1',
  },
  {
    id: 'visual',
    title: '轨道与节奏',
    kind: 'html',
    prompt: '创建独立 HTML，用 SVG 绘制行星轨道，不使用外部资源。',
    version: '1',
  },
];
let rules = [];
let presets = [];
let actions = [];
let runs = [];
let sequence = 0;
let settings = {
  data_dir: 'mock-data',
  workers: 2,
  queue: 32,
  max_models: 8,
  max_samples: 32,
  max_output_tokens: 4096,
  max_response_bytes: 262144,
  daily_calls: 200,
  daily_tokens: 1000000,
  retention_days: 30,
  max_storage_bytes: 134217728,
  scheduling_enabled: false,
  automatic_pause: false,
};
const created = '2026-11-13T13:00:00Z';
const list = (array, url) => {
  const offset = Number(url.searchParams.get('offset') || 0),
    limit = Number(url.searchParams.get('limit') || 50);
  return { items: array.slice(offset, offset + limit), total: array.length, offset, limit };
};
const config = {
  auth_id: 'mock-credential-a',
  provider: 'openai',
  models: ['mock-reasoner'],
  question_id: 'logic',
  samples: 1,
  parameters: { max_tokens: 2048, temperature: 0, stream: true },
  judge: { mode: 'deterministic' },
};

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost');
  if (!url.pathname.startsWith('/v8/management/')) {
    const asset = url.pathname.split('/').pop() || 'index.html';
    if (!['index.html', 'app.js', 'client.js', 'view.js', 'style.css'].includes(asset)) {
      res.writeHead(404).end();
      return;
    }
    res.setHeader('Content-Security-Policy', staticCSP);
    res.setHeader(
      'Content-Type',
      asset.endsWith('.js') ? 'text/javascript' : asset.endsWith('.css') ? 'text/css' : 'text/html',
    );
    res.end(fs.readFileSync(path.join(uiSource, asset)));
    return;
  }
  let body = '';
  for await (const chunk of req) body += chunk;
  const parsed = body ? JSON.parse(body) : {};
  calls.push({
    method: req.method,
    path: url.pathname,
    query: url.search,
    body: parsed,
    authorization: req.headers.authorization,
  });
  if (req.headers.authorization !== 'Bearer offline-test-key') {
    res.writeHead(401, { 'Content-Type': 'application/json' }).end('{"error":"unauthorized"}');
    return;
  }
  const apiPath = url.pathname.replace('/v8/management/plugins/model-quality', '');
  let result = {},
    status = 200;
  switch (`${req.method} ${apiPath}`) {
    case 'GET /targets':
      result = list(targets, url);
      break;
    case 'GET /questions':
      result = list(questions, url);
      break;
    case 'GET /presets':
      result = list(presets, url);
      break;
    case 'POST /presets':
      presets.push({ ...parsed, id: 'preset-1', created_at: created });
      result = presets.at(-1);
      break;
    case 'DELETE /presets':
      presets = presets.filter((preset) => preset.id !== parsed.id);
      result = { ok: true };
      break;
    case 'GET /history':
      result = list(
        [
          {
            id: 'audit-1',
            kind: 'rule_saved',
            time: created,
            rule_id: 'rule-1',
            detail: '规则配置已保存，未启动模型调用',
          },
        ],
        url,
      );
      break;
    case 'GET /rules':
      result = list(rules, url);
      break;
    case 'GET /actions':
      result = list(actions, url);
      break;
    case 'GET /runs':
      result = list(runs, url);
      break;
    case 'GET /runs/detail':
      result = runs.find((run) => run.id === url.searchParams.get('id'));
      break;
    case 'GET /metrics':
      result = {
        correct: 6,
        incorrect: 1,
        unknown: 1,
        quality_pass_rate: 6 / 7,
        coverage: 7 / 8,
        known_tokens: 1893,
        unknown_usage: 1,
        dropped_usage_events: 0,
      };
      break;
    case 'GET /settings':
      result = settings;
      break;
    case 'PUT /settings':
      settings = parsed;
      result = settings;
      break;
    case 'POST /runs': {
      if (
        [parsed.auth_id, parsed.control_auth_id, parsed.judge?.auth_id].some(
          (id) => targets.find((target) => target.auth_id === id)?.quality_paused,
        )
      ) {
        status = 409;
        result = { error: { code: 'blocked', message: 'credential quality pause requires manual release' } };
        break;
      }
      const id = `run-${++sequence}`;
      const sample = {
        model: parsed.models[0],
        index: 0,
        execution_status: 'completed',
        quality_verdict: 'correct',
        output: '20',
        total_ms: 630,
        ttft_ms: 145,
        usage: { input_tokens: 41, output_tokens: 2, total_tokens: 43 },
        comparable: true,
        response_model: 'mock-reasoner',
        reason: 'strict integer match',
        evidence: {
          verified: true, requested_model: parsed.models[0], resolved_model: 'mock-upstream-model',
          auth_id: parsed.auth_id, payload_fingerprint: 'offline-payload-fingerprint',
          conditions_changed: false, path: 'openai/chat/completions', payload_config_version: 'offline-v1',
        },
      };
      runs.unshift({
        id,
        status: 'completed',
        source: 'manual_test',
        created_at: created,
        completed_at: created,
        config: { ...parsed, identity_version: 'identity-a', fingerprint: 'frozen', question: questions[0] },
        samples: [sample],
        action_status: 'not_applicable',
      });
      result = { run_id: id };
      status = 202;
      break;
    }
    case 'POST /runs/cancel':
      runs.find((run) => run.id === parsed.id).status = 'cancelled';
      result = { ok: true };
      break;
    case 'POST /runs/review':
      result = { ok: true };
      break;
    case 'POST /rules':
      rules.push({
        ...parsed,
        id: 'rule-1',
        revision: 1,
        fail_count: 0,
        status: parsed.enabled ? 'detecting' : 'paused',
      });
      result = rules.at(-1);
      break;
    case 'PUT /rules':
      rules = rules.map((rule) =>
        rule.id === parsed.id ? { ...parsed, revision: parsed.revision + 1 } : rule,
      );
      result = rules[0];
      break;
    case 'DELETE /rules':
      rules = rules.filter((rule) => rule.id !== parsed.id);
      result = { ok: true };
      break;
    case 'POST /rules/pause': {
      const rule = rules.find((entry) => entry.id === parsed.id);
      rule.enabled = !parsed.paused;
      rule.status = rule.enabled ? 'detecting' : 'paused';
      rule.revision++;
      result = { ok: true };
      break;
    }
    case 'POST /actions/preview':
      result = {
        can_release: true,
        action: actions[0],
        remaining_reasons: ['manual_disabled'],
        scope: 'all_models',
      };
      break;
    case 'POST /actions/release':
      assert.equal(parsed.pause_rule, true);
      actions = [];
      targets[0].quality_paused = false;
      targets[0].reasons = targets[0].reasons.filter((reason) => reason !== 'quality_pause');
      rules[0].enabled = false;
      rules[0].status = 'paused';
      rules[0].action_id = '';
      rules[0].blocked_auth_id = '';
      result = { ok: true };
      break;
    default:
      status = 404;
      result = { error: `unsupported mock ${req.method} ${apiPath}` };
  }
  res.writeHead(status, { 'Content-Type': 'application/json' }).end(JSON.stringify(result));
});

(async () => {
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  const browser = await chromium.launch({
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
  const viewportEvidence = [];
  const dialogGeometry = [];
  const verifyDialogPadding = async (id) => {
    const previousViewport = page.viewportSize();
    const previousScroll = await page.locator(`#${id}`).evaluate((node) => node.scrollTop);
    for (const width of [360, 390]) {
      await page.setViewportSize({ width, height: 844 });
      const geometry = await page.locator(`#${id}`).evaluate(async (node) => {
        await document.fonts.ready;
        await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
        node.scrollTop = node.scrollHeight;
        const rect = node.getBoundingClientRect();
        const last = node.lastElementChild.getBoundingClientRect();
        return { paddingBottom: parseFloat(getComputedStyle(node).paddingBottom), bottomInset: rect.bottom - last.bottom, dialogBottom: rect.bottom, lastContentBottom: last.bottom };
      });
      assert(geometry.paddingBottom >= 20, `${id} has at least 20px computed bottom padding at ${width}px`);
      assert(geometry.bottomInset >= 19.9, `${id} visible content remains inset from its bottom edge at ${width}px`);
      dialogGeometry.push({ id, width, height: 844, ...geometry });
      await page.screenshot({ path: path.join(output, `dialog-padding-${id}-${width}.png`), animations: 'disabled' });
    }
    await page.setViewportSize(previousViewport);
    await page.locator(`#${id}`).evaluate((node, top) => { node.scrollTop = top; }, previousScroll);
  };
  const pairedCapture = async (name, width = 1440, height = 900, theme = 'light') => {
    const previousViewport = page.viewportSize();
    const previousTheme = await page.locator('html').getAttribute('data-theme');
    await page.setViewportSize({ width, height });
    await page.evaluate(async (value) => {
      document.documentElement.dataset.theme = value;
      scrollTo(0, 0);
      await document.fonts.ready;
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    }, theme);
    await page.screenshot({ path: path.join(output, `paired-${name}.png`), animations: 'disabled' });
    viewportEvidence.push({ name, width, height, theme, ...(await page.evaluate(() => { const answer = [...document.querySelectorAll('.answer-text')].find((node) => node.getClientRects().length); return { pageOverflow: document.documentElement.scrollWidth > innerWidth, firstAnswerTop: answer?.getBoundingClientRect().top ?? null }; })) });
    await page.setViewportSize(previousViewport);
    await page.evaluate((value) => { document.documentElement.dataset.theme = value || 'light'; }, previousTheme);
  };
  const setup = () => page.locator('#page-manual > .view-nav [data-manual-view=setup]').click();
  const history = () => page.locator('#page-manual > .view-nav [data-manual-view=history]').click();
  const operations = async (view = 'attention') => {
    await page.locator('[data-page=operations]').click();
    await page.locator(`[data-ops-view=${view}]`).click();
  };
  const capture = async (name) => {
    await page.evaluate(async () => {
      scrollTo(0, 0);
      await document.fonts.ready;
      await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    });
    await page.screenshot({ animations: 'disabled', path: path.join(output, name), fullPage: true });
  };
  page.on('pageerror', (error) => errors.push(error.message));
  try {
    await page.goto(`${base}/v0/resource/plugins/model-quality/index.html`);
    await page.locator('#run-form').waitFor();
    assert.equal(await page.locator('[data-page]').count(), 2);
    assert.equal(await page.locator('#manual-setup').isVisible(), true);
    assert.equal(await page.locator('#sidebar').count(), 0);
    await capture('disconnected-page.png');
    await page.keyboard.press('Tab');
    assert.equal(await page.evaluate(() => document.activeElement.classList.contains('skip-link')), true);
    assert.notEqual(await page.evaluate(() => getComputedStyle(document.activeElement).outlineStyle), 'none');
    await page.keyboard.press('Enter');
    assert.equal(await page.evaluate(() => document.activeElement.id), 'main');
    await page.emulateMedia({ reducedMotion: 'reduce' });
    assert.equal(await page.locator('#theme-toggle').evaluate((node) => getComputedStyle(node).transitionDuration), '0s');
    await page.emulateMedia({ reducedMotion: 'no-preference' });
    robustnessChecks.push('keyboard skip link, visible focus and reduced-motion preference work');
    const contrastEvidence = [];
    for (const theme of ['light', 'white', 'dark']) {
      await page.locator('#theme-toggle').selectOption(theme);
      assert.equal(await page.locator('html').getAttribute('data-theme'), theme, 'Theme selection exposes the requested theme directly');
      const computed = await page.evaluate(() => {
        const styles = getComputedStyle(document.documentElement);
        const names = ['page', 'surface', 'surface-raised', 'surface-muted', 'hover', 'ink', 'muted', 'good', 'good-bg', 'warn', 'warn-bg', 'bad', 'bad-bg', 'accent', 'accent-hover', 'on-accent', 'border-strong', 'focus'];
        return { colors: Object.fromEntries(names.map((name) => [name, styles.getPropertyValue(`--${name}`).trim()])), placeholder: getComputedStyle(document.getElementById('models'), '::placeholder').color };
      });
      const luminance = (hex) => {
        const channels = hex.replace('#', '').match(/../g).map((part) => parseInt(part, 16) / 255).map((n) => n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4);
        return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722;
      };
      const pairs = [];
      for (const foreground of ['ink', 'muted']) for (const background of ['page', 'surface', 'surface-raised', 'surface-muted', 'hover']) pairs.push([foreground, background, 4.5]);
      for (const foreground of ['good', 'warn', 'bad']) pairs.push([foreground, `${foreground}-bg`, 4.5]);
      pairs.push(['on-accent', 'accent', 4.5], ['on-accent', 'accent-hover', 4.5]);
      for (const background of ['page', 'surface', 'surface-raised']) pairs.push(['border-strong', background, 3], ['focus', background, 3]);
      const checks = pairs.map(([foreground, background, threshold]) => {
        const a = luminance(computed.colors[foreground]), b = luminance(computed.colors[background]);
        const ratio = (Math.max(a, b) + .05) / (Math.min(a, b) + .05);
        assert(ratio >= threshold, `${theme} ${foreground}/${background} ${ratio.toFixed(2)} must meet ${threshold}:1`);
        return { foreground, background, foreground_hex: computed.colors[foreground], background_hex: computed.colors[background], ratio, threshold, result: 'passed' };
      });
      const placeholderHex = '#' + computed.placeholder.match(/\d+/g).slice(0, 3).map((n) => Number(n).toString(16).padStart(2, '0')).join('');
      assert.equal(placeholderHex, computed.colors.muted, 'Placeholder uses the tested secondary text token');
      contrastEvidence.push({ theme, placeholder: computed.placeholder, checks });
    }
    await page.locator('#theme-toggle').selectOption('light');
    fs.writeFileSync(path.join(output, 'contrast-results.json'), JSON.stringify({ result: 'passed', verified_at: new Date().toISOString(), browser: await browser.version(), source_sha256: initialSourceHashes, scope: 'Computed theme color pairs and placeholder text; not an accessibility certification', themes: contrastEvidence }, null, 2) + '\n');
    robustnessChecks.push('explicit theme selection, computed text contrast and control/focus contrast pass in light, white and dark; placeholders use tested muted colors');
    assert.equal(calls.length, 0, 'Public shell must not call management or models');
    await page.locator('#connection-open').click();
    await verifyDialogPadding('connection-dialog');
    await page.locator('#management-key').fill('offline-test-key');
    await page.locator('#connection-form button[type=submit]').click();
    await page.waitForFunction(() => document.getElementById('connection-state').textContent === '已连接');
    assert(
      calls.every((call) => call.method === 'GET'),
      'Connecting must only read',
    );
    assert(calls.every((call) => call.authorization === 'Bearer offline-test-key'));
    assert(!calls.some((call) => call.query.includes('offline-test-key')));
    assert.equal(await page.evaluate(() => localStorage.length + sessionStorage.length), 0);
    assert.equal(await page.locator('#management-key').inputValue(), '');
    await capture('connected-empty-page.png');
    await pairedCapture('setup-desktop');
    await pairedCapture('setup-mobile', 390, 844);
    await page.locator('#question').selectOption('logic');
    await page.locator('#target').selectOption('mock-credential-a');
    await page.locator('#model-chips button').first().click();
    const beforePreset = calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length;
    await page.locator('#preset-tools summary').click();
    await page.locator('#preset-save').click();
    await page.locator('#action-dialog').waitFor({ state: 'visible' });
    await verifyDialogPadding('action-dialog');
    await page.keyboard.press('Escape');
    assert.equal(await page.locator('#action-dialog').isVisible(), false);
    assert.equal(await page.evaluate(() => document.activeElement.id), 'preset-save');
    await page.locator('#preset-save').click();
    await page.locator('#action-dialog').waitFor({ state: 'visible' });
    await page.locator('#action-name').fill('离线逻辑预设');
    await page.locator('#action-confirm').click();
    await page.waitForFunction(() => document.getElementById('preset').options.length === 2);
    await page.locator('#preset').selectOption('preset-1');
    await page.locator('#preset-apply').click();
    await page.waitForFunction(() => document.getElementById('notice').textContent.includes('预设已载入'));
    await page.locator('[data-manual-view=results]').click();
    assert.equal(await page.locator('#notice').isVisible(), false, 'Preset feedback must not appear as task state');
    await setup();
    assert.equal(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length,
      beforePreset,
      'Saving or loading preset must not call a model',
    );
    robustnessChecks.push('preset naming uses an accessible product dialog with Escape and opener focus return');
    const presetDeleteEntered = deferred();
    const presetDeleteGate = deferred();
    const presetDeletesBefore = calls.filter((call) => call.method === 'DELETE' && call.path.endsWith('/presets')).length;
    await page.route('**/presets', async (route) => {
      if (route.request().method() !== 'DELETE') return route.continue();
      presetDeleteEntered.resolve();
      await presetDeleteGate.promise;
      await route.continue();
    });
    await page.locator('#preset-delete').click();
    await presetDeleteEntered.promise;
    await history();
    presetDeleteGate.resolve();
    await page.waitForFunction(() => !document.getElementById('preset-delete').hasAttribute('aria-busy'));
    assert.equal(await page.locator('#notice').isVisible(), false, 'A delayed preset deletion cannot report success in the newly selected history view');
    assert.equal(presets.length, 0);
    assert.equal(calls.filter((call) => call.method === 'DELETE' && call.path.endsWith('/presets')).length, presetDeletesBefore + 1);
    await page.unroute('**/presets');
    await setup();
    robustnessChecks.push('a delayed preset deletion completes exactly once without leaking feedback into another view');
    await page.locator('#run-submit').click();
    await page.locator('.sample pre').filter({ hasText: '20' }).waitFor();
    const firstRun = calls.find((call) => call.method === 'POST' && call.path.endsWith('/runs'));
    assert.equal(firstRun.body.auth_id, 'mock-credential-a');
    assert.equal(firstRun.body.parameters.stream, true);
    assert(!JSON.stringify(firstRun.body).includes('answer'));
    assert.equal(await page.locator('#manual-setup').isVisible(), false, 'Results replace the setup form');
    assert.equal(await page.locator('#manual-results').isVisible(), true);
    await page.locator('[data-evidence-sample]').first().click();
    await page.locator('#evidence-dialog').waitFor({ state: 'visible' });
    assert.match(await page.locator('#evidence-content').textContent(), /mock-upstream-model/);
    await verifyDialogPadding('evidence-dialog');
    await capture('evidence-detail-page.png');
    await page.locator('[data-close=evidence-dialog]').click();
    robustnessChecks.push('sample evidence exposes requested and resolved models with structured conditions');
    await page.locator('.review-disclosure summary').first().click();
    await page.locator('#review-0').selectOption('correct');
    await page.locator('#review-note-0').fill('离线测试：严格整数回答');
    await page.locator('[data-review-sample]').click();
    await page.waitForFunction(() =>
      document.getElementById('local-feedback').textContent.includes('评审已保存'),
    );
    await page.locator('#run-rerun').click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('run-2'));
    const rerun = calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).at(-1);
    assert(!('fingerprint' in rerun.body), 'Frozen evidence fields must not be reposted to strict API');
    assert.notEqual(firstRun.body.idempotency_key, rerun.body.idempotency_key);
    const downloadPromise = page.waitForEvent('download');
    await page.locator('#run-export').click();
    const download = await downloadPromise;
    await download.saveAs(path.join(output, 'sample-run-export.json'));
    const exported = fs.readFileSync(path.join(output, 'sample-run-export.json'), 'utf8');
    assert(!exported.includes('offline-test-key'));
    assert.equal(JSON.parse(exported).id, 'run-2');
    await page.evaluate(() => scrollTo(0, 0));
    await page.screenshot({
      animations: 'disabled',
      path: path.join(output, 'manual-page.png'),
      fullPage: true,
    });
    await setup();
    assert.equal(await page.locator('#notice').isVisible(), false, 'Local review feedback must not survive navigation to configuration');
    robustnessChecks.push('local operation feedback clears when switching workspace views');
    await page.locator('#question').selectOption('random-linear');
    await page.locator('#control-target').selectOption('mock-credential-a');
    await page.locator('#run-submit').click();
    await page.waitForFunction(() =>
      document.getElementById('notice').textContent.includes('必须选择另一个凭证'),
    );
    await page.locator('#control-target').selectOption('mock-credential-b');
    await page.locator('#run-submit').click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('run-3'));
    const controlledRun = calls
      .filter((call) => call.method === 'POST' && call.path.endsWith('/runs'))
      .at(-1);
    assert.equal(controlledRun.body.control_auth_id, 'mock-credential-b');
    assert.equal(controlledRun.body.control_provider, 'openai');
    await setup();
    await page.locator('#control-target').selectOption('');
    await page.locator('#question').selectOption('logic');

    const pending = {
      id: 'cancel-run',
      status: 'running',
      source: 'manual_test',
      created_at: created,
      config,
      samples: [],
      action_status: 'not_applicable',
    };
    runs.unshift(pending);
    await page.locator('#refresh').click();
    await history();
    await page.locator('[data-run="cancel-run"]').first().click();
    await capture('running-page.png');
    await page.locator('#run-cancel').click();
    await page.waitForFunction(() => document.getElementById('run-meta').textContent.includes('已取消'));

    await operations('rules');
    await page.locator('#rule-new').click();
    await page.locator('#rule-name').fill('核心模型巡检');
    await page.locator('#rule-target').selectOption('mock-credential-a');
    await page.locator('#rule-models').fill('mock-reasoner\nmock-visual');
    await capture('rule-editor-page.png');
    const invalidModels = Array.from({ length: 9 }, (_, index) => `offline-model-${index + 1}`).join('\n');
    const writesBeforeInvalidRule = calls.filter((call) => call.method !== 'GET').length;
    await page.locator('#rule-models').fill(invalidModels);
    await page.locator('#rule-form button[type=submit]').click();
    await page.waitForFunction(() => {
      const field = document.getElementById('rule-models');
      return field.getAttribute('aria-invalid') === 'true' && !field.disabled;
    });
    assert.equal(await page.locator('#rule-models').inputValue(), invalidModels, 'Client validation preserves the invalid rule draft');
    assert.equal(await page.locator('#rule-name').inputValue(), '核心模型巡检');
    assert.equal(await page.evaluate(() => document.activeElement.id), 'rule-models', 'Client validation focuses the invalid field after the save lock is released');
    assert.match(await page.locator('#rule-models-error').textContent(), /1–8/);
    assert.equal(calls.filter((call) => call.method !== 'GET').length, writesBeforeInvalidRule, 'Invalid rule validation makes no API writes or model calls');
    await page.locator('#rule-models').fill('mock-reasoner\nmock-visual');
    assert.equal(await page.locator('#rule-models').getAttribute('aria-invalid'), null);
    robustnessChecks.push('client-side rule validation preserves the draft, focuses its invalid field after unlocking, and makes no API writes');
    const callCountBeforeSave = calls.filter(
      (call) => call.method === 'POST' && call.path.endsWith('/runs'),
    ).length;
    const ruleSaveEntered = deferred();
    const ruleSaveGate = deferred();
    await page.route('**/rules', async (route) => {
      if (route.request().method() !== 'POST') return route.continue();
      ruleSaveEntered.resolve();
      await ruleSaveGate.promise;
      await route.continue();
    });
    await page.locator('#rule-form button[type=submit]').click();
    await ruleSaveEntered.promise;
    assert.equal(await page.locator('#rule-name').isEnabled(), false, 'Saving locks the submitted draft against edits');
    assert.equal(await page.locator('[data-close=rule-dialog]').first().isEnabled(), false, 'Saving locks editor close until its result is known');
    await page.locator('#settings-open').click();
    assert.equal(await page.locator('#settings-dialog').isVisible(), false, 'Saving cannot be replaced by another editor');
    ruleSaveGate.resolve();
    await page.locator('#rules-list strong').filter({ hasText: '核心模型巡检' }).waitFor();
    await page.unroute('**/rules');
    robustnessChecks.push('an in-flight rule save locks draft fields and close controls and cannot be replaced by a different editor');
    await capture('rules-page.png');
    assert.equal(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length,
      callCountBeforeSave,
      'Saving rule must not call a model',
    );
    assert.equal(rules[0].action, 'observe_only');
    assert.equal(rules[0].automatic_pause, false);
    assert(!('automatic_recovery' in rules[0]));
    assert(!('pass_threshold' in rules[0]));
    assert(!('retest_seconds' in rules[0]));
    assert.equal(await page.locator('#rule-auto-recovery,#rule-pass,#rule-retest').count(), 0);
    assert.equal(rules[0].scheduling_enabled, false);
    rules.push({ ...rules[0], id: 'delete-rule', name: '待删除的离线规则' });
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    const deletesBefore = calls.filter((call) => call.method === 'DELETE' && call.path.endsWith('/rules')).length;
    await page.locator('[data-rule-delete="delete-rule"]').click();
    assert.match(await page.locator('#action-dialog').textContent(), /待删除的离线规则/);
    await page.locator('#action-cancel').click();
    assert.equal(calls.filter((call) => call.method === 'DELETE' && call.path.endsWith('/rules')).length, deletesBefore);
    await page.locator('[data-rule-delete="delete-rule"]').click();
    await page.locator('#action-confirm').click();
    await page.locator('[data-rule-delete="delete-rule"]').waitFor({ state: 'detached' });
    assert.equal(rules.length, 1);
    assert.equal(calls.filter((call) => call.method === 'DELETE' && call.path.endsWith('/rules')).length, deletesBefore + 1);
    robustnessChecks.push('rule deletion names the object, supports cancellation and submits exactly one explicit delete');
    const rulePauseEntered = deferred();
    const rulePauseGate = deferred();
    const rulePausesBefore = calls.filter((call) => call.method === 'POST' && call.path.endsWith('/rules/pause')).length;
    await page.route('**/rules/pause', async (route) => {
      rulePauseEntered.resolve();
      await rulePauseGate.promise;
      await route.continue();
    });
    await page.locator('[data-rule-pause]').click();
    await rulePauseEntered.promise;
    await page.locator('[data-page=manual]').click();
    await setup();
    rulePauseGate.resolve();
    await page.waitForFunction(() => document.getElementById('rules-list').textContent.includes('恢复检测规则'));
    assert.equal(await page.locator('#notice').isVisible(), false, 'A delayed rule pause cannot report success in the manual workspace');
    assert.equal(calls.filter((call) => call.method === 'POST' && call.path.endsWith('/rules/pause')).length, rulePausesBefore + 1);
    await page.unroute('**/rules/pause');
    await operations('rules');
    await page.locator('[data-rule-pause]').filter({ hasText: '恢复检测规则' }).waitFor();
    assert.equal(rules[0].enabled, false);
    robustnessChecks.push('a delayed rule pause completes exactly once without leaking feedback into the manual workspace');

    const settingsReadEntered = deferred();
    const settingsReadGate = deferred();
    await page.route('**/settings', async (route) => {
      if (route.request().method() !== 'GET') return route.continue();
      settingsReadEntered.resolve();
      await settingsReadGate.promise;
      await route.continue();
    });
    await page.locator('#settings-open').click();
    await settingsReadEntered.promise;
    await page.locator('#rule-new').click();
    await page.locator('#rule-name').fill('保留后打开的草稿');
    settingsReadGate.resolve();
    await page.waitForFunction(() => !document.getElementById('settings-open').hasAttribute('aria-busy'));
    assert.equal(await page.locator('#rule-dialog').isVisible(), true);
    assert.equal(await page.locator('#settings-dialog').isVisible(), false);
    assert.equal(await page.locator('#rule-name').inputValue(), '保留后打开的草稿');
    await page.unroute('**/settings');
    await page.locator('[data-close=rule-dialog]').first().click();
    await page.locator('#action-confirm').click();
    await page.locator('#action-dialog').waitFor({ state: 'hidden' });
    robustnessChecks.push('a delayed budget read cannot replace a rule editor opened after the read began');

    await page.locator('#settings-open').click();
    await page.locator('#setting-daily_calls').fill('120');
    await page.route('**/settings', async (route) => {
      if (route.request().method() === 'PUT')
        return route.fulfill({
          status: 409,
          contentType: 'application/json',
          body: '{"error":{"code":"conflict","message":"configuration revision conflict"}}',
        });
      return route.continue();
    });
    await page.locator('#settings-form button[type=submit]').click();
    await page.locator('#settings-dialog .dialog-message').waitFor({ state: 'visible' });
    assert((await page.locator('#settings-dialog .dialog-message').textContent()).includes('conflict'));
    assert.equal(await page.locator('#settings-dialog').isVisible(), true);
    assert.equal(await page.locator('#setting-daily_calls').inputValue(), '120');
    await page.unroute('**/settings');
    await page.locator('[data-close=settings-dialog]').first().click();
    await page.locator('#action-dialog').waitFor({ state: 'visible' });
    assert.match(await page.locator('#action-dialog').textContent(), /放弃|未保存/);
    await page.locator('#action-cancel').click();
    assert.equal(await page.locator('#settings-dialog').isVisible(), true);
    assert.equal(await page.locator('#setting-daily_calls').inputValue(), '120');
    robustnessChecks.push('unsaved budget edits require explicit discard and cancellation preserves the draft');
    robustnessChecks.push('failed mutation keeps form values and exposes its error inside the inline editor');
    await page.locator('#settings-form button[type=submit]').click();
    await page.waitForFunction(() =>
      document.getElementById('notice').textContent.includes('全局开关与预算已保存'),
    );
    assert.equal(settings.daily_calls, 120);
    assert.equal(settings.data_dir, 'mock-data');
    assert.equal(settings.workers, 2);
    assert(!('automatic_recovery' in settings));
    assert.equal(await page.locator('#settings-auto-recovery').count(), 0);

    actions = [
      {
        auth_id: 'mock-credential-a',
        owner_plugin: 'model-quality',
        rule_id: 'rule-1',
        rule_revision: 1,
        action_id: 'pause-1',
        action_revision: 1,
        identity_version: 'identity-a',
        reason: '所有选定模型连续 3 轮明确失败',
        created_at: created,
      },
    ];
    rules[0].status = 'awaiting_release';
    rules[0].action_id = 'pause-1';
    rules[0].blocked_auth_id = targets[0].auth_id;
    targets[0].quality_paused = true;
    targets[0].manual_disabled = true;
    targets[0].effective_available = false;
    targets[0].reasons = ['manual_disabled', 'quality_pause'];
    await operations();
    await page.locator('#refresh').click();
    await page.locator('[data-action-preview]').waitFor();
    assert.equal(await page.locator('[data-rule-run]').isDisabled(), true);
    assert.equal(await page.locator('[data-rule-pause]').isDisabled(), true);
    assert.equal(await page.locator('#run-submit').isDisabled(), true);
    assert.equal(await page.locator('#run-rerun').isDisabled(), true);
    const heldCallCount = calls.filter(
      (call) => call.method === 'POST' && call.path.endsWith('/runs'),
    ).length;
    await page.locator('[data-page=manual]').click();
    await setup();
    await page.locator('#target').selectOption('mock-credential-b');
    await page.locator('#question').selectOption('random-linear');
    await page.locator('#control-target').selectOption('mock-credential-a');
    assert.equal(await page.locator('#run-submit').isDisabled(), true, 'Paused control must block execution');
    await page.locator('#control-target').selectOption('');
    await page.locator('#question').selectOption('logic');
    await page.locator('.setup-card .advanced summary').click();
    await page.locator('#judge-mode').selectOption('llm');
    await page.locator('#judge-target').selectOption('mock-credential-a');
    assert.equal(await page.locator('#run-submit').isDisabled(), true, 'Paused judge must block execution');
    await page.locator('#judge-mode').selectOption('manual');
    assert.equal(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length,
      heldCallCount,
    );
    await operations();
    await page.evaluate(() => scrollTo(0, 0));
    await page.screenshot({
      animations: 'disabled',
      path: path.join(output, 'operations-page.png'),
      fullPage: true,
    });
    await pairedCapture('operations-paused-desktop');
    await pairedCapture('operations-paused-mobile', 390, 844);
    await page.locator('[data-action-preview]').click();
    await page.locator('#release-dialog').waitFor({ state: 'visible' });
    await page.waitForFunction(() => !document.getElementById('release-confirm').disabled);
    await verifyDialogPadding('release-dialog');
    assert.equal(
      await page.locator('#release-pause-rule').count(),
      0,
      'Manual release cannot keep detection enabled',
    );
    const originalAction = { ...actions[0] };
    await page.route('**/actions/release', (route) => route.fulfill({
      status: 409,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'conflict', message: 'action missing or revision changed' } }),
    }));
    await page.locator('#release-confirm').click();
    await page.waitForFunction(() => document.getElementById('release-dialog').textContent.includes('conflict'));
    assert.equal(await page.locator('#release-dialog').isVisible(), true);
    assert.equal(await page.locator('#release-confirm').isDisabled(), true, 'Stale release requires a fresh preview');
    assert.equal(targets[0].quality_paused, true);
    await capture('release-conflict-page.png');
    await page.unroute('**/actions/release');
    actions[0].action_revision = 2;
    await page.route('**/actions/preview', (route) => route.fulfill({
      status: 503, contentType: 'application/json', body: '{"error":"preview storage unavailable"}',
    }));
    await page.locator('#release-refresh').click();
    await page.waitForFunction(() => document.getElementById('release-feedback').textContent.includes('预览未完成'));
    assert.equal(await page.locator('#release-confirm').isDisabled(), true);
    assert(!(await page.locator('#release-feedback').textContent()).includes('已重新读取'));
    await page.unroute('**/actions/preview');
    const previewGate = deferred();
    const previewRequested = deferred();
    await page.route('**/actions/preview', async (route) => {
      previewRequested.resolve();
      await previewGate.promise;
      await route.continue();
    });
    await page.locator('#release-refresh').click();
    await previewRequested.promise;
    await page.locator('[data-close=release-dialog]').first().click();
    previewGate.resolve();
    await page.waitForFunction(() => !document.getElementById('release-refresh').hasAttribute('aria-busy'));
    assert.equal(await page.locator('#release-dialog').isVisible(), false, 'A late preview cannot reopen a closed release dialog');
    await page.unroute('**/actions/preview');
    await page.locator('[data-action-preview]').click();
    await page.waitForFunction(() => !document.getElementById('release-confirm').disabled);
    robustnessChecks.push('failed preview cannot claim refreshed evidence and a delayed preview cannot reopen a closed dialog');
    await page.locator('#release-confirm').click();
    await page.waitForFunction(() => document.getElementById('release-feedback').textContent.includes('已无此暂停归属'));
    robustnessChecks.push('409 release conflict disables stale confirmation and rebinds a refreshed action revision');
    assert.equal(targets[0].manual_disabled, true);
    assert.equal(rules[0].enabled, false);
    assert.equal(rules[0].status, 'paused');
    const releaseCall = calls.find((call) => call.path.endsWith('/actions/release'));
    assert.deepEqual(Object.keys(releaseCall.body).sort(), [
      'action_id',
      'action_revision',
      'pause_rule',
      'rule_id',
    ]);
    assert.equal(releaseCall.body.action_revision, 2);
    await capture('released-still-restricted-page.png');
    await page.locator('[data-close=release-dialog]').first().click();
    actions = [{ ...originalAction, action_id: 'pause-uncertain', action_revision: 3 }];
    targets[0].quality_paused = true;
    targets[0].reasons = ['manual_disabled', 'quality_pause'];
    rules[0].status = 'awaiting_release';
    rules[0].action_id = 'pause-uncertain';
    await page.locator('#refresh').click();
    await page.locator('[data-action-preview]').click();
    let uncertainPosts = 0;
    await page.route('**/actions/release', (route) => {
      uncertainPosts++;
      actions = [];
      targets[0].quality_paused = false;
      targets[0].reasons = ['manual_disabled'];
      rules[0].status = 'paused';
      rules[0].action_id = '';
      rules[0].blocked_auth_id = '';
      rules[0].fail_count = 0;
      return route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'storage_unavailable', message: 'owned pause released; remaining restrictions await reconciliation' } }),
      });
    });
    await page.locator('#release-confirm').click();
    await page.waitForFunction(() => /未确认|不确定|待核对|待确认|待对账/.test(document.getElementById('release-dialog').textContent));
    assert.equal(await page.locator('#release-confirm').isDisabled(), true);
    assert.equal(await page.locator('#release-dialog').isVisible(), true);
    assert.match(await page.locator('#release-dialog').textContent(), /未确认|不确定|待核对|待确认|待对账/);
    await capture('release-uncertain-page.png');
    await page.route('**/targets?limit=100', (route) => route.fulfill({
      status: 503, contentType: 'application/json', body: '{"error":"remaining restrictions unavailable"}',
    }));
    await page.locator('#release-refresh').click();
    await page.waitForFunction(() => document.getElementById('release-feedback').textContent.includes('状态核对失败'));
    assert.equal(await page.locator('#release-confirm').isDisabled(), true);
    assert.match(await page.locator('#release-feedback').textContent(), /仍待核对/);
    await page.unroute('**/targets?limit=100');
    await page.locator('#release-refresh').click();
    await page.waitForFunction(() => document.getElementById('release-feedback').textContent.includes('已无此暂停归属'));
    await page.waitForFunction(() => document.querySelectorAll('[data-action-preview]').length === 0);
    assert.equal(uncertainPosts, 1, 'Reconciliation only reads; it must not retry the release');
    assert.equal(targets[0].manual_disabled, true);
    assert.equal(rules[0].enabled, false);
    assert.equal(await page.locator('#notice').evaluate((node) => node.classList.contains('error') && !node.classList.contains('hidden')), false, 'Reconciled release clears the stale global error');
    if (await page.locator('#release-dialog').isVisible()) await page.locator('[data-close=release-dialog]').first().click();
    await page.unroute('**/actions/release');
    robustnessChecks.push('uncertain 503 release is reconciled through reads without repeating mutation or clearing other restrictions');

    const attack = `<style>circle{opacity:.7}</style><svg viewBox="0 0 240 160"><circle cx="120" cy="80" r="50" fill="#22604a"/></svg><script>parent.document.body.dataset.compromised='yes';fetch('https://example.invalid/leak')</script><img src="https://example.invalid/image"><meta http-equiv="refresh" content="0;url=https://example.invalid/nav"><a href="https://example.invalid/link">Link<set attributeName="href" to="https://example.invalid/animated"/></a><form action="https://example.invalid/form"></form>`;
    runs.unshift({
      id: 'visual-run',
      status: 'completed',
      source: 'manual_test',
      created_at: created,
      config: { ...config, question_id: 'visual' },
      samples: [
        {
          model: 'mock-visual',
          index: 0,
          execution_status: 'completed',
          quality_verdict: 'unknown',
          output: attack,
          total_ms: 430,
          ttft_ms: null,
          usage: null,
          comparable: false,
        },
      ],
      action_status: 'not_applicable',
    });
    const external = [];
    await page.route('https://example.invalid/**', (route) => {
      external.push(route.request().url());
      route.abort();
    });
    await page.locator('#refresh').click();
    await page.locator('[data-page=manual]').click();
    await history();
    await page.locator('[data-run=visual-run]').click();
    await capture('insufficient-evidence-page.png');
    assert.equal(
      await page.locator('.sample pre').textContent(),
      attack,
      'Raw HTML must be displayed as text',
    );
    await page.locator('[data-preview-sample]').click();
    const frame = page.frameLocator('#safe-preview');
    await frame.locator('svg circle').waitFor();
    assert.equal(
      await frame.locator('svg circle').evaluate((element) => getComputedStyle(element).opacity),
      '0.7',
      'Parent CSP must permit sandbox inline styles',
    );
    assert.equal(await page.locator('#safe-preview').getAttribute('sandbox'), '');
    assert.equal(await frame.locator('script,form,iframe,meta[http-equiv=refresh]').count(), 0);
    assert.equal(await frame.locator('[src],[href],a,area,map,set').count(), 0);
    assert.equal(await page.locator('#safe-preview').getAttribute('inert'), '');
    assert.equal(await page.locator('body').getAttribute('data-compromised'), null);
    assert.deepEqual(external, []);
    await verifyDialogPadding('preview-dialog');
    await page.evaluate(() => scrollTo(0, 0));
    await page.screenshot({
      animations: 'disabled',
      path: path.join(output, 'sandbox-preview.png'),
      fullPage: true,
    });
    await page.locator('[data-close=preview-dialog]').click();
    const longModel = 'offline-research-model-with-a-very-long-deployment-and-reasoning-variant-2026-11-14';
    const comparisonSamples = [longModel, 'mock-reasoner'].flatMap((model, modelIndex) =>
      [0, 1].map((index) => ({
        model, index, execution_status: 'completed',
        quality_verdict: modelIndex === 0 && index === 1 ? 'unknown' : modelIndex === 1 ? 'incorrect' : 'correct',
        comparable: !(modelIndex === 0 && index === 1),
        output: index ? `// Offline synthetic long output\nconst evidence = "${'condition_'.repeat(30)}";\n${'Only complete and comparable evidence can support a decision.\n'.repeat(14)}` : '20',
        total_ms: 650, ttft_ms: null, usage: null,
        reason: modelIndex === 0 && index === 1 ? 'truncated output; evidence incomplete' : 'offline comparison sample',
        evidence: { verified: modelIndex !== 0 || index !== 1, requested_model: model, resolved_model: 'offline-mapped-model', conditions_changed: false },
      })),
    );
    runs.unshift({
      id: 'comparison-run', status: 'completed', source: 'manual_test', created_at: created,
      config: { ...config, models: [longModel, 'mock-reasoner'], samples: 2 },
      samples: comparisonSamples, action_status: 'not_applicable',
    });
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    await history();
    await page.locator('[data-run=comparison-run]').click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('comparison-run'));
    assert.equal(await page.locator('.model-result:visible').count(), 2);
    assert.equal(await page.locator('.sample:visible').count(), 4);
    await page.locator('#sample-filter').selectOption('1');
    assert.equal(await page.locator('.sample:visible').count(), 2);
    assert.equal(await page.locator('.sample[data-sample="1"]:visible').count(), 2);
    assert.equal(await page.locator('[data-review-sample="3"]').count(), 1, 'Review uses original sample array index after filtering');
    const expand = page.locator('[data-expand-sample]').first();
    await expand.click();
    assert.equal(await expand.getAttribute('aria-expanded'), 'true');
    assert.equal(await page.locator('.answer-text.expanded').evaluate((node) => node.scrollHeight <= node.clientHeight + 1), true);
    await expand.click();
    assert.equal(await expand.getAttribute('aria-expanded'), 'false');
    await capture('comparison-page.png');
    await pairedCapture('comparison-desktop-light');
    await pairedCapture('comparison-desktop-dark', 1440, 900, 'dark');
    await pairedCapture('comparison-desktop-white', 1440, 900, 'white');
    await pairedCapture('comparison-mobile-light', 390, 844);
    await page.locator('#theme-toggle').selectOption('dark');
    await capture('comparison-dark-page.png');
    await page.locator('#theme-toggle').selectOption('light');
    await page.locator('#compare-model-b').selectOption('');
    assert.equal(await page.locator('.sample:visible').count(), 1);
    await page.locator('#compare-model-a').selectOption('mock-reasoner');
    await page.locator('#compare-model-b').selectOption(longModel);
    assert.equal(await page.locator('.model-result:visible').count(), 2);
    assert.equal(await page.locator('.model-result:visible').first().getAttribute('data-model'), 'mock-reasoner', 'Primary selected model leads the reading order');
    await page.setViewportSize({ width: 390, height: 844 });
    const taskMenu = page.locator('#task-actions');
    await page.waitForFunction(() => !document.getElementById('task-actions').open);
    assert.equal(await taskMenu.getAttribute('open'), null, 'Phone task actions start collapsed');
    await taskMenu.locator('summary').click();
    assert.equal(await page.locator('#run-export').isVisible(), true);
    await page.locator('#run-export').focus();
    await page.keyboard.press('Escape');
    assert.equal(await taskMenu.getAttribute('open'), null);
    assert.equal(await taskMenu.locator('summary').evaluate((node) => document.activeElement === node), true, 'Escape returns focus to the action disclosure');
    await taskMenu.locator('summary').click();
    await page.locator('#manual-title').click();
    assert.equal(await taskMenu.getAttribute('open'), null, 'Clicking outside closes phone task actions');
    robustnessChecks.push('phone task actions open explicitly, close on Escape or outside click and return keyboard focus to their disclosure');
    await capture('mobile-comparison-page.png');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'Long models and long code cannot overflow the page');
    await page.setViewportSize({ width: 1440, height: 1080 });
    robustnessChecks.push('model comparison and sample filtering preserve original review indices and contain long output on mobile');
    for (const width of [360, 390, 768, 1024, 1280, 1440]) {
      await page.setViewportSize({ width, height: width < 768 ? 844 : 900 });
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, `Comparison must fit ${width}px`);
      if (width < 768) {
        const smallInputs = await page.locator('input:visible,select:visible,textarea:visible').evaluateAll((nodes) => nodes.filter((node) => !['checkbox', 'radio'].includes(node.type) && parseFloat(getComputedStyle(node).fontSize) < 16).map((node) => node.id));
        assert.deepEqual(smallInputs, [], 'Phone fields use at least 16px text');
      }
    }
    assert.equal(await page.evaluate(() => CSS.supports('grid-template-rows', 'subgrid')), true, 'Verified Chromium supports comparison subgrid');
    await page.setViewportSize({ width: 720, height: 450 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, '200% zoom equivalent viewport reflows without horizontal page overflow');
    await page.setViewportSize({ width: 1440, height: 1080 });
    robustnessChecks.push('comparison reflows at 360/390/768/1024/1280/1440px and a 720×450 CSS viewport representing 200% zoom; Chromium subgrid verified');
    await operations('activity');
    assert.equal(await page.locator('#operations-history .empty-line').count(), 0, 'Audit history must not display an empty operational-history state');
    robustnessChecks.push('audit-only operational history does not show an empty state');
    await operations();
    await page.locator('#theme-toggle').selectOption('dark');
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'dark');
    await page.screenshot({
      animations: 'disabled',
      path: path.join(output, 'operations-dark-page.png'),
      fullPage: true,
    });
    await page.locator('#theme-toggle').selectOption('white');
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'white');
    await page.locator('#theme-toggle').selectOption('light');
    assert.equal(await page.locator('html').getAttribute('data-theme'), 'light');
    await page.evaluate(() => {
      location.hash = '#manual';
    });
    await page.locator('#page-manual').waitFor({ state: 'visible' });
    robustnessChecks.push('CPA warm light, dark and white themes with task navigation and hash navigation');
    await setup();

    await page.locator('#question').selectOption('custom');
    await page.locator('#prompt').fill('A custom prompt that must survive a refresh');
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    assert.equal(await page.locator('#prompt').inputValue(), 'A custom prompt that must survive a refresh');
    robustnessChecks.push('refresh preserves unsaved custom question');
    await page.locator('#question').selectOption('logic');

    await page.route('**/metrics', (route) =>
      route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'storage_unavailable', message: 'metrics storage unavailable' },
        }),
      }),
    );
    await page.locator('#refresh').click();
    await page.waitForFunction(() =>
      document.getElementById('notice').textContent.includes('storage_unavailable'),
    );
    assert.equal(await page.locator('#run-submit').isDisabled(), true);
    assert((await page.locator('#data-state').textContent()).includes('失败'));
    await capture('management-error-page.png');
    await page.unroute('**/metrics');
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    assert.equal(await page.locator('#run-submit').isDisabled(), false);
    robustnessChecks.push('management storage failure is visible and execution waits for refreshed state');

    const submitGate = deferred();
    const submitted = deferred();
    await page.route('**/model-quality/runs', async (route) => {
      if (route.request().method() !== 'POST') return route.continue();
      submitted.resolve();
      await submitGate.promise;
      await route.continue();
    });
    const beforeDouble = calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length;
    await page.locator('#run-submit').click();
    await submitted.promise;
    await page.locator('#run-form').evaluate((form) => {
      form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    });
    assert.equal(await page.locator('#run-submit').isDisabled(), true);
    submitGate.resolve();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('run-4'));
    await page.unroute('**/model-quality/runs');
    assert.equal(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length,
      beforeDouble + 1,
    );
    robustnessChecks.push('duplicate submissions are coalesced while request is pending');

    runs.unshift({ ...pending, id: 'stale-run', status: 'completed' });
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    const staleGate = deferred();
    const staleRequested = deferred();
    const staleFinished = deferred();
    await page.route('**/runs/detail?id=stale-run', async (route) => {
      staleRequested.resolve();
      await staleGate.promise;
      await route.continue();
      staleFinished.resolve();
    });
    await history();
    await page.locator('[data-run=stale-run]').first().click();
    await staleRequested.promise;
    await history();
    await page.locator('[data-run=visual-run]').first().click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('visual-run'));
    staleGate.resolve();
    await staleFinished.promise;
    await page.unroute('**/runs/detail?id=stale-run');
    await page.evaluate(
      () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
    );
    assert((await page.locator('#run-frozen-data').textContent()).includes('visual-run'));
    robustnessChecks.push('late task details cannot replace the newly selected task');

    {
      await page.clock.install();
      const reading = { id: 'reading-run', status: 'running', source: 'manual_test', created_at: created, config: { ...config, models: [longModel, 'mock-reasoner'], samples: 2 }, samples: structuredClone(comparisonSamples), action_status: 'not_applicable' };
      reading.samples[1].reason = '';
      runs.unshift(reading);
      let readingPolls = 0;
      await page.route('**/runs/detail?id=reading-run', async (route) => { readingPolls++; await route.continue(); });
      await page.locator('#refresh').click();
      await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
      await history();
      await page.locator('[data-run=reading-run]').first().click();
      await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('reading-run'));
      await page.locator('#sample-filter').selectOption('1');
      await page.locator('.sample:visible .review-disclosure summary').first().click();
      await page.locator('#review-note-1').fill('未保存的轮询评审草稿');
      await page.locator('[data-expand-sample="1"]').click();
      await page.locator('.sample:visible .answer-text').last().evaluate((node) => { node.scrollTop = 120; });
      await page.locator('#review-note-1').focus();
      await page.locator('#review-note-1').evaluate((node) => node.setSelectionRange(2, 6));
      const probeState = () => page.evaluate(() => ({ active: document.activeElement.id, note: document.getElementById('review-note-1').value, selection: [document.getElementById('review-note-1').selectionStart, document.getElementById('review-note-1').selectionEnd], reviewOpen: document.getElementById('review-note-1').closest('details').open, expanded: document.querySelector('[data-expand-sample="1"]').getAttribute('aria-expanded'), scroll: [...document.querySelectorAll('.sample .answer-text')].at(-1).scrollTop, filter: document.getElementById('sample-filter').value }));
      const before = await probeState();
      reading.samples[0].output = 'Unrelated sample completed during polling';
      reading.samples[1].reason = 'exact answer';
      await page.clock.runFor(1200);
      await page.waitForFunction(() => [...document.querySelectorAll('.sample pre')].some((node) => node.textContent === 'Unrelated sample completed during polling'));
      const after = await probeState();
      fs.writeFileSync(path.join(output, 'polling-probe.json'), JSON.stringify({ before, after, readingPolls }, null, 2) + '\n');
      assert.deepEqual(after, before, 'Polling preserves focused review selection, draft, disclosure, expanded answer, internal scroll and filter');
      assert.equal(await page.locator('.sample:visible .sample-reason').first().isVisible(), true);
      assert.equal(await page.locator('.sample:visible .sample-reason').first().textContent(), '答案与冻结参考答案一致');
      robustnessChecks.push('a previously empty sample reason becomes visible when polling supplies a known explanation');
      robustnessChecks.push('controlled polling preserves focused review selection, unsaved note, disclosure, expanded output, internal scroll and comparison filter');
      await page.locator('.sample:visible .answer-text').first().evaluate((node) => {
        const range = document.createRange(); range.setStart(node.firstChild, 4); range.setEnd(node.firstChild, 24);
        const selection = getSelection(); selection.removeAllRanges(); selection.addRange(range);
      });
      const selectedOutput = await page.evaluate(() => getSelection().toString());
      assert(selectedOutput.length > 0);
      reading.status = 'completed';
      await page.clock.runFor(1200);
      await page.waitForFunction(() => document.getElementById('run-meta').textContent.includes('已完成'));
      assert.equal(await page.evaluate(() => getSelection().toString()), selectedOutput, 'Finishing a run preserves output text selection');
      robustnessChecks.push('output text selection survives a background task finishing');
      await page.unroute('**/runs/detail?id=reading-run');
      await history();
      await page.locator('[data-run=visual-run]').first().click();
      await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('visual-run'));
      assert.equal(await page.locator('#sample-filter').inputValue(), 'all');
      assert.equal(await page.locator('.answer-text.expanded').count(), 0);
      assert.equal(await page.locator('#review-note-0').inputValue(), '');
      robustnessChecks.push('per-task review and comparison state resets when selecting another run');
      await page.clock.resume();
    }

    await page.clock.install();
    const polling = { ...pending, id: 'polling-run', status: 'running' };
    runs.unshift(polling);
    let pollReads = 0;
    await page.route('**/runs/detail?id=polling-run', async (route) => {
      pollReads++;
      if (pollReads === 2)
        return route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: '{"error":"temporary read failure"}',
        });
      return route.continue();
    });
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    await history();
    await page.locator('[data-run=polling-run]').first().click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('polling-run'));
    const postsBeforePoll = calls.filter(
      (call) => call.method === 'POST' && call.path.endsWith('/runs'),
    ).length;
    await page.clock.runFor(1200);
    await page.waitForFunction(() => document.getElementById('run-feedback').textContent.includes('自动重试查询'));
    polling.status = 'completed';
    await page.clock.runFor(2400);
    await page.waitForFunction(() => document.getElementById('run-meta').textContent.includes('已完成'));
    assert.equal(pollReads, 3);
    assert.equal(await page.locator('#run-feedback').isVisible(), false, 'Recovered progress clears the stale query error');
    assert.equal(
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/runs')).length,
      postsBeforePoll,
    );
    await page.unroute('**/runs/detail?id=polling-run');
    await page.clock.resume();
    robustnessChecks.push(
      'poll failures retry with backoff using a controlled clock and never resubmit runs',
    );

    const stressModels = Array.from({ length: 8 }, (_, index) => `offline-same-prefix-deployment-model-${'long-name-'.repeat(10)}variant-${index + 1}`);
    const stressOutput = ('Readable offline evidence with a bounded response.\n').repeat(6000).slice(0, 262144);
    const stressRun = { id: 'stress-run', status: 'completed', source: 'manual_test', created_at: created, config: { ...config, models: stressModels, samples: 4 }, samples: stressModels.flatMap((model) => Array.from({ length: 4 }, (_, index) => ({ model, index, output: stressOutput, execution_status: 'completed', quality_verdict: 'unknown', comparable: false, total_ms: 0, usage: null }))), action_status: 'not_applicable' };
    runs.unshift(stressRun);
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    await history();
    const stressStarted = performance.now();
    await page.locator('[data-run=stress-run]').first().click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('stress-run'));
    assert.equal(await page.locator('.sample').count(), 32);
    assert.equal(await page.locator('.model-result:visible').count(), 2);
    assert.equal(await page.locator('.sample:visible').count(), 8);
    await page.locator('#compare-model-b').selectOption(stressModels[7]);
    await page.locator('#sample-filter').selectOption('3');
    assert.equal(await page.locator('.sample:visible').count(), 2);
    assert.equal(await page.locator('.sample:visible .answer-text').first().textContent(), stressOutput);
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    const copied = page.locator('.sample:visible [data-copy-sample]').first();
    await copied.click();
    await page.waitForFunction(() => [...document.querySelectorAll('[data-copy-sample]')].some((node) => node.textContent.includes('已复制')));
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), stressOutput);
    assert.equal(await page.locator('#notice').isVisible(), false, 'Copy feedback stays by its action');
    await page.locator('.model-result:visible [data-copy-model]').last().click();
    assert.equal(await page.evaluate(() => navigator.clipboard.readText()), stressModels[7]);
    const stressEvidence = { models: 8, samples: 32, bytes_per_output: Buffer.byteLength(stressOutput), total_output_bytes: Buffer.byteLength(stressOutput) * 32, load_and_interact_ms: Math.round(performance.now() - stressStarted), output_preserved: true };
    fs.writeFileSync(path.join(output, 'stress-results.json'), JSON.stringify(stressEvidence, null, 2) + '\n');
    robustnessChecks.push('maximum 8 models and 32 samples with 256KiB outputs remain filterable, readable and losslessly copyable; full model IDs remain distinct');
    await page.setViewportSize({ width: 360, height: 800 });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true, 'Maximum content fits 360px');
    await capture('stress-mobile-page.png');
    const mobileSample = page.locator('.sample:visible').first();
    await mobileSample.locator('[data-expand-sample]').click();
    await mobileSample.locator('.answer-text').evaluate((node) => scrollTo(0, node.getBoundingClientRect().top + scrollY + 400));
    const mobileContext = await mobileSample.locator('.sample-heading').evaluate((node) => ({ top: node.getBoundingClientRect().top, model: node.querySelector('.sample-model-label')?.textContent, position: getComputedStyle(node).position }));
    assert.equal(mobileContext.position, 'sticky');
    assert(Math.abs(mobileContext.top) <= 1, 'Reading a long expanded answer keeps sample context at the top');
    assert.equal(mobileContext.model, stressModels[0]);
    await page.screenshot({ path: path.join(output, 'mobile-reading-context.png'), animations: 'disabled' });
    robustnessChecks.push('mobile long-answer scrolling retains the model and sample identity in the single local sticky heading');
    await mobileSample.locator('[data-expand-sample]').click();
    const eightSamples = { ...stressRun, id: 'eight-samples-run', config: { ...stressRun.config, models: [stressModels[0]], samples: 8 }, samples: Array.from({ length: 8 }, (_, index) => ({ ...stressRun.samples[0], index })) };
    runs.unshift(eightSamples);
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    await history();
    await page.locator('[data-run=eight-samples-run]').first().click();
    await page.waitForFunction(() => document.getElementById('run-frozen-data').textContent.includes('eight-samples-run'));
    assert.equal(await page.locator('.sample:visible').count(), 8);
    await page.locator('#sample-filter').selectOption('7');
    assert.equal(await page.locator('.sample:visible').count(), 1);
    assert.equal(await page.locator('.sample:visible').getAttribute('data-sample'), '7');
    robustnessChecks.push('the maximum eight samples per model render and filter to sample eight without index drift');
    runs.push(...Array.from({ length: 25 }, (_, index) => ({ ...pending, id: `page-run-${index}`, status: 'completed' })));
    await page.locator('#refresh').click();
    await page.waitForFunction(() => !document.getElementById('refresh').hasAttribute('aria-busy'));
    await history();
    const firstHistory = await page.locator('#history-list').textContent();
    await page.locator('#history-next').click();
    await page.waitForFunction((previous) => document.getElementById('history-list').textContent !== previous, firstHistory);
    assert.equal(await page.locator('[data-run]').count(), 10);
    await page.locator('#history-prev').click();
    await page.waitForFunction((previous) => document.getElementById('history-list').textContent === previous, firstHistory);
    robustnessChecks.push('history pagination advances and returns without mixing task records');
    await page.locator('[data-run=polling-run]').first().click();
    await page.setViewportSize({ width: 390, height: 844 });
    await page.evaluate(() => scrollTo(0, 0));
    await page.screenshot({
      animations: 'disabled',
      path: path.join(output, 'mobile-page.png'),
      fullPage: true,
    });
    assert.equal(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      true,
      'Mobile page must not overflow horizontally',
    );
    await operations();
    assert.equal(await page.locator('#page-operations').isVisible(), true);
    await page.locator('[data-page=manual]').click();
    await setup();
    await capture('mobile-setup-page.png');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.setViewportSize({ width: 820, height: 1180 });
    await capture('tablet-page.png');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    robustnessChecks.push('mobile and tablet task navigation have no horizontal page overflow');

    await page.setViewportSize({ width: 1440, height: 1080 });
    const disconnectGate = deferred();
    const disconnectRequested = deferred();
    await page.route('**/targets?limit=100', async (route) => {
      disconnectRequested.resolve();
      await disconnectGate.promise;
      await route.continue().catch(() => {});
    });
    await page.locator('#refresh').click();
    await disconnectRequested.promise;
    await page.locator('#connection-top').click();
    await page.locator('#disconnect').click();
    disconnectGate.resolve();
    await page.unroute('**/targets?limit=100');
    await page.waitForFunction(() => document.getElementById('connection-state').textContent === '未连接');
    assert.equal(await page.locator('#target option').count(), 1);
    assert.equal(await page.locator('#compare-model-a option,#compare-model-b option,#sample-filter option').count(), 0);
    assert.equal(await page.locator('.sample').count(), 0);
    assert.equal(await page.locator('#run-submit').isDisabled(), true);
    assert(!(await page.locator('body').textContent()).includes('mock-credential-a'));
    robustnessChecks.push(
      'disconnect aborts in-flight reads and removes credentials, history, outputs and dialogs',
    );

    await page.locator('#connection-top').click();
    await page.locator('#management-key').fill('offline-test-key');
    await page.locator('#connection-form button[type=submit]').click();
    await page.waitForFunction(() => document.getElementById('connection-state').textContent === '已连接');
    await page.route('**/metrics', (route) =>
      route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"expired"}' }),
    );
    await page.locator('#refresh').click();
    await page.waitForFunction(() => document.getElementById('connection-state').textContent === '未连接');
    assert.equal(await page.locator('#target option').count(), 1);
    assert((await page.locator('#notice').textContent()).includes('鉴权已失效'));
    await page.unroute('**/metrics');
    robustnessChecks.push('401 invalidates the whole management session without stale data');
    assert.equal(await page.evaluate(() => localStorage.length + sessionStorage.length), 0);
    assert.deepEqual(errors, [], 'No browser JavaScript errors');
    assert.deepEqual(sourceHashes(), initialSourceHashes, 'UI source must remain unchanged while browser validation is running');
    fs.writeFileSync(path.join(output, 'dialog-layout-results.json'), JSON.stringify({ result: 'passed', source_sha256: initialSourceHashes, browser: await browser.version(), scope: 'Real computed dialog bottom padding and scrolled content inset at 360/390px; safe-area inset is zero in desktop emulation', checks: dialogGeometry }, null, 2) + '\n');
    robustnessChecks.push('all five product dialogs retain at least 20px computed bottom padding and visible content inset at 360px and 390px');
    const report = {
      result: 'passed',
      verified_at: new Date().toISOString(),
      source_sha256: initialSourceHashes,
      viewport_evidence: viewportEvidence,
      browser: await browser.version(),
      platform: process.platform,
      architecture: process.arch,
      checks: [
        'static shell no API/model calls',
        'authenticated read-only connection',
        'memory-only key',
        'manual fixed-auth request',
        'stream option',
        'human review',
        'strict rerun config',
        'JSON export excludes management key',
        'random control selection excludes self',
        'running cancellation',
        'preset save and load no execution',
        'rule save no execution',
        'server audit history',
        'separate rule pause',
        'settings preserve immutable fields',
        'held target disables manual and rerun execution',
        'held target disables rule run and resume',
        'paused control and judge disable submit',
        'removed automatic recovery UI and payload fields',
        'manual release always keeps dependent rules paused',
        'action preview and CAS release',
        'operator disable retained',
        'HTML text escaping',
        'sandbox script and navigation suppression',
        'host CSP permits sandbox CSS',
        'responsive mobile layout',
        'no browser errors',
        ...robustnessChecks,
      ],
      api_calls: calls.length,
      screenshots: fs.readdirSync(output).filter((name) => name.endsWith('.png')).sort(),
      screenshot_sha256: Object.fromEntries(fs.readdirSync(output).filter((name) => name.endsWith('.png')).sort().map((name) => [name, require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(output, name))).digest('hex')])),
    };
    fs.writeFileSync(path.join(output, 'ui-results.json'), JSON.stringify(report, null, 2) + '\n');
    console.log(JSON.stringify(report, null, 2));
  } catch (error) {
    await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true }).catch(() => {});
    fs.writeFileSync(path.join(output, 'failure.json'), JSON.stringify({ error: error.stack, browserErrors: errors, lastCalls: calls.slice(-6), text: await page.locator('body').innerText() }, null, 2));
    throw error;
  } finally {
    await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
