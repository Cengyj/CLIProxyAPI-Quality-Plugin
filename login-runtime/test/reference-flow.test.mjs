import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { chromium } from 'playwright-core';
import { runLogin, browserExecutable } from '../core.mjs';
import { generateTotp } from '../totp.mjs';

const email = 'reference.fixture@example.test';
const password = 'Reference-Fixture-Password9!';
const secret = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ';
const state = 'host-owned-reference-state';
const browserCheckPath = '/cdn-cgi/challenge-platform/h/g/flow/offline-check';
const sdk = `<script>globalThis.SentinelSDK={token:async flow=>JSON.stringify({p:'fixture',t:'fixture',c:'fixture',id:document.cookie.split('; ').find(cookie=>cookie.startsWith('oai-did='))?.slice(8)||'',flow,token:'fixture'}),sessionObserverToken:async()=>''};</script>`;
const fixedPhases = new Set(['browser_start','browser_verification','session','email','password','totp','oauth','workspace','callback','authorization','consent']);

async function referenceFixture(t, scenario = 'success', { proxy = false, workspaceID = '', responseCase } = {}) {
  const calls = [], forwarded = [], progress = [];
  let authOrigin, chatOrigin, redirect, webDone = false;
  let signinReached, releaseSignin;
  const pendingSignin = new Promise(resolve => { signinReached = resolve; });
  const signinGate = new Promise(resolve => { releaseSignin = resolve; });
  const fixtureSDK = scenario === 'sdk_error' ? sdk.replace("token:'fixture'", "token:'fixture',e:'SDK_REJECTED'") : scenario === 'sdk_missing_fields' ? `<script>globalThis.SentinelSDK={token:async flow=>JSON.stringify({token:'incomplete',flow})};</script>` : sdk;
  const json = (response, value, status = 200) => { response.writeHead(status, { 'content-type': 'application/json' }); response.end(JSON.stringify(value)); };
  const html = (response, value, headers = {}) => { response.writeHead(200, { 'content-type': 'text/html', ...headers }); response.end('<!doctype html><html><head><title>Offline account</title></head><body>' + value + fixtureSDK + '</body></html>'); };
  const jump = (response, location, cookie) => { response.writeHead(302, { location, ...(cookie ? { 'set-cookie': cookie } : {}) }); response.end(); };
  const staleWorkspaces = () => scenario === 'callback_with_workspaces' ? { 'oai-client-auth-session': { workspaces: [{ id: 'workspace-a', kind: 'personal' }] } } : {};
  const mfa = () => ({ page: { type: 'mfa_challenge' }, ...(scenario === 'mfa_without_continuation' ? {} : { continue_url: authOrigin + '/mfa-challenge/fixture-factor' }), 'oai-client-auth-session': { mfa_challenge_factors: scenario === 'missing_factor' ? [{ factor_type: 'sms', id: 'sms-factor' }] : [{ factor_type: 'sms', id: 'sms-factor' }, { factor_type: 'totp', id: 'fixture-factor' }] } });
  const read = async (request, source) => {
    let body = ''; for await (const chunk of request) body += chunk;
    const url = new URL(request.url, source === 'auth' ? authOrigin : chatOrigin);
    const call = { source, method: request.method, path: url.pathname, query: Object.fromEntries(url.searchParams), body, cookie: request.headers.cookie || '', headers: request.headers };
    calls.push(call); return call;
  };
  const auth = http.createServer(async (request, response) => {
    const call = await read(request, 'auth');
    switch (call.path) {
      case '/existing-session': return jump(response, chatOrigin + '/api/auth/callback/openai?code=offline-web-code', 'fixture_auth=session-bound; Path=/');
      case '/log-in/password': return html(response, '<h1>Enter your password</h1>', { 'set-cookie': 'fixture_auth=session-bound; Path=/' });
      case '/api/accounts/password/verify':
        if (scenario === 'password_rejected') return json(response, { error: { code: 'invalid_password' } }, 401);
        if (scenario === 'password_rejected_403') return json(response, { error: { code: 'incorrect_password' } }, 403);
        if (scenario === 'password_challenge_403') { response.writeHead(403, { 'content-type': 'text/html', 'cf-mitigated': 'challenge' }); response.end('<html><title>Just a moment...</title><body>Verify you are human</body></html>'); return; }
        if (scenario === 'unknown_password_success') return json(response, {});
        if (scenario === 'untrusted_continuation') return json(response, { page: { type: 'external_url' }, continue_url: 'http://untrusted.fixture.test/collect' });
        if (['email_verification','phone_verification','profile_required'].includes(scenario)) {
          const [type, path] = scenario === 'email_verification' ? ['email_verification','/email-verification'] : scenario === 'phone_verification' ? ['add_phone','/add-phone'] : ['about_you','/about-you'];
          return json(response, { page: { type }, continue_url: authOrigin + path });
        }
        return json(response, mfa());
      case '/mfa-challenge/fixture-factor': return html(response, '<h1>Authenticator app</h1>');
      case '/api/accounts/mfa/issue_challenge': return json(response, {});
      case '/api/accounts/mfa/verify':
        if (scenario === 'totp_rejected') return json(response, { error: { code: 'invalid_totp' } }, 400);
        if (scenario === 'totp_rejected_422') return json(response, { error: { code: 'invalid_totp' } }, 422);
        if (scenario === 'totp_permission_403') return json(response, { error: { code: 'permission_denied' } }, 403);
        return json(response, { page: { type: 'external_url' }, continue_url: chatOrigin + '/api/auth/callback/openai?code=offline-web-code', ...staleWorkspaces() });
      case '/oauth/authorize': return jump(response, authOrigin + (scenario === 'slow_consent' ? '/sign-in-with-chatgpt/codex/consent' : '/choose-an-account'));
      case '/choose-an-account': return html(response, scenario === 'hidden_session' ? '<input type="hidden" name="session_id" value="us_fixture_1234567890">' : `<button data-session-id="us_fixture_1234567890" data-email="${email}">${email}<input type="hidden" name="session_id" value="us_fixture_1234567890"></button>`);
      case '/api/accounts/session/select': return json(response, { page: { type: 'workspace' }, continue_url: authOrigin + '/sign-in-with-chatgpt/codex/consent', 'oai-client-auth-session': { workspaces: scenario === 'multiple_workspaces' || workspaceID ? [{ id: 'workspace-a', kind: 'personal' }, { id: 'workspace-b', kind: 'organization' }] : [{ id: 'workspace-a', kind: 'personal' }] } });
      case '/sign-in-with-chatgpt/codex/consent': return html(response, scenario === 'slow_consent' ? `<h1>Continue to Codex</h1><button id="consent">Allow</button><script>document.getElementById('consent').addEventListener('click',async()=>{await fetch('/api/accounts/consent',{method:'POST'});setTimeout(()=>{location.href=${JSON.stringify(redirect + '?code=offline-oauth-code&state=' + state)}},800);});</script>` : '<h1>Continue to Codex</h1>');
      case '/api/accounts/consent': return json(response, {});
      case '/api/accounts/workspace/select': return json(response, { page: { type: 'external_url' }, continue_url: redirect + '?code=offline-oauth-code&state=' + (scenario === 'callback_state_mismatch' ? 'different-state' : state), ...staleWorkspaces() });
      case '/email-verification': return html(response, '<h1>Enter your email code</h1>');
      case '/about-you': return html(response, '<h1>Complete your profile</h1>');
      default: return json(response, { error: { code: 'fixture_unexpected_route' } }, 404);
    }
  });
  const chat = http.createServer(async (request, response) => {
    const call = await read(request, 'chat');
    if (call.path === '/api/auth/csrf' && responseCase) {
      response.writeHead(responseCase.status, responseCase.headers);
      response.end(responseCase.body);
      return;
    }
    switch (call.path) {
      case '/': {
        if (scenario.startsWith('chat_check_') && !webDone && !call.cookie.includes('fixture_browser=passed')) {
          const done = `location.assign('/');`;
          const fetchCheck = `fetch('${browserCheckPath}',{method:'POST',body:'offline-check'})`;
          let content = 'Checking your browser';
          let script = '';
          if (scenario === 'chat_check_fetch') script = `${fetchCheck}.then(()=>{${done}}).catch(()=>{});`;
          if (scenario === 'chat_check_xhr') script = `const xhr=new XMLHttpRequest();xhr.open('POST','${browserCheckPath}');xhr.onload=()=>{${done}};xhr.send('offline-check');`;
          if (scenario === 'chat_check_blank_navigation') script = `setTimeout(()=>location.assign('/browser-check-transition'),300);`;
          if (scenario === 'chat_check_stuck') script = `${fetchCheck}.catch(()=>{});`;
          if (scenario === 'chat_check_human') content = '<label><input type="checkbox">Verify you are human</label>';
          if (scenario === 'chat_check_put' || scenario === 'chat_check_unknown_write') {
            const path = scenario === 'chat_check_put' ? browserCheckPath : '/api/unrelated-write';
            const method = scenario === 'chat_check_put' ? 'PUT' : 'POST';
            script = `fetch('${path}',{method:'${method}',body:'offline-check'}).catch(()=>{}).finally(()=>{document.title='ChatGPT';document.body.innerHTML='<h1>ChatGPT offline</h1>';});`;
          }
          if (scenario === 'chat_check_navigation_post') {
            content += `<form method="post" action="${browserCheckPath}"><input name="check" value="offline"></form>`;
            script = 'document.forms[0].submit();';
          }
          response.writeHead(403, { 'content-type': 'text/html', 'cf-mitigated': 'challenge', 'set-cookie': 'fixture_csrf=csrf-session; Path=/' });
          response.end('<!doctype html><title>Just a moment...</title><body>' + content + '<script>' + script + '</script>');
          return;
        }
        return html(response, '<h1>ChatGPT offline</h1>', { 'set-cookie': scenario === 'server_device' ? ['fixture_csrf=csrf-session; Path=/', 'oai-did=fixture-existing-device; Path=/'] : 'fixture_csrf=csrf-session; Path=/' });
      }
      case '/browser-check-transition':
        response.writeHead(200, { 'content-type': 'text/html' });
        response.end(`<!doctype html><title></title><body><script>setTimeout(()=>{document.title='Just a moment...';document.body.innerHTML='Checking your browser';setTimeout(()=>fetch('${browserCheckPath}',{method:'POST',body:'offline-check'}).then(()=>location.assign('/')).catch(()=>{}),250);},700);</script>`);
        return;
      case browserCheckPath:
        await new Promise(resolve => setTimeout(resolve,350));
        if (response.destroyed) return;
        response.setHeader('set-cookie', 'fixture_browser=passed; Path=/');
        return json(response, { complete: true });
      case '/api/unrelated-write': return json(response, {});
      case '/api/auth/providers': return json(response, { openai: { id: 'openai' } });
      case '/api/auth/csrf':
        if (scenario === 'csrf_html_503') { response.writeHead(503, { 'content-type': 'text/html' }); response.end('<h1>Temporarily unavailable</h1>'); return; }
        return json(response, { csrfToken: 'fixture-csrf' });
      case '/api/auth/signin/openai':
        if (scenario === 'pending_signin') { signinReached(); await signinGate; if (response.destroyed) return; }
        return json(response, { url: authOrigin + (scenario.startsWith('existing_session') ? '/existing-session' : '/log-in/password') });
      case '/api/auth/callback/openai': webDone = true; return jump(response, chatOrigin + '/', 'fixture_web=verified-web-session; Path=/');
      case '/api/auth/session': return json(response, webDone ? { user: { email: ['session_mismatch','existing_session_mismatch'].includes(scenario) ? 'different@example.test' : email }, expires: '2099-01-01T00:00:00Z' } : {});
      default: return json(response, { error: { code: 'fixture_unexpected_route' } }, 404);
    }
  });
  const callback = http.createServer((request, response) => { calls.push({ source: 'callback', method: request.method, path: new URL(request.url, redirect).pathname }); response.writeHead(500); response.end('callback must be intercepted'); });
  for (const server of [auth, chat, callback]) { server.listen(0, '127.0.0.1'); await once(server, 'listening'); t.after(() => new Promise(resolve => { server.closeAllConnections(); server.close(resolve); })); }
  redirect = `http://127.0.0.1:${callback.address().port}/auth/callback`;
  authOrigin = proxy ? 'http://auth.fixture.test' : `http://127.0.0.1:${auth.address().port}`;
  chatOrigin = proxy ? 'http://chat.fixture.test' : `http://127.0.0.1:${chat.address().port}`;
  let proxyURL;
  if (proxy) {
    const expected = 'Basic ' + Buffer.from('reference-proxy:fixture-only').toString('base64');
    const server = http.createServer((request, response) => {
      if (request.headers['proxy-authorization'] !== expected) { response.writeHead(407, { 'proxy-authenticate': 'Basic realm="reference-fixture"' }); response.end(); return; }
      let url; try { url = new URL(request.url); } catch { response.writeHead(400); response.end(); return; }
      const target = url.hostname === 'auth.fixture.test' ? auth : url.hostname === 'chat.fixture.test' ? chat : undefined;
      if (!target) { forwarded.push({ host: url.hostname, path: url.pathname, rejected: true }); response.writeHead(502); response.end(); return; }
      forwarded.push({ host: url.hostname, path: url.pathname, method: request.method });
      const headers = { ...request.headers }; delete headers['proxy-authorization'];
      const upstream = http.request({ hostname: '127.0.0.1', port: target.address().port, path: url.pathname + url.search, method: request.method, headers }, reply => { response.writeHead(reply.statusCode, reply.headers); reply.pipe(response); });
      upstream.on('error', () => { if (!response.headersSent) response.writeHead(502); response.end(); }); request.pipe(upstream);
    });
    server.on('connect', (_request, socket) => socket.end('HTTP/1.1 502 Bad Gateway\r\n\r\n'));
    server.listen(0, '127.0.0.1'); await once(server, 'listening');
    t.after(() => new Promise(resolve => { server.closeAllConnections(); server.close(resolve); }));
    proxyURL = `http://reference-proxy:fixture-only@127.0.0.1:${server.address().port}`;
  }
  const query = new URLSearchParams({ client_id: 'app_EMoamEEZ73f0CkXaXp7hrann', redirect_uri: redirect, state, response_type: 'code', code_challenge: 'a'.repeat(43), code_challenge_method: 'S256', scope: 'openid profile email offline_access', codex_cli_simplified_flow: 'true', id_token_add_organizations: 'true' });
  const input = { version: 1, job_id: 'reference-offline-job', attempt: 1, authorization_url: authOrigin + '/oauth/authorize?' + query, redirect_uri: redirect, state, credentials: { email, password, totp_secret: secret }, ...(proxyURL ? { proxy_url: proxyURL } : {}), ...(workspaceID ? { workspace_id: workspaceID } : {}) };
  const options = { chromium, executablePath: browserExecutable(), policy: { authOrigin, chatOrigin }, now: () => 59000, onProgress: phase => progress.push(phase) };
  return { input, options, calls, forwarded, progress, authOrigin, chatOrigin, redirect, pendingSignin, releaseSignin };
}

