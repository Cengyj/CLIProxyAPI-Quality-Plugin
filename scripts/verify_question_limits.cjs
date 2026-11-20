// Exercise both editors against the local fixture. All writes are answered in memory.
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const output = process.env.UI_OUTPUT;
  if (output) fs.mkdirSync(output, { recursive: true });
  const checks = [], writes = [], errors = [];
  let browser, run;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const info = JSON.parse(line), origin = new URL(info.url).origin;
    assert.equal(info.pid, fixture.pid);
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' });
    page.setDefaultTimeout(5000);
    page.on('pageerror', error => errors.push(error.message));
    await page.route('**/*', async route => {
      const request = route.request(), url = new URL(request.url());
      assert.equal(url.origin, origin);
      if (request.method() === 'GET') {
        if (run && url.pathname.endsWith('/runs/detail') && url.searchParams.get('id') === run.id) return route.fulfill({ json: run });
        return route.continue();
      }
      const api = url.pathname.split('/').pop();
      assert.equal(request.method(), 'POST');
      assert(['runs', 'rules'].includes(api));
      const body = request.postDataJSON();
      writes.push({ api, body });
      if (Buffer.byteLength(body.prompt) > 16384 || Buffer.byteLength(body.review_standard || '') > 4096) {
        return route.fulfill({ status: 400, json: { error: { message: 'custom input exceeds fixture byte limit' } } });
      }
      if (api === 'rules') return route.fulfill({ json: { id: 'limit-rule' } });
      run = { id: 'limit-' + writes.length, status: 'completed', source: 'manual_test', config: body, samples: [] };
      return route.fulfill({ status: 202, json: { run_id: run.id } });
    });
    await page.goto(origin + '/embedded.html?demo=1&page=manual');
    const ui = page.frameLocator('#fixture-plugin');
    const field = id => ui.locator('#' + id);
    await ui.locator('body.connected').waitFor();
    const version = JSON.parse(await field('build-info').textContent()).ui_version;
    const nearLimit = '题'.repeat(5461);
    const limits = async (id, button) => {
      const input = field(id);
      for (const [width, height, theme] of [[1440, 900, 'light'], [1280, 640, 'white'], [390, 844, 'dark'], [320, 680, 'light']]) {
        await page.setViewportSize({ width, height });
        await page.selectOption('#fixture-theme', theme);
        if (id === 'prompt' && width < 600) await ui.locator('[data-manual-pane=setup]').click();
        // The embedded document updates its workbench height on the next animation frame.
        await input.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        await input.fill(nearLimit);
        await input.evaluate(node => node.scrollIntoView({ block: 'center' }));
        await input.press('End');
        await page.keyboard.insertText('题');
        await input.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        const observed = await input.evaluate(node => {
          const error = document.getElementById(node.id + '-error');
          const scroll = node.closest('.rule-form-scroll, #run-form');
          const rect = error?.getBoundingClientRect(), visible = scroll.getBoundingClientRect();
          return {
            bytes: new TextEncoder().encode(node.value).length,
            error: error?.textContent || '', invalid: node.getAttribute('aria-invalid') === 'true',
            errorVisible: !!rect && rect.top >= visible.top && rect.bottom <= visible.bottom,
            focused: document.activeElement === node, scroll: scroll.scrollTop,
            fieldY: Math.round(node.getBoundingClientRect().y), overflow: document.documentElement.scrollWidth > window.innerWidth,
          };
        });
        const rect = await button.boundingBox();
        checks.push({ id, width, height, theme, ...observed, buttonDisabled: await button.isDisabled(), buttonX: Math.round(rect.x), buttonY: Math.round(rect.y) });
        if (output) await page.screenshot({ path: path.join(output, id + '-' + width + '.png') });
      }
      await page.setViewportSize({ width: 1440, height: 900 });
    };
    await field('target').selectOption('fixture-quality-primary');
    await field('models').selectOption('fixture-reasoner');
    // Editing a built-in question must detach its frozen answer.
    await field('question').selectOption('logic-candy');
    const start = field('run-submit');
    await limits('prompt', start);
    if (await start.isEnabled()) await start.click();
    else await field('run-form').evaluate(node => node.requestSubmit());
    await ui.locator('#run-submit[aria-busy=false]').waitFor();
    const invalidRuns = writes.filter(w => w.api === 'runs').length;
    for (const prompt of [nearLimit, 'x'.repeat(16384)]) {
      await field('prompt').fill(prompt);
      const sent = page.waitForRequest(req => req.method() === 'POST' && req.url().endsWith('/runs'));
      await start.click();
      await sent;
      await ui.locator('#run-submit:enabled').waitFor();
    }
    await ui.locator('[data-page=operations]').click();
    await field('rule-new').click();
    await field('rule-name').fill('Synthetic byte limit');
    await field('rule-question').selectOption('custom');
    const save = ui.locator('#rule-form button[type=submit]');
    await limits('rule-prompt', save);
    await save.click();
    await ui.locator('#rule-form button[type=submit]:not([aria-busy=true])').waitFor();
    const invalidRules = writes.filter(w => w.api === 'rules').length;
    const promptFocusedAfterSave = await field('rule-prompt').evaluate(node => document.activeElement === node);
    const prompt = ' ' + nearLimit, standard = '😀'.repeat(1024);
    await field('rule-prompt').fill(prompt);
    await field('rule-review-standard').fill(standard + '!');
    await save.click();
    await ui.locator('#rule-form button[type=submit]:not([aria-busy=true])').waitFor();
    const invalidStandards = writes.filter(w => w.api === 'rules').length - invalidRules;
    const standardFocusedAfterSave = await field('rule-review-standard').evaluate(node => document.activeElement === node);
    await field('rule-review-standard').fill(standard);
    await save.click();
    await field('rule-dialog').waitFor({ state: 'hidden' });
    const storedRule = writes.at(-1).body;
    const validManual = writes.filter(w => w.api === 'runs').slice(invalidRuns).map(w => w.body);
    const report = { version, checks, invalidRuns, invalidRules, invalidStandards, promptFocusedAfterSave, standardFocusedAfterSave,
      acceptedManualBytes: validManual.map(body => Buffer.byteLength(body.prompt)),
      acceptedRuleBytes: Buffer.byteLength(storedRule.prompt), ruleTextPreserved: storedRule.prompt === prompt,
      acceptedStandardBytes: Buffer.byteLength(storedRule.review_standard), pageErrors: errors };
    if (output) fs.writeFileSync(path.join(output, 'question-limits.json'), JSON.stringify(report, null, 2) + '\n');
    assert.deepEqual([invalidRuns, invalidRules, invalidStandards], [0, 0, 0]);
    assert(promptFocusedAfterSave && standardFocusedAfterSave);
    for (const entry of checks) {
      assert.equal(entry.bytes, 16386);
      assert(entry.focused && entry.invalid && entry.errorVisible && !entry.overflow, entry.id + ' at ' + entry.width);
      assert.match(entry.error, /16,?384/);
      assert.match(entry.error, /2\s*字节/);
      if (entry.id === 'prompt') assert(entry.buttonDisabled);
      assert(entry.buttonX >= 0 && entry.buttonX < entry.width && entry.buttonY >= 0 && entry.buttonY < entry.height);
    }
    assert.deepEqual(validManual.map(body => [body.question_id, body.prompt]), [['custom', nearLimit], ['custom', 'x'.repeat(16384)]]);
    assert.equal(storedRule.prompt, prompt);
    assert.equal(storedRule.review_standard, standard);
    assert.deepEqual(errors, []);
    console.log('Custom question UTF-8 limits, exact payloads, focus and embedded layouts passed.');
  } finally {
    await browser?.close();
    fixture.kill();
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
