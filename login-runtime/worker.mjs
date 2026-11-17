#!/usr/bin/env node
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { browserExecutable, ERROR_CODES, errorDiagnostics, LoginError, runLogin, VERSION } from './core.mjs';
import { retainProcessGroup } from './process-scope.mjs';

retainProcessGroup();

const inputLimit = 16384;
const controller = new AbortController();
let identity = { version: 1 };
for (const name of ['SIGTERM','SIGINT']) process.once(name, () => controller.abort());
process.on('uncaughtException', () => { process.stdout.write(JSON.stringify({...identity,error_code:'browser_failed'})+'\n'); process.exit(1); });
process.on('unhandledRejection', () => { process.stdout.write(JSON.stringify({...identity,error_code:'browser_failed'})+'\n'); process.exit(1); });
try {
  const { chromium } = await import('playwright-core');
  if (process.argv.length === 3 && process.argv[2] === '--self-test') {
    const executable = browserExecutable();
    if (!executable) throw new LoginError('browser_unavailable');
    const browser = await chromium.launch({ executablePath: executable, headless: true, timeout: 20000, args: ['--disable-breakpad','--disable-crash-reporter','--no-first-run'] });
    const version = browser.version();
    await browser.close();
    const sha256 = createHash('sha256').update(await readFile(executable)).digest('hex');
    process.stdout.write(JSON.stringify({ version: 1, runtime_version: VERSION, engine: 'official_browser_forms', platform: process.platform, arch: process.arch, methods: ['password_totp'], capabilities: ['external_authorize_session','callback_code_only','bounded_cancel'], browser_executable: executable, browser_version: version, browser_sha256: sha256 }) + '\n');
  } else {
    if (process.argv.length !== 2) throw new LoginError('invalid_input');
    let size = 0;
    const chunks = [];
    for await (const chunk of process.stdin) {
      size += chunk.length;
      if (size > inputLimit) throw new LoginError('invalid_input');
      chunks.push(chunk);
    }
    let input;
    const bytes = Buffer.concat(chunks);
    for (const chunk of chunks) chunk.fill(0);
    try { input = JSON.parse(bytes.toString('utf8')); } catch { throw new LoginError('invalid_input'); } finally { bytes.fill(0); }
    if (typeof input?.job_id === 'string' && input.job_id.length <= 256) identity.job_id = input.job_id;
    if (Number.isInteger(input?.attempt)) identity.attempt = input.attempt;
    const result = await runLogin(input, { chromium, signal: controller.signal, onProgress: phase => {
      process.stdout.write(JSON.stringify({ ...identity, type: 'progress', phase }) + '\n');
    } });
    if (Buffer.byteLength(JSON.stringify(result)) > 12288) throw new LoginError('callback_invalid');
    process.stdout.write(JSON.stringify(result) + '\n');
  }
} catch (error) {
  const code = ERROR_CODES.has(error?.code) ? error.code : error?.code === 'ERR_MODULE_NOT_FOUND' ? 'runtime_unavailable' : 'browser_failed';
  process.stdout.write(JSON.stringify({ ...identity, error_code: code, ...errorDiagnostics(error) }) + '\n');
  process.exitCode = 1;
}