function requests(fixture, path) { return fixture.calls.filter(call => call.path === path); }
function body(call) { return JSON.parse(call.body); }
function noUnexpectedWrites(fixture) {
  assert(!fixture.calls.some(call => /oauth\/token|auth\/callback$|create_account|password\/(?:reset|add)|mfa\/(?:enroll|activate|disable)/.test(call.path)));
  assert(!fixture.forwarded.some(call => call.rejected));
}

for (const [name, status, headers, responseBody, expected] of [
  ['JSON denial mentioning challenge resources', 403, { 'content-type': 'application/json' }, '{"error":{"code":"account_disabled","message":"See /cdn-cgi/challenge-platform/help"}}', { code: 'interaction_required', stage: 'session', reason: 'upstream_rejected' }],
  ['JSON denial mislabeled HTML', 403, { 'content-type': 'text/html' }, '{"error":{"code":"permission_denied","message":"cf-chl-error"}}', { code: 'interaction_required', stage: 'session', reason: 'upstream_rejected' }],
  ['JSON rate limit mentioning a challenge', 429, { 'content-type': 'application/problem+json' }, '{"error":"cf-challenge"}', { code: 'rate_limited' }],
  ['HTML rate limit with generic marker', 429, { 'content-type': 'text/html' }, '<html><title>Too many requests</title>cf-challenge</html>', { code: 'rate_limited' }],
  ['ordinary service failure with Cloudflare styles', 503, { 'content-type': 'text/html' }, '<html><link href="/cdn-cgi/styles/error.css">Unavailable</html>', { code: 'service_error' }],
  ['explicit case-insensitive mitigation header', 403, { 'content-type': 'application/json', 'cf-mitigated': ' Challenge ' }, '{"error":"blocked"}', { code: 'interaction_required', stage: 'browser_verification', reason: 'browser_verification_required' }],
  ['alternate explicit mitigation header', 429, { 'content-type': 'application/json', 'x-cf-mitigated': 'challenge' }, '{"error":"blocked"}', { code: 'interaction_required', stage: 'browser_verification', reason: 'browser_verification_required' }],
  ['HTML verification conflict', 409, { 'content-type': 'text/html' }, '<html><title>Just a moment...</title></html>', { code: 'interaction_required', stage: 'browser_verification', reason: 'browser_verification_required' }],
  ['successful JSON containing a resource marker', 200, { 'content-type': 'application/json' }, '{"csrfToken":"fixture-csrf","help":"/cdn-cgi/challenge-platform/help"}', null],
]) {
  test(`reference response preserves ${name}`, async t => {
    const fixture = await referenceFixture(t, 'success', { responseCase: { status, headers, body: responseBody } });
    if (expected) {
      await assert.rejects(runLogin(fixture.input, fixture.options), expected);
      assert.equal(requests(fixture, '/api/auth/signin/openai').length, 0);
      assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
    } else {
      const result = await runLogin(fixture.input, fixture.options);
      assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
      assert.equal(requests(fixture, '/api/accounts/password/verify').length, 1);
      assert.equal(requests(fixture, '/api/accounts/mfa/verify').length, 1);
    }
    assert.equal(requests(fixture, '/api/auth/csrf').length, 1);
    noUnexpectedWrites(fixture);
  });
}

