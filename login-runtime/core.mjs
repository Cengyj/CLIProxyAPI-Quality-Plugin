import { existsSync } from 'node:fs';
import { isAbsolute } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { generateTotp, normalizeTotpSecret } from './totp.mjs';
import { runReferenceFlow } from './reference-flow.mjs';

export const VERSION = '1.2.2';
export const ERROR_CODES = new Set(['invalid_input','invalid_totp_secret','unsupported_totp_parameters','runtime_unavailable','browser_unavailable','browser_failed','transport_error','timeout','cancelled','interaction_required','password_rejected','totp_rejected','authorization_denied','rate_limited','service_error','callback_invalid','callback_state_mismatch','workspace_mismatch','proxy_unsupported']);
export const ERROR_STAGES = new Set(['browser_start','session','authorization','email','password','totp','oauth','consent','workspace','callback','browser_verification']);
export const ERROR_REASONS = new Set(['human_verification','browser_verification_timeout','browser_verification_required','email_verification','phone_verification','unsupported_request','unsupported_navigation','submit_unavailable','totp_method_unconfirmed','workspace_selection','account_selection','unexpected_dialog','unexpected_download','unsupported_page','upstream_rejected','proxy_connection_failed','session_unconfirmed','authentication_not_advanced','totp_factor_missing','sentinel_unavailable']);
export class LoginError extends Error {
  constructor(code, stage = '', reason = '') {
    super(ERROR_CODES.has(code) ? code : 'browser_failed');
    this.code = this.message;
    this.stage = ERROR_STAGES.has(stage) ? stage : '';
    this.reason = ERROR_REASONS.has(reason) ? reason : '';
  }
}
export const errorDiagnostics = error => ({
  ...(ERROR_STAGES.has(error?.stage) ? { error_stage: error.stage } : {}),
  ...(ERROR_REASONS.has(error?.reason) ? { error_reason: error.reason } : {}),
});
const fail = (code, stage, reason) => { throw new LoginError(code, stage, reason); };
const AUTH_ORIGIN = 'https://auth.openai.com';
const CLIENT_ID = 'app_EMoamEEZ73f0CkXaXp7hrann';
const POSTS = new Set(['/api/accounts/authorize/continue','/api/accounts/password/verify','/api/accounts/mfa/issue_challenge','/api/accounts/mfa/verify','/api/accounts/session/select','/api/accounts/workspace/select','/api/accounts/organization/select','/api/accounts/consent','/api/accounts/consent/accept']);
// Match complete path segments: /log-in-or-create-account is an existing-account login entry.
const FORBIDDEN_PATH = /\/(?:oauth\/token|password\/(?:add|reset)|reset-password|create[._-]account|mfa\/(?:enroll|activate|disable)|add-phone|phone-otp)(?:\/|$)/i;
const OFFICIAL_HOSTS = new Set(['auth.openai.com','chatgpt.com','www.chatgpt.com','cdn.openai.com','cdn.auth0.com','auth0.openai.com','sentinel.openai.com','oaistatic.com','www.oaistatic.com']);
const REJECTION_TEXT = {
  password: /incorrect password|wrong password|invalid password|password is incorrect|密码错误|密码不正确/i,
  totp: /invalid (?:verification )?code|incorrect (?:verification )?code|wrong (?:verification )?code|code (?:is )?(?:invalid|incorrect|expired)|验证码(?:不正确|错误|已过期)/i,
};

export function browserExecutable() {
  // The host sets this only after checking its administrator-installed manifest.
  if (process.env.MQ_LOGIN_BROWSER_PATH) return isAbsolute(process.env.MQ_LOGIN_BROWSER_PATH) && existsSync(process.env.MQ_LOGIN_BROWSER_PATH) ? process.env.MQ_LOGIN_BROWSER_PATH : '';
  const candidates = process.platform === 'darwin'
    ? ['/Applications/Google Chrome.app/Contents/MacOS/Google Chrome','/Applications/Chromium.app/Contents/MacOS/Chromium']
    : process.platform === 'win32'
      ? [process.env.PROGRAMFILES && `${process.env.PROGRAMFILES}\\Google\\Chrome\\Application\\chrome.exe`,process.env['PROGRAMFILES(X86)'] && `${process.env['PROGRAMFILES(X86)']}\\Microsoft\\Edge\\Application\\msedge.exe`]
      : ['/usr/bin/google-chrome','/usr/bin/google-chrome-stable','/usr/bin/chromium','/usr/bin/chromium-browser'];
  return candidates.find(candidate => candidate && existsSync(candidate)) || '';
}

