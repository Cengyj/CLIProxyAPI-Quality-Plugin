// Exercise the account page's first paint and connection lifecycle on a loopback fixture.
const assert = require('node:assert/strict');
const path = require('node:path');
const { spawn } = require('node:child_process');
const { once } = require('node:events');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

const api = '/v0/management/plugins/model-quality';
const storedKey = `model-quality:management-key:${api}`;
const accountRow = '[data-account-row="fixture-enabled"]';
const hostPage = '/management.html#/plugin-pages/model-quality/0';

function nextRead(page, resource) {
  return page.waitForRequest(request => new URL(request.url()).pathname === api + resource);
}

function rememberKey(page, key) {
  return page.addInitScript(([name, value]) => {
    if (window.top === window) localStorage.setItem(name, value);
  }, [storedKey, key]);
}

async function checkShell(view) {
  await view.locator('#login-view-accounts').waitFor();
  assert(await view.locator('#account-import-open').isVisible());
  assert.equal(await view.locator('#login-disconnected, .login-connect-intro, .login-connect-guide').count(), 0);
  assert.equal(await view.locator('#page-login #login-connect').count(), 0);
  assert.doesNotMatch(await view.locator('#page-login').innerText(), /连接后可|连接管理接口|未连接/);
}

async function checkWaiting(view) {
  await checkShell(view);
  assert(await view.locator('#account-import-open').isDisabled());
  assert(await view.locator('#account-empty').isVisible());
  assert(await view.locator('#account-import-empty').isDisabled());
  assert.doesNotMatch(await view.locator('#login-view-accounts').innerText(), /0\s*个账号|全部状态\s*·\s*0/);
  assert.equal(await view.locator('[data-account-row]').count(), 0);
}

async function checkReturning(view) {
  await checkShell(view);
  assert(await view.locator(accountRow).isVisible());
  assert.equal(await view.locator('#connection-state').innerText(), '已连接');
  assert(await view.locator('#connection-top .connection-dot').isVisible());
  assert(await view.locator('#connection-dialog').isHidden());
}

async function checkSavedConnection(view) {
  assert.doesNotMatch(await view.locator('body').innerText(), /正在连接管理接口/);
}

async function emptyAccountLayout(view, connectionLabel) {
  const pill = view.getByRole('button', { name: connectionLabel, exact: true });
  assert(await view.locator('#account-empty').isVisible());
  assert(await view.locator('#account-list-summary').isHidden());
  assert.doesNotMatch(await view.locator('body').innerText(), /正在读取(?:账号|数据)|正在连接管理接口/);
  assert(await pill.evaluate(button => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) return false;
    return [button, button.querySelector('.connection-dot')].every(element => {
      const style = getComputedStyle(element);
      return style.animationName === 'none' && [style.animationDuration, style.transitionDuration, style.transitionDelay]
        .every(durations => durations.split(',').every(duration => parseFloat(duration) === 0));
    });
  }), 'The connection button and its status dot have no animation or transition with normal motion settings');
  const rects = { connection: await pill.boundingBox() };
  for (const selector of ['#login-workbench', '#account-empty', '#account-import-empty', '.account-toolbar',
    '#account-status-filter', '#account-search', '#account-import-open', '#account-convert-open',
    '#account-inspect', '#account-history-open', '#account-settings-open', '#refresh']) {
    rects[selector] = await view.locator(selector).boundingBox();
    assert(rects[selector], selector + ' must be visible in the account workspace');
  }
  return rects;
}

function changeVisibility(page, hidden) {
  // Headless Chromium keeps every tab visible. Deliver the browser event with the corresponding state.
  return page.evaluate(hidden => {
    Object.defineProperty(document, 'hidden', { configurable: true, value: hidden });
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: hidden ? 'hidden' : 'visible' });
    document.dispatchEvent(new Event('visibilitychange'));
  }, hidden);
}