for (const proxy of [false, true]) {
  test(`reference web login retains one session through password, TOTP and host OAuth${proxy ? ' using one authenticated proxy' : ''}`, async t => {
    const fixture = await referenceFixture(t, 'success', { proxy });
    const result = await runLogin(fixture.input, fixture.options);
    assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
    const signin = requests(fixture, '/api/auth/signin/openai')[0];
    assert.deepEqual(Object.fromEntries(new URLSearchParams(signin.body)), { callbackUrl: fixture.chatOrigin + '/', csrfToken: 'fixture-csrf', json: 'true' });
    assert.equal(signin.query.prompt, 'login'); assert.equal(signin.query.login_hint, email); assert.equal(signin.query.screen_hint, 'login_or_signup');
    assert(signin.query['ext-oai-did']); assert(signin.query.auth_session_logging_id); assert(signin.cookie.includes('fixture_csrf=csrf-session'));
    assert.deepEqual(body(requests(fixture, '/api/accounts/password/verify')[0]), { password });
    assert.deepEqual(body(requests(fixture, '/api/accounts/mfa/issue_challenge')[0]), { type: 'totp', id: 'fixture-factor', force_fresh_challenge: false });
    assert.deepEqual(body(requests(fixture, '/api/accounts/mfa/verify')[0]), { type: 'totp', id: 'fixture-factor', code: generateTotp(secret, 59000) });
    for (const path of ['/api/accounts/password/verify','/api/accounts/mfa/verify']) {
      const matches = requests(fixture, path); assert.equal(matches.length, 1);
      assert.equal(JSON.parse(matches[0].headers['openai-sentinel-token']).flow, 'password_verify');
      assert.equal(JSON.parse(matches[0].headers['openai-sentinel-token']).id, signin.query['ext-oai-did']);
      assert.equal(matches[0].cookie.split('; ').find(cookie => cookie.startsWith('oai-did='))?.slice(8), signin.query['ext-oai-did']);
      assert(matches[0].cookie.includes('fixture_auth=session-bound'));
    }
    const session = requests(fixture, '/api/auth/session')[0]; assert(session.cookie.includes('fixture_web=verified-web-session'));
    const authorization = requests(fixture, '/oauth/authorize'); assert.equal(authorization.length, 1);
    assert.deepEqual(authorization[0].query, Object.fromEntries(new URL(fixture.input.authorization_url).searchParams));
    assert(fixture.calls.indexOf(session) < fixture.calls.indexOf(authorization[0]));
    assert.deepEqual(body(requests(fixture, '/api/accounts/session/select')[0]), { session_id: 'us_fixture_1234567890' });
    assert.deepEqual(body(requests(fixture, '/api/accounts/workspace/select')[0]), { workspace_id: 'workspace-a' });
    for (const phase of ['session','password','totp','oauth','callback']) assert(fixture.progress.includes(phase), `missing fixed progress ${phase}`);
    assert(fixture.progress.every(phase => typeof phase === 'string' && fixedPhases.has(phase)));
    const serialized = JSON.stringify(fixture.progress); for (const value of [email,password,secret,'offline-web-code','offline-oauth-code',state]) assert(!serialized.includes(value));
    if (proxy) for (const call of fixture.calls.filter(call => call.path !== '/favicon.ico')) assert(fixture.forwarded.some(value => value.path === call.path && value.method === call.method));
    noUnexpectedWrites(fixture);
  });
}

