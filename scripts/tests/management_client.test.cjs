// Exercise the real browser client with deterministic transport and clock substitutes.
// No browser, network listener, real credentials, or upstream calls are involved.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const sourcePath = path.resolve(__dirname, '../../plugin/ui/client.js');
const source = fs.readFileSync(sourcePath, 'utf8')
  .replace(/^export\s+(?=class |const )/gm, '')
  + '\nglobalThis.clientModule = { ManagementClient, DiscardedRequest, isDiscarded };\n';
const prefix = '/v0/management/plugins/model-quality';

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function abortError() {
  const error = new Error('synthetic cancellation');
  error.name = 'AbortError';
  return error;
}
function response(value, status = 200) {
  return { status, ok: status >= 200 && status < 300, text: async () => JSON.stringify(value) };
}

class FakeClock {
  now = 0;
  sequence = 0;
  timers = new Map();
  setTimeout = (callback, delay = 0) => {
    const id = ++this.sequence;
    this.timers.set(id, { at: this.now + Number(delay), callback });
    return id;
  };
  clearTimeout = id => this.timers.delete(id);
  advance(milliseconds) {
    const end = this.now + milliseconds;
    for (;;) {
      const next = [...this.timers].filter(([, value]) => value.at <= end)
        .sort((a, b) => a[1].at - b[1].at || a[0] - b[0])[0];
      if (!next) break;
      this.now = next[1].at;
      this.timers.delete(next[0]);
      next[1].callback();
    }
    this.now = end;
  }
}

function harness({ respectAbort = true, onUnauthorized } = {}) {
  const clock = new FakeClock();
  const calls = [];
  const fetch = (url, options) => {
    const pending = deferred();
    const call = { url, options, ...pending };
    calls.push(call);
    if (respectAbort) {
      if (options.signal.aborted) pending.reject(abortError());
      else options.signal.addEventListener('abort', () => pending.reject(abortError()), { once: true });
    }
    return pending.promise;
  };
  const context = vm.createContext({
    URL, AbortController, Error, TypeError,
    fetch, setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout,
  });
  new vm.Script(source, { filename: sourcePath }).runInContext(context);
  const module = context.clientModule;
  const client = new module.ManagementClient(prefix, onUnauthorized);
  client.open('fixture-management-key');
  return { clock, calls, client, ...module };
}

test('a GET aborts at 15 seconds and a later read succeeds in the same session', async () => {
  const { client, clock, calls, isDiscarded } = harness();
  const request = client.requestHostLogin('/accounts');
  const failure = assert.rejects(request, error => error.code === 'read_timeout' && !isDiscarded(error));
  assert.equal(calls.length, 1);
  clock.advance(14999);
  assert.equal(calls[0].options.signal.aborted, false);
  clock.advance(1);
  await failure;
  assert.equal(calls[0].options.signal.aborted, true);
  assert.equal(calls.length, 1, 'a timed-out read is not automatically replayed by the client');
  assert.equal(clock.timers.size, 0);

  const recovered = client.requestHostLogin('/accounts');
  assert.equal(calls[1].options.headers.Authorization, 'Bearer fixture-management-key');
  calls[1].resolve(response({ items: [{ id: 'fixture-account' }] }));
  assert.equal((await recovered).items[0].id, 'fixture-account');
  assert.equal(clock.timers.size, 0);
});

test('the GET deadline also bounds response-body reading', async () => {
  const { client, clock, calls } = harness();
  const reading = client.requestHostLogin('/jobs?limit=200');
  const failure = assert.rejects(reading, error => error.code === 'read_timeout');
  const body = deferred();
  const enteredBody = deferred();
  calls[0].options.signal.addEventListener('abort', () => body.reject(abortError()), { once: true });
  calls[0].resolve({ status: 200, ok: true, text: () => { enteredBody.resolve(); return body.promise; } });
  await enteredBody.promise;
  clock.advance(15000);
  await failure;
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
});

test('clear discards pending reads and requires a new management session', async () => {
  const { client, calls, clock, isDiscarded } = harness();
  const epoch = client.epoch;
  const reading = client.requestHostLogin('/accounts/detail?id=fixture-account');
  const discarded = assert.rejects(reading, isDiscarded);
  client.clear();
  await discarded;
  assert.equal(calls[0].options.signal.aborted, true);
  assert.ok(client.epoch > epoch);
  assert.equal(clock.timers.size, 0);
  await assert.rejects(client.requestHostLogin('/accounts'), /请先连接管理接口/);
  assert.equal(calls.length, 1);
});

test('opening a new session rejects a late old response and cannot log out the new key', async () => {
  const invalidated = [];
  const { client, calls, clock, isDiscarded } = harness({ respectAbort: false, onUnauthorized: key => invalidated.push(key) });
  const oldRead = client.requestHostLogin('/accounts');
  const discarded = assert.rejects(oldRead, isDiscarded);
  client.open('replacement-fixture-key');
  assert.equal(calls[0].options.signal.aborted, true);
  const currentRead = client.requestHostLogin('/accounts');
  calls[1].resolve(response({ items: [] }));
  await currentRead;
  calls[0].resolve(response({ code: 'unauthorized' }, 401));
  await discarded;
  assert.deepEqual(invalidated, []);
  assert.equal(calls[1].options.headers.Authorization, 'Bearer replacement-fixture-key');
  assert.equal(clock.timers.size, 0);
});

