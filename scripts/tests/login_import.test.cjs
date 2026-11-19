const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const source = fs.readFileSync(path.join(__dirname, '../../plugin/ui/login.js'), 'utf8')
  .replace(/^import .*;$/gm, '').replace(/^export /gm, '');
const context = vm.createContext({ URL, Set, Map, Error });
new vm.Script(source + '\nthis.parser = { parseAccountImport, validImportTOTP };').runInContext(context);
const { parseAccountImport, validImportTOTP } = context.parser;
const seed = 'JBSWY3DPEHPK3PXP';
const row = (email = 'fixture@example.test', password = 'password', totp = seed) => `${email}----${password}----${totp}`;

test('import preserves opaque passwords, normalizes email and keeps original line numbers', () => {
  const result = parseAccountImport('\n' + row(' Fixture@Example.Test ', '  opaque----password\t ') + '\n');
  assert.equal(result[0].email, 'fixture@example.test');
  assert.equal(result[0].password, '  opaque----password\t ');
  assert.equal(result[0].totp_secret, seed);
  assert.throws(() => parseAccountImport('\n' + row() + '\n\ninvalid'), /第 4 行/);
});
test('duplicates and existing accounts are rejected before submission', () => {
  assert.throws(() => parseAccountImport(row() + '\n\n' + row('FIXTURE@example.test')), /第 3 行.*重复/);
  assert.throws(() => parseAccountImport('\n' + row(), ['FIXTURE@EXAMPLE.TEST']), /第 2 行.*已存在/);
});
test('import accepts only 1 to 100 nonempty lines', () => {
  assert.throws(() => parseAccountImport(' \n\n'), /1–100/);
  const rows = Array.from({ length: 101 }, (_, i) => row(`fixture${i}@example.test`));
  assert.equal(parseAccountImport(rows.slice(0, 100).join('\n')).length, 100);
  assert.throws(() => parseAccountImport(rows.join('\n')), /1–100/);
});
test('TOTP permits canonical Base32 and standard six digit SHA1 URI', () => {
  for (const value of [seed, ' jbsw-y3dp\tehpk 3pxp==\n', 'A'.repeat(18), `otpauth://totp/Fixture?secret=${seed}&algorithm=sha1&digits=6&period=30`]) {
    assert.equal(validImportTOTP(value), true, value);
  }
});
test('TOTP rejects codes, bad tail bits, duplicate parameters and unsupported options', () => {
  const uri = `otpauth://totp/Fixture?secret=${seed}`;
  for (const value of ['123456', 'A'.repeat(17) + 'B', 'A'.repeat(17), `${uri}&digits=8`, `${uri}&period=60`, `${uri}&algorithm=SHA256`, `${uri}&%73ecret=${seed}`]) {
    assert.equal(validImportTOTP(value), false, value);
  }
  assert.throws(() => parseAccountImport('\n\n' + row('fixture@example.test', 'password', '123456')), /第 3 行.*2FA/);
});
test('TOTP URI rejects userinfo and malformed raw URLs before URL normalization', () => {
  for (const value of [
    `otpauth://@totp/Fixture?secret=${seed}`, `otpauth://user@totp/Fixture?secret=${seed}`,
    `otpauth://totp/%ZZ?secret=${seed}`, `otpauth://to\ttp/Fixture?secret=${seed}`,
    `otpauth://totp/Fixture?secret=${seed}\t`, `otpauth://totp:80/Fixture?secret=${seed}`,
    `otpauth://TOTP/Fixture?secret=${seed}`, `otpauth://totp/Fixture?secret=${seed}#fragment`,
  ]) assert.equal(validImportTOTP(value), false, value);
});