test('reference sign-in preserves the server device across distinct ChatGPT and authentication hosts', async t => {
  const fixture = await referenceFixture(t, 'server_device', { proxy: true });
  const result = await runLogin(fixture.input, fixture.options);
  assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
  const signin = requests(fixture, '/api/auth/signin/openai')[0];
  assert.equal(signin.query['ext-oai-did'], 'fixture-existing-device');
  for (const path of ['/api/accounts/password/verify', '/api/accounts/mfa/verify']) {
    const call = requests(fixture, path)[0];
    assert.equal(call.cookie.split('; ').find(cookie => cookie.startsWith('oai-did='))?.slice(8), 'fixture-existing-device');
    assert.equal(JSON.parse(call.headers['openai-sentinel-token']).id, 'fixture-existing-device');
  }
  noUnexpectedWrites(fixture);
});

for (const scenario of ['existing_session', 'existing_session_mismatch']) {
  test(`reference sign-in validates ${scenario} before host OAuth without resubmitting credentials`, async t => {
    const fixture = await referenceFixture(t, scenario, { proxy: true });
    if (scenario === 'existing_session') {
      const result = await runLogin(fixture.input, fixture.options);
      assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
      const session = requests(fixture, '/api/auth/session')[0];
      const oauth = requests(fixture, '/oauth/authorize')[0];
      assert(session.cookie.includes('fixture_web=verified-web-session'));
      assert(fixture.calls.indexOf(session) < fixture.calls.indexOf(oauth));
    } else {
      await assert.rejects(runLogin(fixture.input, fixture.options), { code: 'interaction_required', stage: 'session', reason: 'session_unconfirmed' });
      assert.equal(requests(fixture, '/oauth/authorize').length, 0);
    }
    assert.equal(requests(fixture, '/api/auth/session').length, 1);
    for (const path of ['/api/accounts/authorize/continue', '/api/accounts/password/verify', '/api/accounts/mfa/issue_challenge', '/api/accounts/mfa/verify']) assert.equal(requests(fixture, path).length, 0);
    assert(!fixture.progress.includes('password'));
    assert(!fixture.progress.includes('totp'));
    noUnexpectedWrites(fixture);
  });
}

