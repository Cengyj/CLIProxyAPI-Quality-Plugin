// Runs inside verify_workspace_ui's GET-only synthetic host and iframe.
const assert = require('node:assert/strict');
const path = require('node:path');

module.exports = async function checkSecurityDialog({ page, frame, origin, out, report }) {
  const row = frame.locator('[data-account-row="fixture-enabled"]');
  const opener = row.locator('[data-account-action="reset_totp"]');
  const more = row.locator('[data-account-action="more"]');
  const dialog = frame.locator('#account-security-dialog');
  const get = name => dialog.locator(`#account-security-${name}`);
  const focused = loc => loc.evaluate(el => el === document.activeElement);
  const activeSteps = () => dialog.locator('.account-security-flow-step:visible strong').allTextContents();
  const shot = async name => {
    await dialog.screenshot({ path: path.join(out, `${name}.png`), animations: 'disabled' });
    report.screenshots.push(`${name}.png`);
  };
  const layout = async label => {
    const info = await dialog.evaluate(el => {
      const rect = n => n.getBoundingClientRect();
      const box = rect(el), scroller = el.querySelector('.account-security-scroll');
      const footer = rect(el.querySelector('.account-security-footer'));
      const count = el.querySelector('#account-security-plan-hint');
      const summary = el.querySelector('#account-security-summary');
      const choices = [...el.querySelectorAll('.account-security-choice')];
      const [a, b] = choices.map(rect);
      const stacked = b.top >= a.bottom && Math.abs(a.left - b.left) < 1;
      const impact = rect(el.querySelector('#account-security-impact')), options = rect(el.querySelector('.account-security-plan'));
      const heading = rect(el.querySelector('#account-security-title'));
      const text = el.querySelectorAll('.account-security-heading-text, #account-security-identity, #account-security-impact, .account-security-step-content, #account-security-plan-hint, #account-security-summary');
      return { width: box.width, height: box.height, viewport: [innerWidth, innerHeight],
        inside: box.left >= 0 && box.right <= innerWidth && box.top >= 0 && box.bottom <= innerHeight + 1,
        unclipped: [el, scroller, ...text].every(n => n.scrollWidth <= n.clientWidth + 1),
        readableHeading: heading.width >= 80, stacked, aligned: heading.bottom <= impact.top && impact.bottom <= options.top && options.top <= a.top && stacked && Math.abs(a.width - b.width) < 1,
        fullTargets: choices.every(choice => {
          const label = choice.querySelector('label'), input = choice.querySelector('input'), r = rect(label), hit = rect(input), c = rect(choice);
          return r.height >= 44 && r.width >= c.width - 2 && r.height >= c.height - 2 && label.control === input && hit.left >= r.left && hit.right <= r.right;
        }),
        countUnclipped: count.scrollWidth <= count.clientWidth,
        flowVisible: rect(summary).top >= footer.top && rect(summary).bottom <= footer.bottom && summary.offsetHeight > 0,
        footerTop: footer.top, footerVisible: Math.abs(box.bottom - footer.bottom) <= 2 && footer.top >= box.top && [...el.querySelectorAll('.account-security-footer button')].every(button => {
          const r = rect(button); return button.contains(document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2)) && (innerWidth > 600 || r.height >= 44);
        }),
        scrollable: scroller.scrollHeight > scroller.clientHeight + 1, emailHeight: el.querySelector('#account-security-identity').offsetHeight };
    });
    assert(info.readableHeading && info.inside && info.unclipped && info.footerVisible && info.flowVisible && info.countUnclipped && info.aligned && info.fullTargets, `${label}: ${JSON.stringify(info)}`);
    report.layouts.push({ label, ...info });
    return info;
  };
  const defaults = async () => {
    assert.equal(await dialog.getByRole('switch').count(), 3);
    assert(!await get('logout').isChecked() && !await get('reauthorize').isChecked() && await get('upload').isDisabled());
    assert(await focused(get('cancel')));
    assert.equal(await dialog.getByRole('status', { name: '确认后' }).locator('.reset-execution-speech').textContent(), '仅重置 2FA');
    assert.equal(await get('plan-hint').innerText(), '已选 1 项');
    assert(await get('step-count').isHidden());
    assert(!await get('notes').evaluate(el => el.open));
  };
  for (const [width, height, theme] of [[1064, 850, 'light'], [1064, 850, 'white'], [1064, 850, 'dark'], [620, 800, 'white'], [600, 800, 'light'], [550, 800, 'white'], [510, 800, 'light'], [390, 844, 'light'], [320, 750, 'light']]) {
    await page.setViewportSize({ width, height });
    await page.locator('#fixture-theme').selectOption(theme);
    await frame.locator(`html[data-theme="${theme}"][data-theme-source="host"]`).waitFor({ state: 'attached' });
    await opener.click(); await defaults();
    assert(await get('identity').isHidden());
    assert.equal(await get('identity').textContent(), '');
    const name = `reset-${width}-${theme}`;
    await page.mouse.move(0, 0);
    await shot(name);
    if (width === 1064 && theme !== 'white') {
      await get('logout-option').hover(); await shot(`${name}-hover`);
      await get('reauthorize-option').hover(); await shot(`${name}-selected-hover`);
      await page.mouse.move(0, 0);
    }
    if (width === 1064 || width === 390) {
      await get('notes').locator('summary').click();
      assert(await get('notes').evaluate(el => el.open));
      assert.match(await get('notes').innerText(), /不会恢复旧密钥/);
      await layout(`${name}/details`);
      await shot(`${name}-details`);
      await get('notes').locator('summary').click();
    }
    const label = dialog.locator('label[for="account-security-logout"]');
    await label.click({ position: { x: 8, y: 6 } }); assert(await get('logout').isChecked());
    await dialog.getByText('退出所有设备', { exact: true }).click(); assert(!await get('logout').isChecked());
    let dimensions;
    for (const [logout, reauthorize, upload] of [[false, true, false], [true, true, true], [true, false, false], [false, false, false]]) {
      await get('logout').setChecked(logout); await get('reauthorize').setChecked(reauthorize);
      if (reauthorize) await get('upload').setChecked(upload);
      else assert(await get('upload').isDisabled() && !await get('upload').isChecked());
      const expected = ['重置 2FA', ...(logout ? ['退出设备'] : []), ...(reauthorize ? ['重登授权'] : []), ...(upload ? ['上传 CPA'] : [])];
      assert.equal(await get('summary').locator('.reset-execution-speech').textContent(), expected.length === 1 ? '仅重置 2FA' : expected.join(' → '));
      assert.equal(await get('plan-hint').innerText(), `已选 ${expected.length} 项`);
      assert.equal(await get('summary').locator('.reset-execution-step').count(), expected.length);
      assert.deepEqual(await get('summary').locator('.reset-execution-step strong').allTextContents(), expected.length === 1 ? ['仅重置 2FA'] : expected);
      assert.deepEqual(await get('summary').locator('.reset-execution-step').evaluateAll(nodes => nodes.map(n => n.dataset.order)), expected.map((_, i) => String(i + 1)));
      if (logout && reauthorize && (width === 1064 && theme === 'light' || width === 390)) await shot(`${name}-full-flow`);
      for (const [id, checked] of [['logout', logout], ['reauthorize', reauthorize], ['upload', upload]]) {
        assert.equal(await get(`${id}-option`).getAttribute('data-selected'), String(checked));
      }
      const info = await layout(`${name}/${+logout}${+reauthorize}`);
      dimensions ??= [info.width, info.height, info.footerTop];
      assert.deepEqual([info.width, info.height, info.footerTop], dimensions, 'Changing the plan must not move the dialog or footer');
    }
    await get('cancel').click(); assert(await focused(opener));
  }
  await opener.click(); await defaults();
  await page.keyboard.press('Shift+Tab'); assert(await focused(get('notes').locator('summary')));
  await page.keyboard.press('Enter'); assert(await get('notes').evaluate(el => el.open));
  assert(!await get('logout').isChecked() && !await get('reauthorize').isChecked());
  await page.keyboard.press('Space'); assert(!await get('notes').evaluate(el => el.open));
  await page.keyboard.press('Shift+Tab'); assert(await focused(get('reauthorize')));
  await page.keyboard.press('Shift+Tab'); assert(await focused(get('logout')));
  assert(await get('logout-option').evaluate(el => getComputedStyle(el).outlineStyle !== 'none'));
  await page.keyboard.press('Space'); assert(await get('logout').isChecked());
  assert.equal(await get('summary').locator('.reset-execution-speech').textContent(), '重置 2FA → 退出设备');
  await page.keyboard.press('Escape'); assert(await focused(opener));
  await opener.click();
  assert(await focused(get('cancel')));
  assert(await get('logout').isChecked());
  await get('logout').uncheck();
  await get('cancel').focus();
  await defaults();
  await get('notes').locator('summary').click();
  await get('close').click(); assert(await focused(opener));
  await opener.click(); await defaults(); await get('cancel').click();
  const longEmail = `${'a'.repeat(60)}@${'b'.repeat(50)}.${'c'.repeat(40)}.example.test`;
  const listURL = `${origin}/v0/management/plugins/model-quality/login/accounts`;
  let revisionDelta = 0;
  const customize = async route => {
    assert.equal(route.request().method(), 'GET');
    const response = await route.fetch({ maxRedirects: 0 }), json = await response.json();
    const account = json.items.find(item => item.id === 'fixture-enabled');
    account.email = longEmail; account.revision += revisionDelta;
    await route.fulfill({ response, json });
  };
  await page.route(listURL, customize);
  try {
    const reload = () => frame.evaluate(() => window.dispatchEvent(new Event('online')));
    await reload(); await row.getByText(longEmail, { exact: true }).first().waitFor();
    await page.setViewportSize({ width: 320, height: 430 }); await opener.click(); await defaults();
    assert(await get('identity').isHidden());
    assert.equal(await get('identity').textContent(), '');
    const short = await layout('reset-long-email/short-viewport');
    assert(short.scrollable && short.emailHeight === 0);
    await shot('reset-short-long-email');
    await get('notes').locator('summary').click();
    assert((await layout('reset-long-email/short-viewport/details')).scrollable);
    await shot('reset-short-details');
    revisionDelta++; await reload(); await get('error').waitFor();
    assert.match(await dialog.getByRole('alert').innerText(), /账号状态已变化/);
    assert(await get('submit').isDisabled() && await get('logout').isDisabled() && await get('reauthorize').isDisabled());
    assert(await get('error').evaluate(el => {
      const r = el.getBoundingClientRect(), s = el.closest('.account-security-scroll').getBoundingClientRect();
      return r.top >= s.top && r.bottom <= s.bottom + 1;
    }));
    await layout('reset-short-viewport/error'); await shot('reset-short-error');
    await get('cancel').click(); assert(await focused(opener));
  } finally { await page.unroute(listURL, customize); }
  await page.setViewportSize({ width: 1064, height: 850 });
  await more.click(); await frame.locator('#account-logout-all').click();
  assert.equal(await get('title').innerText(), '退出所有设备');
  assert.equal(await dialog.getByRole('switch').count(), 0);
  assert(await get('logout-option').isHidden() && await get('summary').isHidden() && await get('impact').isHidden());
  assert(await get('notes').isHidden());
  assert.equal(await get('plan-hint').innerText(), '可选');
  assert.deepEqual(await activeSteps(), ['退出设备', '重登授权', '上传 CPA']);
  await shot('logout-independent');
  await get('reauthorize').uncheck(); assert.equal(await get('step-count').innerText(), '1 项操作');
  assert.equal(await get('flow-reauthorize').getAttribute('data-selected'), 'false');
  await page.keyboard.press('Escape'); assert(await focused(more));
  report.checks.push('Reset: three optional switches, upload dependency, mobile flow, keyboard focus, stale revision, and independent logout');
};