export function validateInput(input, policy = {}) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) fail('invalid_input');
  const keys = new Set(['version','job_id','attempt','authorization_url','redirect_uri','state','credentials','proxy_url','timeout_seconds','workspace_id']);
  if (Object.keys(input).some(key => !keys.has(key)) || input.version !== 1) fail('invalid_input');
  for (const name of ['job_id','state']) if (typeof input[name] !== 'string' || input[name].length < 1 || input[name].length > 256) fail('invalid_input');
  if (!Number.isInteger(input.attempt) || input.attempt < 1 || input.attempt > 100) fail('invalid_input');
  if (typeof input.authorization_url !== 'string' || input.authorization_url.length > 8192 || typeof input.redirect_uri !== 'string' || input.redirect_uri.length > 1024) fail('invalid_input');
  let auth, redirect;
  try { auth = new URL(input.authorization_url); redirect = new URL(input.redirect_uri); } catch { fail('invalid_input'); }
  if (auth.origin !== (policy.authOrigin || AUTH_ORIGIN) || auth.pathname !== '/oauth/authorize' || auth.username || auth.password || auth.hash) fail('invalid_input');
  if (!['localhost','127.0.0.1','[::1]'].includes(redirect.hostname) || redirect.protocol !== 'http:' || redirect.pathname !== '/auth/callback' || redirect.search || redirect.hash || redirect.username || redirect.password || !redirect.port) fail('invalid_input');
  for (const name of ['client_id','redirect_uri','state','response_type','code_challenge','code_challenge_method','scope']) if (auth.searchParams.getAll(name).length !== 1) fail('invalid_input');
  if (auth.searchParams.get('client_id') !== CLIENT_ID || auth.searchParams.get('redirect_uri') !== input.redirect_uri || auth.searchParams.get('state') !== input.state || auth.searchParams.get('response_type') !== 'code' || auth.searchParams.get('code_challenge_method') !== 'S256' || !/^[A-Za-z0-9_-]{43}$/.test(auth.searchParams.get('code_challenge'))) fail('invalid_input');
  const scopes = new Set(auth.searchParams.get('scope').split(/\s+/));
  if (!['openid','profile','email','offline_access'].every(scope => scopes.has(scope))) fail('invalid_input');
  const credentials = input.credentials;
  if (!credentials || typeof credentials !== 'object' || Array.isArray(credentials) || Object.keys(credentials).some(key => !['email','password','totp_secret'].includes(key))) fail('invalid_input');
  if (typeof credentials.email !== 'string' || credentials.email.length > 320 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(credentials.email) || typeof credentials.password !== 'string' || credentials.password.length < 1 || Buffer.byteLength(credentials.password) > 4096) fail('invalid_input');
  try { normalizeTotpSecret(credentials.totp_secret); } catch (error) { fail(error.message); }
  if (input.workspace_id != null && (typeof input.workspace_id !== 'string' || input.workspace_id.length > 256)) fail('invalid_input');
  if (input.timeout_seconds != null && (!Number.isInteger(input.timeout_seconds) || input.timeout_seconds < 10 || input.timeout_seconds > 900)) fail('invalid_input');
  browserProxy(input.proxy_url);
  return input;
}