for (const scenario of ['chat_check_fetch', 'chat_check_xhr', 'chat_check_blank_navigation']) {
  test(`reference ChatGPT session completes ${scenario} before password and TOTP`, async t => {
    const fixture = await referenceFixture(t, scenario);
    const result = await runLogin(fixture.input, { ...fixture.options, policy: { ...fixture.options.policy, browserVerificationTimeoutMs: 5000 } });
    assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
    const checks = requests(fixture, browserCheckPath);
    assert.equal(checks.length, 1);
    assert.equal(checks[0].source, 'chat');
    assert.equal(checks[0].method, 'POST');
    const providers = requests(fixture, '/api/auth/providers');
    assert.equal(providers.length, 1);
    assert(fixture.calls.indexOf(checks[0]) < fixture.calls.indexOf(providers[0]));
    assert(providers[0].cookie.includes('fixture_browser=passed'));
    for (const path of ['/api/auth/signin/openai', '/api/accounts/password/verify', '/api/accounts/mfa/issue_challenge', '/api/accounts/mfa/verify']) assert.equal(requests(fixture, path).length, 1);
    assert.equal(fixture.progress.filter(phase => phase === 'browser_verification').length, 1);
    assert.deepEqual(fixture.progress.slice(0, fixture.progress.indexOf('password')), ['browser_start', 'session', 'browser_verification', 'session']);
    noUnexpectedWrites(fixture);
  });
}

