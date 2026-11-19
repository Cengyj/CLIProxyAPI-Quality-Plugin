// Requests use a page-local session; the app manages explicit credential persistence.
export class DiscardedRequest extends Error {
  constructor() {
    super('The request belongs to an inactive view or session.');
    this.name = 'DiscardedRequest';
  }
}

export const isDiscarded = (error) => error instanceof DiscardedRequest || error?.name === 'AbortError';

export class ManagementClient {
  #key = '';
  #epoch = 0;
  #pending = new Set();

  constructor(prefix, onUnauthorized) {
    this.prefix = prefix;
    this.onUnauthorized = onUnauthorized;
  }

  get epoch() {
    return this.#epoch;
  }

  open(key) {
    this.clear();
    this.#key = key.trim();
  }

  clear() {
    this.#key = '';
    this.#epoch++;
    for (const controller of this.#pending) controller.abort();
    this.#pending.clear();
  }

  async request(path, method = 'GET', body) {
    return this.#send(this.prefix, path, method, body);
  }

  async requestHostLogin(path, method = 'GET', body, idempotencyKey) {
    const allowed = {
      '/capabilities': ['GET'], '/targets': ['GET'],
      '/jobs': ['GET', 'DELETE'], '/detail': ['GET'], '/logs': ['GET'], '/candidate': ['GET'],
      '/callback': ['POST'], '/cancel': ['POST'], '/commit': ['POST'], '/discard': ['POST'],
      '/vault/status': ['GET'],
      '/accounts': ['GET'], '/accounts/detail': ['GET'], '/accounts/import': ['POST'], '/accounts/entry': ['PUT', 'DELETE'],
      '/accounts/security': ['POST'], '/accounts/security/complete': ['POST'],
      '/conversion/preview': ['POST'], '/conversion/upload': ['POST'],
      '/accounts/start': ['POST'], '/accounts/inspect': ['POST'], '/accounts/settings': ['GET', 'PUT'],
    };
    const parsed = new URL(path, 'https://management.invalid');
    const queryKeys = new Set(['id', 'auth_id', 'account_id', 'cursor', 'limit']);
    if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//') || path.split('?')[0] !== parsed.pathname || parsed.origin !== 'https://management.invalid' || parsed.hash ||
        !allowed[parsed.pathname]?.includes(method) || [...parsed.searchParams.keys()].some(key => !queryKeys.has(key))) {
      throw new Error('不支持的登录管理操作。');
    }
    if (method !== 'GET' && !idempotencyKey) throw new Error('登录操作缺少请求标识。');
    return this.#send(`${this.prefix}/login`, parsed.pathname + parsed.search, method, body, idempotencyKey);
  }

  async #send(prefix, path, method, body, idempotencyKey) {
    if (!this.#key) throw new Error('请先连接管理接口。');
    const epoch = this.#epoch;
    const controller = new AbortController();
    this.#pending.add(controller);
    let readTimedOut = false;
    const readTimer = method === 'GET' ? setTimeout(() => { readTimedOut = true; controller.abort(); }, 15000) : null;
    const headers = { Authorization: `Bearer ${this.#key}`, Accept: 'application/json' };
    if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    try {
      const response = await fetch(`${prefix}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        cache: 'no-store',
        credentials: 'omit',
        redirect: 'error',
        signal: controller.signal,
      });
      const text = response.status === 204 ? '' : await response.text();
      if (epoch !== this.#epoch) throw new DiscardedRequest();
      if (response.status === 401) {
        this.onUnauthorized?.(this.#key);
        throw new DiscardedRequest();
      }
      let data = {};
      try {
        data = text ? JSON.parse(text) : {};
      } catch {
        const invalid = new Error(`管理接口返回了无法解析的响应 (${response.status})，请核对任务状态。`);
        invalid.code = 'management_response_invalid';
        invalid.outcomeUnknown = method !== 'GET';
        throw invalid;
      }
      if (!response.ok) {
        const message = data.error?.message || data.error || data.message || `接口返回 ${response.status}`;
        const code = data.code || data.error?.code;
        const error = new Error(
          `${typeof message === 'string' ? message : '操作失败'}${code ? ` (${code})` : ''}`,
        );
        error.status = response.status;
        error.code = code;
        throw error;
      }
      return data;
    } catch (error) {
      if (readTimedOut && epoch === this.#epoch) {
        const timeout = new Error('管理数据读取超时，将重新读取。');
        timeout.code = 'read_timeout';
        throw timeout;
      }
      if (epoch !== this.#epoch || error.name === 'AbortError') throw new DiscardedRequest();
      if (error instanceof TypeError) {
        const network = new Error('无法连接管理接口，请检查宿主连接后核对状态。');
        network.code = 'management_network_error';
        network.outcomeUnknown = method !== 'GET';
        throw network;
      }
      throw error;
    } finally {
      clearTimeout(readTimer);
      this.#pending.delete(controller);
    }
  }
}
