// Management credentials and cancellation stay inside one page-local session.
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
    if (!this.#key) throw new Error('请先连接管理接口。');
    const epoch = this.#epoch;
    const controller = new AbortController();
    this.#pending.add(controller);
    const headers = { Authorization: `Bearer ${this.#key}`, Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    try {
      const response = await fetch(`${this.prefix}${path}`, {
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
        this.onUnauthorized?.();
        throw new DiscardedRequest();
      }
      let data = {};
      try {
        data = text ? JSON.parse(text) : {};
      } catch {
        throw new Error(`管理接口返回了无法解析的响应 (${response.status})，请核对宿主版本。`);
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
      if (epoch !== this.#epoch || error.name === 'AbortError') throw new DiscardedRequest();
      if (error instanceof TypeError) throw new Error('无法连接管理接口，请检查宿主连接后重试。');
      throw error;
    } finally {
      this.#pending.delete(controller);
    }
  }
}