export function browserProxy(value) {
  if (value == null || value === '') return undefined;
  if (typeof value !== 'string' || Buffer.byteLength(value) > 4096) fail('invalid_input');
  value = value.trim();
  if (!value || ['direct','off','none'].includes(value.toLowerCase())) return undefined;
  if (/[\u0000-\u0020\u007f?#]/.test(value)) fail('invalid_input');
  let parsed, username, password;
  try {
    parsed = new URL(value);
    username = decodeURIComponent(parsed.username);
    password = decodeURIComponent(parsed.password);
  } catch { fail('invalid_input'); }
  if (!['http:','https:','socks5:','socks5h:'].includes(parsed.protocol) || !parsed.hostname || parsed.hash || parsed.search || !['','/'].includes(parsed.pathname) || (parsed.port && Number(parsed.port) < 1) || /[\u0000-\u001f\u007f]/.test(username + password)) fail('invalid_input');
  const socks = ['socks5:','socks5h:'].includes(parsed.protocol);
  const hasCredentials = /^[^:]+:\/\/[^/]*@/.test(value);
  if (socks && hasCredentials) fail('proxy_unsupported', 'browser_start');
  if (hasCredentials && !username) fail('invalid_input');
  // Chromium resolves SOCKS5 targets through the proxy but does not accept the socks5h alias.
  return { server: `${socks ? 'socks5:' : parsed.protocol}//${parsed.host}`, ...(username || password ? { username, password } : {}) };
}

export function validateCallback(value, input) {
  let url, target;
  try { url = new URL(value); target = new URL(input.redirect_uri); } catch { fail('callback_invalid'); }
  if (url.origin !== target.origin || url.pathname !== target.pathname || url.hash || url.username || url.password || url.toString().length > 8192) fail('callback_invalid');
  if (url.searchParams.getAll('state').length !== 1 || url.searchParams.get('state') !== input.state) fail('callback_state_mismatch');
  if (url.searchParams.has('error')) fail('authorization_denied');
  if (url.searchParams.getAll('code').length !== 1 || !url.searchParams.get('code') || url.searchParams.get('code').length > 4096) fail('callback_invalid');
  return value;
}

async function visible(locator) {
  for (let index = 0, count = Math.min(await locator.count(), 12); index < count; index++) {
    const item = locator.nth(index);
    if (await item.isVisible()) return item;
  }
  return null;
}
async function submit(page, allowedNames) {
  const button = await visible(page.getByRole('button', { name: allowedNames || /^(?:continue|next|sign in|log in|verify|submit|继续|下一步|登录|验证|确认)$/i }));
  if (!button) fail('interaction_required', '', 'submit_unavailable');
  await button.click({ timeout: 10000 });
}

// All test endpoint overrides are dependency injection; the worker IPC cannot enable them.
export async function runLogin(raw, { chromium, executablePath = browserExecutable(), policy = {}, signal, now = Date.now, onProgress } = {}) {
  const input = validateInput(raw, policy);
  if (signal?.aborted) fail('cancelled');
  if (!chromium) fail('runtime_unavailable');
  if (!executablePath) fail('browser_unavailable');
  const authOrigin = policy.authOrigin || AUTH_ORIGIN;
  const chatOrigin = policy.chatOrigin || 'https://chatgpt.com';
  const verificationOrigin = policy.browserVerificationOrigin || 'https://challenges.cloudflare.com';
  const proxy = browserProxy(input.proxy_url);
  let browser, context, callback, fatal, deadline, cancel;
  let stage = 'browser_start';
  let oauthStarted = !!policy.directOAuthFixture;
  const progress = value => {
    if (!ERROR_STAGES.has(value)) return;
    if (stage !== value || value === 'browser_start') { stage = value; onProgress?.(value); }
  };
  const started = Date.now();
  const timeout = input.timeout_seconds || 180;
  let usernameSent = false, passwordSent = false, totpSent = false, factorChosen = false, consentSent = false, accountSelected = false;
  const submitted = new Set();
  let verificationSince = 0;
  let verificationResumeStage = '', readyDocument = '', readySince = 0;
  const verificationTimeout = policy.browserVerificationTimeoutMs ?? 30000;
  let closing;
  const close = () => closing ||= (async () => { if (context) await context.close().catch(() => {}); if (browser) await browser.close().catch(() => {}); })();
  try {
    progress('browser_start');
    browser = await chromium.launch({ executablePath, headless: true, timeout: 20000, proxy,
      env: Object.fromEntries(['PATH','TMPDIR','TEMP','TMP','SYSTEMROOT','WINDIR','HOME','LANG'].filter(name => process.env[name]).map(name => [name, process.env[name]])),
      args: ['--disable-breakpad','--disable-crash-reporter','--disable-background-networking','--disable-component-update','--disable-sync','--no-first-run', ...(!proxy ? ['--no-proxy-server'] : [])],
    });
    context = await browser.newContext({ locale: 'en-US', acceptDownloads: false, ignoreHTTPSErrors: false, serviceWorkers: 'block' });
    context.setDefaultTimeout(10000);
    cancel = () => { fatal = new LoginError('cancelled'); void close(); };
    signal?.addEventListener('abort', cancel, { once: true });
    if (signal?.aborted) cancel();
    deadline = setTimeout(() => { fatal = new LoginError('timeout', stage, stage === 'browser_verification' ? 'browser_verification_timeout' : ''); void close(); }, timeout * 1000);
    await context.route('**/*', async route => {
      const request = route.request();
      let url;
      try { url = new URL(request.url()); } catch { await route.abort(); return; }
      const target = new URL(input.redirect_uri);
      if (url.origin === target.origin && url.pathname === target.pathname) {
        if (!oauthStarted) { fatal = new LoginError('callback_invalid', stage); await route.abort(); return; }
        progress('callback');
        try { callback = validateCallback(request.url(), input); } catch (error) { fatal = error; }
        await route.abort(); return;
      }
      const verificationHost = url.origin === verificationOrigin || (!policy.browserVerificationOrigin && url.protocol === 'https:' && url.hostname.endsWith('.challenges.cloudflare.com'));
      const verificationResource = verificationHost && /^\/(?:turnstile\/|cdn-cgi\/challenge-platform\/)/.test(url.pathname) && !(request.isNavigationRequest() && !request.frame().parentFrame());
      const allowed = verificationResource || (policy.authOrigin ? [authOrigin, chatOrigin].includes(url.origin) : url.protocol === 'https:' && (OFFICIAL_HOSTS.has(url.hostname) || url.hostname.endsWith('.oaistatic.com')));
      if (!allowed || url.username || url.password || FORBIDDEN_PATH.test(url.pathname)) {
        if (request.isNavigationRequest()) fatal = new LoginError('interaction_required', stage, 'unsupported_navigation');
        await route.abort(); return;
      }
      if (!['GET','HEAD','OPTIONS'].includes(request.method())) {
        // Let the official page perform its browser check; never synthesize challenge answers.
        const browserVerification = ([authOrigin, chatOrigin].includes(url.origin) || verificationResource) && url.pathname.startsWith('/cdn-cgi/challenge-platform/') && request.method() === 'POST' && !request.isNavigationRequest() && ['xhr','fetch'].includes(request.resourceType());
        if (url.origin === authOrigin && !POSTS.has(url.pathname) && !browserVerification) {
          fatal = new LoginError('interaction_required', stage, 'unsupported_request'); await route.abort(); return;
        }
        const webSignin = url.origin === chatOrigin && url.pathname === '/api/auth/signin/openai' && request.method() === 'POST';
        if (url.origin !== authOrigin && url.hostname !== 'sentinel.openai.com' && !browserVerification && !webSignin) { await route.abort(); return; }
        if (['/api/accounts/password/verify','/api/accounts/mfa/verify'].includes(url.pathname)) {
          if (submitted.has(url.pathname)) { fatal = new LoginError(url.pathname.includes('/mfa/') ? 'totp_rejected' : 'password_rejected'); await route.abort(); return; }
          submitted.add(url.pathname);
        }
      }
      await route.continue();
    });
    const page = await context.newPage();
    page.on('dialog', dialog => { fatal = new LoginError('interaction_required', stage, 'unexpected_dialog'); void dialog.dismiss(); });
    page.on('download', download => { fatal = new LoginError('interaction_required', stage, 'unexpected_download'); void download.cancel(); });
    page.on('response', response => {
      const status = response.status(), path = new URL(response.url()).pathname;
      // Redirected document responses may bypass the initial route handler.
      if (response.request().isNavigationRequest() && FORBIDDEN_PATH.test(path)) fatal = new LoginError('interaction_required', stage, 'unsupported_navigation');
      else if (policy.directOAuthFixture && status === 429) fatal = new LoginError('rate_limited');
      else if (policy.directOAuthFixture && status >= 500 && path.startsWith('/api/accounts/')) fatal = new LoginError('service_error');
      else if (policy.directOAuthFixture && status >= 400 && path === '/api/accounts/password/verify') fatal = new LoginError(status === 400 || status === 401 ? 'password_rejected' : 'interaction_required', 'password', 'upstream_rejected');
      else if (policy.directOAuthFixture && status >= 400 && path === '/api/accounts/mfa/verify') fatal = new LoginError(status === 400 || status === 401 ? 'totp_rejected' : 'interaction_required', 'totp', 'upstream_rejected');
    });
    const checkpoint = async () => {
      const readinessDeadline = Date.now() + 30000;
      for (;;) {
        if (fatal) throw fatal;
        if (signal?.aborted) fail('cancelled');
        if (callback) return;
        let inspectionTimer;
        let document;
        try {
          document = await Promise.race([
            page.evaluate(() => ({
              url: location.href,
              identity: performance.timeOrigin,
              ready: globalThis.document.readyState !== 'loading',
              title: globalThis.document.title,
              text: (globalThis.document.body?.innerText || '').slice(0,32000),
              hasSessionChoice: !!globalThis.document.querySelector('input[name="session_id"]'),
            })).catch(() => null),
            new Promise(resolve => { inspectionTimer = setTimeout(() => resolve(null), 2000); }),
          ]);
        } finally { clearTimeout(inspectionTimer); }
        if (fatal) throw fatal;
        if (signal?.aborted) fail('cancelled');
        if (callback) return;
        const text = document?.text || '';
        // Only visible instructions for a human action justify this diagnosis.
        if (/verify (?:that )?you(?: are|'re|’re) human|验证您是人类|验证您是真人|确认您是人类|请完成.{0,12}人机验证/i.test(text)) fail('interaction_required','browser_verification','human_verification');
        const checking = document && (/just a moment/i.test(document.title) || /checking your browser|performing security verification|正在检查.*浏览器/i.test(text));
        if (checking) {
          if (!verificationSince) {
            verificationSince = Date.now();
            verificationResumeStage = stage;
          }
          progress('browser_verification');
        }
        if (verificationSince && Date.now() - verificationSince >= verificationTimeout) fail('timeout','browser_verification','browser_verification_timeout');
        // Navigation gaps and unreadable documents are not evidence that a check passed.
        const readable = document?.ready && document.url !== 'about:blank' && (text.trim() || document.hasSessionChoice);
        if (!checking && readable) {
          const key = `${document.identity}:${document.url}:${document.title}`;
          if (readyDocument !== key) { readyDocument = key; readySince = Date.now(); }
          if (Date.now() - readySince >= 400) {
            if (verificationSince) {
              verificationSince = 0;
              const resume = verificationResumeStage;
              verificationResumeStage = '';
              progress(resume);
            }
            return;
          }
        } else { readyDocument = ''; readySince = 0; }
        if (Date.now() >= readinessDeadline) fail('timeout',stage,verificationSince ? 'browser_verification_timeout' : '');
        await delay(200);
      }
    };
    const navigate = async url => {
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30000 }).catch(error => { if (!callback && !fatal) throw error; });
      await checkpoint();
    };
    if (!policy.directOAuthFixture) {
      await runReferenceFlow({page,context,input,authOrigin,chatOrigin,navigate,checkpoint,progress,now,
        fail:(code,where,reason)=>fail(code,where||stage,reason),getCallback:()=>callback,beginOAuth:()=>{oauthStarted=true;}});
      if (fatal) throw fatal;
      return {version:1,job_id:input.job_id,attempt:input.attempt,callback_url:validateCallback(callback,input)};
    }
    stage = 'authorization';
    await page.goto(input.authorization_url, { waitUntil: 'domcontentloaded', timeout: 30000 }).catch(error => { if (!callback && !fatal) throw error; });
    let idleSince = Date.now();
    while (!callback) {
      if (fatal) throw fatal;
      if (signal?.aborted) fail('cancelled');
      if (Date.now() - started > timeout * 1000) fail('timeout');
      const url = new URL(page.url());
      const text = (await page.locator('body').innerText({ timeout: 2000 }).catch(() => '')).slice(0, 32000);
      const title = await page.title().catch(() => '');
      const browserVerificationPage = /just a moment/i.test(title) || /checking your browser|performing security verification|正在检查.*浏览器/i.test(text);
      if (browserVerificationPage) {
        verificationSince ||= Date.now();
        if (Date.now() - verificationSince >= verificationTimeout) fail('interaction_required', stage, 'human_verification');
        await delay(200);
        continue;
      }
      if (verificationSince) { verificationSince = 0; idleSince = Date.now(); }
      if (/captcha|verify you are human|verify you're human|security check|验证您是人类/i.test(text)) fail('interaction_required', stage, 'human_verification');
      if (/\/email-verification(?:\/|$)/.test(url.pathname)) fail('interaction_required', 'email', 'email_verification');
      if (/\/(?:phone-verification|add-phone)(?:\/|$)/.test(url.pathname)) fail('interaction_required', stage, 'phone_verification');
      if (/\/(?:about-you|create-account|reset-password|passkey)(?:\/|$)/.test(url.pathname)) fail('interaction_required', stage, 'unsupported_page');
      if (passwordSent && REJECTION_TEXT.password.test(text)) fail('password_rejected');
      if (totpSent && REJECTION_TEXT.totp.test(text)) fail('totp_rejected');
      const password = await visible(page.locator('input[type=password]'));
      const email = await visible(page.locator('input[type=email], input[autocomplete=username], input[name=email]'));
      const otp = await visible(page.locator('input[autocomplete=one-time-code], input[name=code], input[name=otp], input[name=totp]'));
      if (password && !passwordSent) {
        stage = 'password';
        if (email) await email.fill(input.credentials.email);
        passwordSent = true;
        await password.fill(input.credentials.password);
        await submit(page); idleSince = Date.now();
      } else if (email && !usernameSent && !passwordSent) {
        stage = 'email';
        usernameSent = true;
        await email.fill(input.credentials.email);
        await submit(page); idleSince = Date.now();
      } else if (otp && !totpSent) {
        stage = 'totp';
        // Alternate verification links do not describe the currently displayed OTP form.
        const instructions = await page.locator('body').evaluate(body => {
          const copy = body.cloneNode(true);
          copy.querySelectorAll('button,a,nav,footer,script,style').forEach(node => node.remove());
          return (copy.textContent || '').slice(0, 32000);
        });
        if (!/authenticator|authentication app|two.factor|2fa|mfa|身份验证|认证器/i.test(instructions + url.pathname) || /email.*code|text message|sms/i.test(instructions)) fail('interaction_required', stage, 'totp_method_unconfirmed');
        totpSent = true;
        await otp.fill(generateTotp(input.credentials.totp_secret, now()));
        await submit(page); idleSince = Date.now();
      } else if (!factorChosen && /mfa|two.factor/i.test(text + url.pathname)) {
        stage = 'totp';
        const factor = await visible(page.getByRole('button', { name: /^(?:authenticator app|authentication app|use (?:an? )?authenticator app|身份验证器应用)$/i }));
        if (factor) { factorChosen = true; await factor.click(); idleSince = Date.now(); }
      } else if (!accountSelected && /choose.account|choose-an-account/.test(url.pathname)) {
        const account = await visible(page.getByRole('button').filter({ hasText: input.credentials.email }));
        if (!account) fail('interaction_required', stage, 'account_selection');
        accountSelected = true; await account.click(); idleSince = Date.now();
      } else if (!consentSent && /consent/.test(url.pathname)) {
        stage = 'consent';
        // Consent is for the host-supplied Codex authorization session only.
        if (!/codex/i.test(text)) fail('interaction_required', stage, 'unsupported_page');
        consentSent = true; await submit(page, /^(?:continue|authorize|allow|accept|继续|授权|允许)$/i); idleSince = Date.now();
      } else if (/workspace|organization/.test(url.pathname)) {
        stage = 'workspace';
        // Never choose an arbitrary workspace. The host must receive a candidate for a proven identity.
        const matches = page.locator('[data-workspace-id]');
        let option;
        for (let index = 0; index < Math.min(await matches.count(), 32); index++) {
          const item = matches.nth(index);
          if (input.workspace_id && await item.getAttribute('data-workspace-id') === input.workspace_id && await item.isVisible()) option = item;
        }
        if (option) { await option.click(); idleSince = Date.now(); }
        else fail('interaction_required', stage, 'workspace_selection');
      }
      if (!callback) await delay(200);
      if (Date.now() - idleSince > 15000) fail('interaction_required', stage, 'unsupported_page');
    }
    if (fatal) throw fatal;
    return { version: 1, job_id: input.job_id, attempt: input.attempt, callback_url: validateCallback(callback, input) };
  } catch (error) {
    const failure = fatal || (error instanceof LoginError ? error : new LoginError(signal?.aborted ? 'cancelled' : error?.name === 'TimeoutError' ? 'timeout' : 'transport_error', stage,
      /ERR_(?:PROXY_CONNECTION_FAILED|TUNNEL_CONNECTION_FAILED|SOCKS_CONNECTION_FAILED|NO_SUPPORTED_PROXIES|PROXY_AUTH_UNSUPPORTED|INVALID_AUTH_CREDENTIALS)/.test(error?.message || '') ? 'proxy_connection_failed' : ''));
    if (!failure.stage) failure.stage = stage;
    throw failure;
  } finally {
    clearTimeout(deadline);
    if (cancel) signal?.removeEventListener('abort', cancel);
    await close();
  }
}