for (const scenario of ['chat_check_put', 'chat_check_unknown_write']) {
  test(`reference ChatGPT browser-check permission blocks ${scenario}`, async t => {
    const fixture = await referenceFixture(t, scenario);
    const result = await runLogin(fixture.input, { ...fixture.options, policy: { ...fixture.options.policy, browserVerificationTimeoutMs: 3000 } });
    assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
    assert.equal(requests(fixture, browserCheckPath).length, 0);
    assert.equal(requests(fixture, '/api/unrelated-write').length, 0);
    noUnexpectedWrites(fixture);
  });
}

test('reference ChatGPT browser-check permission never sends a navigation POST or subsequent credentials', async t => {
  const fixture = await referenceFixture(t, 'chat_check_navigation_post');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 3000);
  try {
    await assert.rejects(runLogin(fixture.input, { ...fixture.options, signal: controller.signal, policy: { ...fixture.options.policy, browserVerificationTimeoutMs: 1000 } }), error => ['cancelled', 'interaction_required', 'timeout', 'transport_error'].includes(error.code));
    assert.equal(requests(fixture, browserCheckPath).length, 0);
    assert.equal(requests(fixture, '/api/auth/signin/openai').length, 0);
    assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
    noUnexpectedWrites(fixture);
  } finally { clearTimeout(timer); controller.abort(); }
});