test('a current-session 401 invokes logout once and discards the response', async () => {
  const invalidated = [];
  let client;
  const setup = harness({ onUnauthorized: key => { invalidated.push(key); client.clear(); } });
  client = setup.client;
  const reading = client.requestHostLogin('/accounts');
  const discarded = assert.rejects(reading, setup.isDiscarded);
  setup.calls[0].resolve(response({ code: 'unauthorized' }, 401));
  await discarded;
  assert.deepEqual(invalidated, ['fixture-management-key']);
  await assert.rejects(client.requestHostLogin('/accounts'), /请先连接管理接口/);
  assert.equal(setup.calls.length, 1);
  assert.equal(setup.clock.timers.size, 0);
});

test('account-scoped history is allowed while invalid routes never reach fetch', async () => {
  const { client, calls } = harness();
  const reading = client.requestHostLogin('/jobs?account_id=fixture-account&limit=200');
  assert.equal(calls[0].url, `${prefix}/login/jobs?account_id=fixture-account&limit=200`);
  calls[0].resolve(response({ items: [] }));
  await reading;
  for (const [route, method] of [
    ['jobs', 'GET'], ['https://outside.invalid/jobs', 'GET'], ['//outside.invalid/jobs', 'GET'],
    ['/accounts/../jobs', 'GET'], ['/jobs#fragment', 'GET'], ['/jobs?redirect=outside', 'GET'],
    ['/jobs?unknown=1', 'GET'], ['/accounts/start', 'GET'], ['/jobs', 'POST'], ['/accounts', 'PATCH'],
  ]) {
    await assert.rejects(client.requestHostLogin(route, method, {}, 'fixture-request'), /不支持的登录管理操作/, `${method} ${route}`);
  }
  await assert.rejects(client.requestHostLogin('/accounts/start', 'POST', { id: 'fixture-account' }), /缺少请求标识/);
  assert.equal(calls.length, 1);
});

test('writes have no client deadline, keep their idempotency key, and are sent only once', async () => {
  const { client, calls, clock } = harness();
  const body = { id: 'fixture-account', revision: 7 };
  let settled = false;
  const write = client.requestHostLogin('/accounts/start', 'POST', body, 'fixture-once');
  write.then(() => { settled = true; }, () => { settled = true; });
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
  clock.advance(120000);
  await Promise.resolve();
  assert.equal(settled, false);
  assert.equal(calls[0].options.signal.aborted, false);
  assert.equal(calls[0].options.headers['Idempotency-Key'], 'fixture-once');
  assert.equal(calls[0].options.method, 'POST');
  assert.deepEqual(JSON.parse(calls[0].options.body), body);
  calls[0].resolve(response({ id: 'fixture-job', status: 'queued' }));
  assert.equal((await write).status, 'queued');
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
});

test('a write transport failure is marked uncertain and never replayed', async () => {
  const { client, calls, clock } = harness();
  const write = client.requestHostLogin('/accounts/security', 'POST', { id: 'fixture-account', revision: 1, action: 'reset_totp' }, 'fixture-security-once');
  const failure = assert.rejects(write, error => {
    assert.equal(error.code, 'management_network_error');
    assert.doesNotMatch(error.message, /synthetic-secret|private-endpoint/);
    return true;
  });
  calls[0].reject(new TypeError('synthetic-secret at private-endpoint'));
  await failure;
  clock.advance(120000);
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
});

test('a GET transport error is distinct from an upstream login network_error', async () => {
  const { client, calls, clock } = harness();
  const read = client.requestHostLogin('/accounts');
  const failure = assert.rejects(read, error => error.code === 'management_network_error');
  calls[0].reject(new TypeError('synthetic network interruption'));
  await failure;
  const retry = client.requestHostLogin('/accounts');
  calls[1].resolve(response({ items: [] }));
  assert.equal((await retry).items.length, 0);
  assert.equal(calls.length, 2);
  assert.equal(clock.timers.size, 0);
});

test('a server refusal preserves its code and is not automatically retried', async () => {
  const { client, calls, clock } = harness();
  const write = client.requestHostLogin('/accounts/start', 'POST', { id: 'fixture-account', revision: 1 }, 'fixture-server-error');
  const failure = assert.rejects(write, error => error.status === 503 && error.code === 'storage_unavailable');
  calls[0].resolve(response({ error: 'storage temporarily unavailable', code: 'storage_unavailable' }, 503));
  await failure;
  clock.advance(120000);
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
});

test('an unreadable write response remains uncertain without replaying the request', async () => {
  const { client, calls, clock } = harness();
  const write = client.requestHostLogin('/accounts/security', 'POST', { id: 'fixture-account' }, 'fixture-truncated-once');
  const failure = assert.rejects(write, error => {
    assert.equal(error.code, 'management_response_invalid');
    assert.equal(error.outcomeUnknown, true);
    assert.doesNotMatch(error.message, /synthetic-secret/);
    return true;
  });
  calls[0].resolve({ status: 200, ok: true, text: async () => '{"synthetic-secret":' });
  await failure;
  clock.advance(120000);
  assert.equal(calls.length, 1);
  assert.equal(clock.timers.size, 0);
});
