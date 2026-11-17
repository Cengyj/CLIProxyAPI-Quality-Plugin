import { randomUUID } from 'node:crypto';
import { generateTotp } from './totp.mjs';

const pageType = value => value?.page?.type || '';
const continuation = value => value?.continue_url || value?.page?.payload?.url || '';
const object = value => value && typeof value === 'object' && !Array.isArray(value);

// Follow the existing-account protocol while the official browser owns JS, cookies and transport.
export async function runReferenceFlow({ page, context, input, authOrigin, chatOrigin, navigate, checkpoint, progress, fail, now, getCallback, beginOAuth }) {
  let passwordUsed = false, totpUsed = false;
  const trustedURL = (raw, allowCallback = false) => {
    let url;
    try { url = new URL(raw, authOrigin); } catch { fail('interaction_required', '', 'unsupported_navigation'); }
    const target = new URL(input.redirect_uri);
    if (allowCallback && url.origin === target.origin && url.pathname === target.pathname) return url.href;
    if (url.username || url.password || url.hash || ![authOrigin, chatOrigin].includes(url.origin)) fail('interaction_required', '', 'unsupported_navigation');
    if (/\/(?:create[._-]account|about-you|reset-password|add-phone|phone-otp)(?:\/|$)/.test(url.pathname)) fail('interaction_required', '', 'unsupported_page');
    return url.href;
  };
  const readJSON = async (path, { method = 'GET', body, form, sentinel = '' } = {}) => {
    await checkpoint();
    const url = new URL(trustedURL(path));
    if (new URL(page.url()).origin !== url.origin) fail('interaction_required', '', 'unsupported_navigation');
    if (sentinel) {
      const ready = await page.waitForFunction(() => typeof globalThis.SentinelSDK?.token === 'function', null, { timeout: 10000 }).then(() => true, () => false);
      if (!ready) fail('interaction_required', '', 'sentinel_unavailable');
    }
    const result = await page.evaluate(async ({ url, method, body, form, sentinel, invocationID }) => {
      const headers = { Accept: 'application/json' };
      if (method === 'POST') headers['X-Access-Flow-Invocation-Id'] = invocationID;
      if (sentinel) {
        try {
          const validToken = (raw, keys) => {
            if (typeof raw !== 'string' || !raw || raw.length > 32768) return false;
            const value = JSON.parse(raw);
            return value && typeof value === 'object' && !Array.isArray(value) && keys.every(key => Object.hasOwn(value,key)) && !value.e && value.flow === sentinel;
          };
          const token = await globalThis.SentinelSDK.token(sentinel);
          if (!validToken(token,['p','t','c','id','flow'])) return { failure: 'sentinel_unavailable' };
          headers['Openai-Sentinel-Token'] = token;
          if (typeof globalThis.SentinelSDK.sessionObserverToken === 'function') {
            const observer = await globalThis.SentinelSDK.sessionObserverToken(sentinel);
            if (observer !== '') {
              if (!validToken(observer,['so','c','id','flow'])) return { failure: 'sentinel_unavailable' };
              headers['Openai-Sentinel-So-Token'] = observer;
            }
          }
        } catch { return { failure: 'sentinel_unavailable' }; }
      }
      let payload;
      if (form) { headers['Content-Type'] = 'application/x-www-form-urlencoded'; payload = new URLSearchParams(form).toString(); }
      else if (body !== undefined) { headers['Content-Type'] = 'application/json'; payload = JSON.stringify(body); }
      const response = await fetch(url, { method, headers, body: payload, credentials: 'include', redirect: 'error' });
      if (Number(response.headers.get('content-length')) > 262144) return { failure: 'authentication_not_advanced' };
      const reader = response.body?.getReader();
      let size = 0; const chunks = [];
      if (reader) for (;;) {
        const { done, value } = await reader.read(); if (done) break;
        size += value.length;
        if (size > 262144) { await reader.cancel(); return { failure: 'authentication_not_advanced' }; }
        chunks.push(value);
      }
      const bytes = new Uint8Array(size); let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
      const text = new TextDecoder().decode(bytes);
      let data, parsed = false;
      try { data = JSON.parse(text); parsed = true; } catch {}
      const explicitChallenge = ['cf-mitigated', 'x-cf-mitigated'].some(name => response.headers.get(name)?.trim().toLowerCase() === 'challenge');
      const contentType = (response.headers.get('content-type') || '').toLowerCase();
      const html = contentType.includes('text/html') || /<!doctype html|<html[\s>]/i.test(text);
      const marker = /\/cdn-cgi\/challenge-platform\/|_cf_chl_opt|cf-challenge|cf-chl-/i.test(text) || html && /<title>\s*Just a moment/i.test(text);
      // A JSON rejection mentioning verification resources is still a business response.
      const challenge = explicitChallenge || !parsed && !contentType.includes('json') && marker && ([403,503].includes(response.status) || [400,409].includes(response.status) && html);
      if (challenge) return { failure: 'browser_verification_required' };
      if (!parsed) return { failure: 'authentication_not_advanced', status: response.status };
      return { status: response.status, data };
    }, { url: url.href, method, body, form, sentinel, invocationID: randomUUID() });
    if (result.failure === 'browser_verification_required') fail('interaction_required', 'browser_verification', result.failure);
    await checkpoint();
    if (result.status === 429) fail('rate_limited');
    if (result.status >= 500) fail('service_error');
    if (result.failure) fail('interaction_required', '', result.failure);
    if (result.status < 200 || result.status >= 300) {
      const code = result.data?.error?.code;
      const rejected = [400,401,403,422].includes(result.status);
      if (rejected && ['invalid_password','incorrect_password','wrong_password'].includes(code) && url.pathname === '/api/accounts/password/verify') fail('password_rejected', 'password', 'upstream_rejected');
      if (rejected && ['invalid_totp','invalid_otp','invalid_code','incorrect_code','wrong_code'].includes(code) && url.pathname === '/api/accounts/mfa/verify') fail('totp_rejected', 'totp', 'upstream_rejected');
      fail('interaction_required', '', 'upstream_rejected');
    }
    if (!object(result.data)) fail('interaction_required', '', 'authentication_not_advanced');
    return result.data;
  };
  const nextStep = value => {
    const type = pageType(value), next = continuation(value);
    if (type === 'email_verification' || /\/email-verification(?:\/|$)/.test(next)) fail('interaction_required', 'email', 'email_verification');
    if (type === 'add_phone' || /\/(?:add-phone|phone-verification)(?:\/|$)/.test(next)) fail('interaction_required', '', 'phone_verification');
    if (type === 'about_you' || /\/about-you(?:\/|$)/.test(next)) fail('interaction_required', '', 'unsupported_page');
    return type;
  };
  const advanced = (value, previous, path) => {
    if (!object(value) || value.error || ('success' in value && value.success !== true) || [previous, 'login_password'].includes(pageType(value))) fail('interaction_required', '', 'authentication_not_advanced');
    const next = continuation(value);
    if (!next) {
      if (['mfa_challenge','workspace'].includes(pageType(value))) return value;
      fail('interaction_required', '', 'authentication_not_advanced');
    }
    const url = new URL(trustedURL(next, true));
    if (url.pathname === path || previous === 'mfa_challenge' && url.pathname.startsWith('/mfa-challenge/')) fail('interaction_required', '', 'authentication_not_advanced');
    nextStep(value);
    return value;
  };
  const selectWorkspace = async value => {
    const workspaces = value?.['oai-client-auth-session']?.workspaces;
    if (!Array.isArray(workspaces) || !workspaces.length) fail('interaction_required', 'workspace', 'workspace_selection');
    const valid = workspaces.filter(w => typeof w?.id === 'string' && w.id);
    const selected = input.workspace_id ? valid.find(w => w.id === input.workspace_id) : valid.length === 1 ? valid[0] : null;
    if (!selected) fail('interaction_required', 'workspace', 'workspace_selection');
    progress('workspace');
    return readJSON(authOrigin + '/api/accounts/workspace/select', { method: 'POST', body: { workspace_id: selected.id } });
  };
  const continuePayload = async (value, oauth = false) => {
    for (let n = 0; n < 4; n++) {
      const type = nextStep(value);
      const next = continuation(value);
      const target = next ? new URL(trustedURL(next, oauth)) : null;
      const callbackTarget = new URL(input.redirect_uri);
      if (target && (target.origin === chatOrigin && target.pathname === '/api/auth/callback/openai' || oauth && target.origin === callbackTarget.origin && target.pathname === callbackTarget.pathname)) {
        await navigate(target.href); return;
      }
      if (type === 'workspace' || target && /workspace|consent/.test(target.pathname) && value?.['oai-client-auth-session']?.workspaces?.length) {
        value = await selectWorkspace(value); continue;
      }
      if (!next) fail('interaction_required', '', 'authentication_not_advanced');
      await navigate(trustedURL(next, oauth));
      return;
    }
    fail('interaction_required', 'workspace', 'workspace_selection');
  };

  progress('session');
  await navigate(chatOrigin + '/');
  await readJSON(chatOrigin + '/api/auth/providers');
  const csrf = await readJSON(chatOrigin + '/api/auth/csrf');
  if (typeof csrf.csrfToken !== 'string' || !csrf.csrfToken) fail('interaction_required', 'session', 'session_unconfirmed');
  let device = (await context.cookies(chatOrigin)).find(cookie => cookie.name === 'oai-did')?.value;
  if (!device) {
    device = randomUUID();
    await context.addCookies([{ name: 'oai-did', value: device, url: chatOrigin + '/', secure: chatOrigin.startsWith('https:') }]);
  }
  const query = new URLSearchParams({ prompt: 'login', 'ext-oai-did': device, auth_session_logging_id: randomUUID(), screen_hint: 'login_or_signup', login_hint: input.credentials.email });
  const signin = await readJSON(chatOrigin + '/api/auth/signin/openai?' + query, { method: 'POST', form: { callbackUrl: chatOrigin + '/', csrfToken: csrf.csrfToken, json: 'true' } });
  if (typeof signin.url !== 'string' || new URL(trustedURL(signin.url)).origin !== authOrigin) fail('interaction_required', 'session', 'session_unconfirmed');
  const authURL = new URL(authOrigin);
  // Keep the web sign-in and official authentication SDK on one operation device.
  // OpenAI shares this non-authentication identifier with its Sentinel subdomain.
  const sharedOpenAI = authURL.protocol === 'https:' && (authURL.hostname === 'openai.com' || authURL.hostname.endsWith('.openai.com'));
  await context.addCookies([{ name: 'oai-did', value: device, ...(sharedOpenAI ? { domain: '.openai.com', path: '/', secure: true } : { url: authOrigin + '/', secure: authURL.protocol === 'https:' }) }]);
  await navigate(signin.url);
  let path = new URL(page.url()).pathname;
  if (/email-verification/.test(path)) fail('interaction_required', 'email', 'email_verification');
  if (['/log-in', '/log-in-or-create-account'].includes(path)) {
    progress('email');
    const email = await readJSON(authOrigin + '/api/accounts/authorize/continue', { method: 'POST', body: { username: { kind: 'email', value: input.credentials.email } }, sentinel: 'authorize_continue' });
    nextStep(email);
    await navigate(trustedURL(continuation(email)));
    path = new URL(page.url()).pathname;
  }
  const completedWebPage = new URL(page.url()).origin === chatOrigin && path === '/';
  if (!completedWebPage) {
    if (path !== '/log-in/password') fail('interaction_required', 'session', 'unsupported_page');
    progress('password');
    passwordUsed = true;
    let current = advanced(await readJSON(authOrigin + '/api/accounts/password/verify', { method: 'POST', body: { password: input.credentials.password }, sentinel: 'password_verify' }), 'login_password', path);
    if (pageType(current) === 'mfa_challenge' || /\/mfa-challenge\//.test(continuation(current))) {
      progress('totp');
      const session = current['oai-client-auth-session'];
      const factors = [...(Array.isArray(session?.mfa_challenge_factors) ? session.mfa_challenge_factors : []), ...(Array.isArray(session?.mfa_factors) ? session.mfa_factors : [])];
      const factor = factors.find(factor => factor?.factor_type === 'totp' && typeof factor.id === 'string' && factor.id);
      if (!factor) fail('interaction_required', 'totp', 'totp_factor_missing');
      const next = trustedURL(continuation(current) || authOrigin + '/mfa-challenge/' + encodeURIComponent(factor.id));
      await navigate(next);
      await readJSON(authOrigin + '/api/accounts/mfa/issue_challenge', { method: 'POST', body: { type: 'totp', id: factor.id, force_fresh_challenge: false } });
      totpUsed = true;
      current = advanced(await readJSON(authOrigin + '/api/accounts/mfa/verify', { method: 'POST', body: { type: 'totp', id: factor.id, code: generateTotp(input.credentials.totp_secret, now()) }, sentinel: 'password_verify' }), 'mfa_challenge', new URL(next).pathname);
    }
    await continuePayload(current);
  }
  if (new URL(page.url()).origin !== chatOrigin || new URL(page.url()).pathname !== '/') fail('interaction_required', 'session', 'session_unconfirmed');
  progress('session');
  const session = await readJSON(chatOrigin + '/api/auth/session');
  if (typeof session.user?.email !== 'string' || session.user.email.toLowerCase() !== input.credentials.email.toLowerCase()) fail('interaction_required', 'session', 'session_unconfirmed');

  progress('oauth');
  beginOAuth();
  await navigate(input.authorization_url);
  const clickAndAdvance = async button => {
    const previous = page.url();
    await button.click();
    const deadline = Date.now() + 15000;
    while (!getCallback() && page.url() === previous) {
      await checkpoint();
      if (Date.now() >= deadline) fail('interaction_required', 'oauth', 'authentication_not_advanced');
      await page.waitForTimeout(100);
    }
    if (!getCallback()) {
      await page.waitForLoadState('domcontentloaded');
      await checkpoint();
    }
  };
  for (let step = 0; step < 8 && !getCallback(); step++) {
    await checkpoint();
    const url = new URL(page.url());
    if (url.origin !== authOrigin) fail('interaction_required', 'oauth', 'unsupported_navigation');
    if (/choose-an-account|choose.account/.test(url.pathname)) {
      // Select only an account button whose visible identity matches this authenticated account.
      const buttons = page.getByRole('button').filter({ hasText: input.credentials.email });
      let sessionID;
      if (await buttons.count() === 1) sessionID = await buttons.getAttribute('data-session-id');
      else {
        const sessions = await page.locator('input[name=session_id]').evaluateAll(nodes => [...new Set(nodes.map(node => node.value).filter(value => /^us_[A-Za-z0-9_-]{10,}$/.test(value)))]);
        if (sessions.length !== 1) fail('interaction_required', 'oauth', 'account_selection');
        sessionID = sessions[0];
      }
      if (sessionID) {
        const selected = await readJSON(authOrigin + '/api/accounts/session/select', { method: 'POST', body: { session_id: sessionID } });
        await continuePayload(selected, true);
      } else await clickAndAdvance(buttons);
    } else if (/consent/.test(url.pathname)) {
      progress('consent');
      if (!/codex/i.test(await page.locator('body').innerText())) fail('interaction_required', 'consent', 'unsupported_page');
      const button = page.getByRole('button', { name: /^(?:continue|authorize|allow|accept|继续|授权|允许)$/i });
      if (await button.count() !== 1) fail('interaction_required', 'consent', 'submit_unavailable');
      await clickAndAdvance(button);
    } else if (/workspace|organization/.test(url.pathname)) {
      progress('workspace');
      const options = page.locator('[data-workspace-id]');
      let selected;
      for (let i = 0; i < await options.count(); i++) {
        const option = options.nth(i);
        if (await option.isVisible() && (input.workspace_id ? await option.getAttribute('data-workspace-id') === input.workspace_id : await options.count() === 1)) selected = option;
      }
      if (!selected) fail('interaction_required', 'workspace', 'workspace_selection');
      await clickAndAdvance(selected);
    } else {
      nextStep({ continue_url: url.href });
      fail('interaction_required', 'oauth', 'session_unconfirmed');
    }
  }
  if (!getCallback()) fail('interaction_required', 'oauth', 'session_unconfirmed');
  progress('callback');
  return { passwordUsed, totpUsed };
}