test('reference automatic browser-check timeout is not reported as a human challenge', async t => {
  const fixture = await referenceFixture(t, 'chat_check_stuck');
  await assert.rejects(runLogin(fixture.input, { ...fixture.options, policy: { ...fixture.options.policy, browserVerificationTimeoutMs: 700 } }), { code: 'timeout', stage: 'browser_verification', reason: 'browser_verification_timeout' });
  assert.equal(requests(fixture, browserCheckPath).length, 1);
  assert.equal(requests(fixture, '/api/auth/providers').length, 0);
  assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
  assert.deepEqual(fixture.progress, ['browser_start', 'session', 'browser_verification']);
  noUnexpectedWrites(fixture);
});

test('reference explicit human checkbox is reported without waiting for the automatic-check timeout', async t => {
  const fixture = await referenceFixture(t, 'chat_check_human');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 4000);
  try {
    await assert.rejects(runLogin(fixture.input, { ...fixture.options, signal: controller.signal, policy: { ...fixture.options.policy, browserVerificationTimeoutMs: 10000 } }), { code: 'interaction_required', stage: 'browser_verification', reason: 'human_verification' });
    assert.equal(requests(fixture, browserCheckPath).length, 0);
    assert.equal(requests(fixture, '/api/auth/providers').length, 0);
    assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
    noUnexpectedWrites(fixture);
  } finally { clearTimeout(timer); controller.abort(); }
});

for (const [scenario, expected, forbidden] of [
  ['unknown_password_success', ['interaction_required'], '/api/accounts/mfa/issue_challenge'],
  ['password_rejected', ['password_rejected'], '/api/accounts/mfa/issue_challenge'],
  ['missing_factor', ['interaction_required'], '/api/accounts/mfa/issue_challenge'],
  ['totp_rejected', ['totp_rejected'], '/oauth/authorize'],
  ['session_mismatch', ['workspace_mismatch','interaction_required','authorization_denied'], '/oauth/authorize'],
  ['multiple_workspaces', ['interaction_required','workspace_mismatch'], '/api/accounts/workspace/select'],
  ['callback_state_mismatch', ['callback_state_mismatch'], '/oauth/token'],
  ['untrusted_continuation', ['interaction_required','callback_invalid'], '/api/accounts/mfa/issue_challenge'],
]) {
  test(`reference protocol stops at ${scenario} without replay or advancing`, async t => {
    const fixture = await referenceFixture(t, scenario);
    await assert.rejects(runLogin(fixture.input, fixture.options), error => { assert(expected.includes(error.code), `unexpected fixed code ${error.code}`); return true; });
    assert.equal(requests(fixture, forbidden).length, 0);
    assert(requests(fixture, '/api/accounts/password/verify').length <= 1);
    assert(requests(fixture, '/api/accounts/mfa/verify').length <= 1);
    noUnexpectedWrites(fixture);
  });
}

test('reference flow stops at email, phone and profile requirements without sending codes or creating accounts', async t => {
  for (const scenario of ['email_verification','phone_verification','profile_required']) {
    await t.test(scenario, async child => {
      const fixture = await referenceFixture(child, scenario);
      await assert.rejects(runLogin(fixture.input, fixture.options), { code: 'interaction_required' });
      assert.equal(requests(fixture, '/api/accounts/password/verify').length, 1);
      assert.equal(requests(fixture, '/api/accounts/mfa/issue_challenge').length, 0);
      assert.equal(requests(fixture, '/oauth/authorize').length, 0);
      noUnexpectedWrites(fixture);
    });
  }
});