async function checkPluginMotion(view) {
  const moving = await view.locator('html').evaluate(root => {
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) throw new Error('Expected normal motion settings');
    const reported = [];
    for (const element of [root, ...root.querySelectorAll('*')]) {
      for (const pseudo of [null, '::before', '::after']) {
        const style = getComputedStyle(element, pseudo);
        const timed = style.transitionProperty !== 'none' && style.transitionDuration.split(',').some(time => parseFloat(time) > 0);
        if (timed || style.animationName !== 'none' || style.scrollBehavior === 'smooth') {
          reported.push((element.id || element.className || element.tagName) + (pseudo || ''));
        }
      }
    }
    return reported.slice(0, 10);
  });
  assert.deepEqual(moving, [], 'Plugin controls and pseudo-elements should update immediately');
}

async function main() {
  const fixture = spawn(process.execPath, [path.join(__dirname, 'login_ui_fixture.cjs')], { stdio: ['ignore', 'pipe', 'inherit'] });
  const exited = once(fixture, 'exit');
  let browser;
  try {
    const [line] = await once(createInterface({ input: fixture.stdout }), 'line');
    const { url, management_key: key } = JSON.parse(line);
    const { origin, hostname } = new URL(url);
    assert.equal(hostname, '127.0.0.1');
    browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' });

    async function scenario(name, test, { responses = {}, ...options } = {}) {
      const context = await browser.newContext(options);
      const errors = [], reads = [], holds = new Map(), releases = [];
      try {
        const page = await context.newPage();
        page.setDefaultTimeout(6000);
        page.on('pageerror', error => errors.push(error.message));
        await context.route('**/*', async route => {
          const request = route.request(), url = new URL(request.url());
          if (url.origin !== origin || request.method() !== 'GET') {
            errors.push(`${request.method()} ${url.pathname}`);
            return route.abort();
          }
          if (url.pathname.startsWith(api + '/')) {
            const resource = url.pathname.slice(api.length);
            if (![`Bearer ${key}`, 'Bearer expired-fixture-key'].includes(request.headers().authorization)) {
              errors.push('Missing or unexpected test authorization on ' + resource);
            }
            reads.push(resource);
            if (holds.has(resource)) await holds.get(resource);
            if (request.headers().authorization === `Bearer ${key}` && Object.hasOwn(responses, resource)) {
              return route.fulfill({ json: responses[resource] });
            }
          }
          return route.continue();
        });
        const hold = (...resources) => {
          let release;
          const pending = new Promise(resolve => { release = resolve; });
          for (const resource of resources) holds.set(resource, pending);
          releases.push(release);
          return release;
        };
        await test(page, hold, reads);
        assert.deepEqual(errors, []);
        console.log(`Passed: ${name}`);
      } finally {
        for (const release of releases) release();
        await context.close();
      }
    }

    await scenario('account workspace in the first HTML paint', async page => {
      await page.goto(origin);
      await checkWaiting(page);
      assert(await page.getByRole('button', { name: '未连接', exact: true }).isVisible());
    }, { javaScriptEnabled: false });

    await scenario('account controls, connection dialog and quality tabs stay still', async page => {
      await page.goto(origin + hostPage);
      const view = page.frameLocator('#fixture-plugin');
      await checkWaiting(view);
      await view.getByRole('button', { name: '未连接', exact: true }).click();
      await checkPluginMotion(view);
      await view.locator('#management-key').fill(key);
      await view.locator('#connection-form button[type=submit]').click();
      await view.locator('#account-import-open:enabled').waitFor();
      await checkPluginMotion(view);
      for (const section of ['manual', 'operations']) {
        await view.locator(`[data-page="${section}"]`).click();
        await view.locator(`#page-${section}`).waitFor();
        await checkPluginMotion(view);
      }
    }, { reducedMotion: 'no-preference' });

    await scenario('embedded empty account layout stays in place during authentication', async (page, hold, reads) => {
      await page.goto(origin + hostPage);
      const view = page.frameLocator('#fixture-plugin');
      await view.locator('body.embedded.host-themed').waitFor();
      await checkWaiting(view);
      const guest = await emptyAccountLayout(view, '未连接');
      assert.deepEqual(reads, []);

      await rememberKey(page, key);
      const release = hold('/settings', '/login/accounts');
      const requested = nextRead(page, '/login/accounts');
      await page.reload();
      await requested;
      await checkWaiting(view);
      assert.deepEqual(await emptyAccountLayout(view, '未连接'), guest);
      release();
      await view.locator('#account-import-empty:enabled').waitFor();
      assert.equal(await view.locator('#account-status-filter option:checked').textContent(), '全部状态 · 0');
      assert.deepEqual(await emptyAccountLayout(view, '已连接'), guest);
    }, {
      reducedMotion: 'no-preference',
      responses: { '/login/accounts': { items: [] }, '/login/jobs': { items: [] } },
    });

    for (const expired of [false, true]) {
      await scenario(expired ? 'expired key and manual reconnect' : 'first visit and manual connect', async (page, hold, reads) => {
        if (expired) await rememberKey(page, 'expired-fixture-key');
        const rejected = expired && page.waitForResponse(response => response.url().includes(api + '/') && response.status() === 401);
        await page.goto(expired ? origin : origin + hostPage);
        const view = expired ? page : page.frameLocator('#fixture-plugin');
        if (expired) {
          await rejected;
          await page.waitForFunction(name => localStorage.getItem(name) === null, storedKey);
        } else assert.deepEqual(reads, []);
        await checkWaiting(view);
        const pill = view.getByRole('button', { name: '未连接', exact: true });
        assert.equal(await view.locator('#connection-open:visible,#connection-top:visible').count(), 1);
        if (!expired) assert(await pill.evaluate(button => {
          const box = button.getBoundingClientRect();
          return box.x > innerWidth / 2 && box.top >= 0 && box.top < 100 && box.right <= innerWidth;
        }), 'The embedded guest connection pill is visible at the top right');
        await pill.click();
        await view.locator('#management-key').fill(key);
        await view.locator('#connection-form button[type=submit]').click();
        await view.locator(accountRow).waitFor();
        await view.locator('#account-import-open:enabled').waitFor();
        await checkReturning(view);
      });
    }

    await scenario('embedded account list loads independently of secondary reads', async (page, hold, reads) => {
      const releaseAuth = hold('/settings');
      const releaseAccounts = hold('/login/accounts');
      const releaseSettings = hold('/login/accounts/settings');
      const releaseQuality = hold('/targets', '/questions', '/rules', '/metrics', '/runs');
      const authStarted = nextRead(page, '/settings');
      const accountsStarted = nextRead(page, '/login/accounts');
      await page.goto(origin + '/embedded.html?demo=1');
      const view = page.frameLocator('#fixture-plugin');
      await Promise.all([authStarted, accountsStarted]);
      await checkWaiting(view);
      await checkSavedConnection(view);
      releaseAccounts();
      await view.locator(accountRow).waitFor();
      assert(await view.locator('#account-import-open').isDisabled());
      releaseAuth();
      releaseSettings();
      await view.locator('#account-import-open:enabled').waitFor();
      assert.deepEqual(reads.filter(resource => !resource.startsWith('/login/')), ['/settings']);
      releaseQuality();
      await view.locator('[data-page=manual]').click();
      await view.locator('#target option[value="fixture-quality-primary"]').waitFor({ state: 'attached' });
    });

    await scenario('plugin tab revisits and browser visibility during a slow poll', async (page, hold, reads) => {
      await rememberKey(page, key);
      await page.goto(origin);
      await page.locator('#account-import-open:enabled').waitFor();
      await page.click('[data-page=manual]');
      await page.locator('#target option[value="fixture-quality-primary"]').waitFor({ state: 'attached' });
      await page.click('[data-page=login]');
      await page.locator('#account-import-open:enabled').waitFor();

      const count = path => reads.filter(resource => resource === path).length;
      const previousSettings = count('/settings');
      const previousAccounts = count('/login/accounts');
      const release = hold('/login/accounts');
      await nextRead(page, '/login/accounts');
      for (const tab of ['manual', 'operations', 'manual']) {
        await page.click(`[data-page=${tab}]`);
        await page.click('[data-page=login]');
        await checkReturning(page);
      }
      for (const hidden of [true, false, true, false]) await changeVisibility(page, hidden);
      await checkReturning(page);
      assert.equal(count('/settings'), previousSettings);
      assert.equal(count('/login/accounts'), previousAccounts + 1, 'Tab and visibility changes share the pending account poll');
      release();
      await page.locator('#account-import-open:enabled').waitFor();
    });

    await scenario('CPA sidebar returns recreate the iframe, read accounts first and enforce expiry', async (page, hold, reads) => {
      await rememberKey(page, key);
      const firstValidation = nextRead(page, '/settings');
      await page.goto(origin + hostPage);
      const frame = page.frameLocator('#fixture-plugin');
      await firstValidation;
      await frame.locator('#account-import-open:enabled').waitFor();
      const count = path => reads.filter(resource => resource === path).length;
      const leavePlugin = async () => {
        await page.getByRole('link', { name: '认证文件' }).click();
        await page.locator('#fixture-plugin').waitFor({ state: 'detached' });
      };
      const returnToPlugin = () => page.getByRole('link', { name: 'Credential Ops' }).click();
      for (let index = 0; index < 2; index++) {
        await leavePlugin();
        const releaseQuality = hold('/settings');
        const release = hold('/login/accounts');
        const previousSettings = count('/settings'), previousAccounts = count('/login/accounts');
        const validation = nextRead(page, '/settings');
        const requested = nextRead(page, '/login/accounts');
        await returnToPlugin();
        await Promise.all([requested, validation]);
        await checkWaiting(frame);
        await checkSavedConnection(frame);
        release();
        await frame.locator(accountRow).waitFor();
        assert.equal(count('/settings'), previousSettings + 1);
        assert.equal(count('/login/accounts'), previousAccounts + 1);
        releaseQuality();
        await frame.locator('#account-import-open:enabled').waitFor();
      }

      await leavePlugin();
      await page.evaluate(name => localStorage.setItem(name, 'expired-fixture-key'), storedKey);
      const unauthorized = page.waitForResponse(response => response.url().includes(api + '/') && response.status() === 401);
      await returnToPlugin();
      assert.equal((await unauthorized).request().headers().authorization, 'Bearer expired-fixture-key');
      await page.waitForFunction(name => localStorage.getItem(name) === null, storedKey);
      await frame.getByRole('button', { name: '未连接', exact: true }).waitFor();
      await checkWaiting(frame);
      assert.equal(await page.evaluate(name => localStorage.getItem(name), storedKey), null);
    });

    await scenario('persisted navigation restores with authenticated reads', async (page, hold) => {
      await rememberKey(page, key);
      await page.goto(origin);
      await page.locator('#account-import-open:enabled').waitFor();
      const release = hold('/login/accounts');
      const requested = nextRead(page, '/login/accounts');
      await page.evaluate(() => {
        dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
        dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
      });
      await requested;
      await checkShell(page);
      await checkSavedConnection(page);
      release();
      await page.locator(accountRow).waitFor();
    });

    for (const section of ['manual', 'operations']) {
      await scenario(`direct link to ${section}`, async (page, hold, reads) => {
        await rememberKey(page, key);
        await page.goto(`${origin}/#${section}`);
        await page.locator('body.connected').waitFor();
        assert(await page.locator(`#page-${section}`).isVisible());
        assert(await page.locator('#page-login').isHidden());
        assert.equal(reads.includes('/login/accounts'), false);
      });
    }
  } finally {
    try { if (browser) await browser.close(); } finally { fixture.kill(); await exited; }
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