test('reference reauthorization selects only the host-bound workspace from multiple choices', async t => {
  const fixture = await referenceFixture(t, 'success', { workspaceID: 'workspace-b' });
  const result = await runLogin(fixture.input, fixture.options);
  assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
  assert.deepEqual(body(requests(fixture, '/api/accounts/workspace/select')[0]), { workspace_id: 'workspace-b' });
  noUnexpectedWrites(fixture);
});

for (const scenario of ['callback_with_workspaces', 'mfa_without_continuation', 'hidden_session']) {
  test(`reference flow accepts ${scenario} without changing or replaying the protocol step`, async t => {
    const fixture = await referenceFixture(t, scenario);
    const result = await runLogin(fixture.input, fixture.options);
    assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
    assert.equal(requests(fixture, '/api/accounts/workspace/select').length, 1);
    assert.equal(requests(fixture, '/api/accounts/session/select').length, 1);
    assert.equal(requests(fixture, '/api/accounts/mfa/issue_challenge').length, 1);
    assert.equal(requests(fixture, '/mfa-challenge/fixture-factor').length, 1);
    assert.deepEqual(body(requests(fixture, '/api/accounts/mfa/issue_challenge')[0]), { type: 'totp', id: 'fixture-factor', force_fresh_challenge: false });
    noUnexpectedWrites(fixture);
  });
}

for (const scenario of ['sdk_error', 'sdk_missing_fields']) {
  test(`reference flow rejects ${scenario} before submitting a password`, async t => {
    const fixture = await referenceFixture(t, scenario);
    await assert.rejects(runLogin(fixture.input, fixture.options), { code: 'interaction_required', reason: 'sentinel_unavailable' });
    assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
    assert.equal(requests(fixture, '/api/accounts/mfa/issue_challenge').length, 0);
    noUnexpectedWrites(fixture);
  });
}

test('reference flow classifies an HTML 503 from the session endpoint as a service failure', async t => {
  const fixture = await referenceFixture(t, 'csrf_html_503');
  await assert.rejects(runLogin(fixture.input, fixture.options), { code: 'service_error' });
  assert.equal(requests(fixture, '/api/auth/signin/openai').length, 0);
  assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
  noUnexpectedWrites(fixture);
});

test('reference flow clicks delayed OAuth consent only once and waits for its callback', async t => {
  const fixture = await referenceFixture(t, 'slow_consent');
  const result = await runLogin(fixture.input, fixture.options);
  assert.equal(result.callback_url, fixture.redirect + '?code=offline-oauth-code&state=' + state);
  assert.equal(requests(fixture, '/api/accounts/consent').length, 1);
  assert.equal(requests(fixture, '/api/accounts/session/select').length, 0);
  noUnexpectedWrites(fixture);
});

test('reference flow cancellation during pending signin sends no password or later authentication request', { timeout: 15000 }, async t => {
  const fixture = await referenceFixture(t, 'pending_signin');
  const controller = new AbortController();
  const result = runLogin(fixture.input, { ...fixture.options, signal: controller.signal });
  try {
    await fixture.pendingSignin;
    controller.abort();
    await assert.rejects(result, { code: 'cancelled' });
    assert.equal(requests(fixture, '/api/auth/signin/openai').length, 1);
    assert.equal(requests(fixture, '/log-in/password').length, 0);
    assert.equal(requests(fixture, '/api/accounts/password/verify').length, 0);
    noUnexpectedWrites(fixture);
  } finally { controller.abort(); fixture.releaseSignin(); }
});

for (const [scenario, expected] of [
  ['password_challenge_403', { code: 'interaction_required', stage: 'browser_verification', reason: 'browser_verification_required' }],
  ['totp_permission_403', { code: 'interaction_required', stage: 'totp', reason: 'upstream_rejected' }],
  ['password_rejected_403', { code: 'password_rejected', stage: 'password', reason: 'upstream_rejected' }],
  ['totp_rejected_422', { code: 'totp_rejected', stage: 'totp', reason: 'upstream_rejected' }],
]) {
  test(`reference flow classifies ${scenario} by the response rather than status alone`, async t => {
    const fixture = await referenceFixture(t, scenario);
    await assert.rejects(runLogin(fixture.input, fixture.options), expected);
    assert.equal(requests(fixture, '/api/accounts/password/verify').length, 1);
    assert.equal(requests(fixture, '/api/accounts/mfa/verify').length, scenario.startsWith('totp_') ? 1 : 0);
    assert.equal(requests(fixture, '/oauth/authorize').length, 0);
    noUnexpectedWrites(fixture);
  });
}
