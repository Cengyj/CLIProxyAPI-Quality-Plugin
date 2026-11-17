import { createHmac } from 'node:crypto';

// Adapted from the MIT-licensed toSub2 implementation pinned in SOURCE_MANIFEST.json.
export function normalizeTotpSecret(value) {
  if (typeof value !== 'string' || value.length > 2048) throw new Error('invalid_totp_secret');
  let seed = value;
  if (value.toLowerCase().startsWith('otpauth:')) {
    let uri;
    try { uri = new URL(value); } catch { throw new Error('invalid_totp_secret'); }
    if (uri.protocol !== 'otpauth:' || uri.hostname !== 'totp' || uri.hash || uri.username || uri.password) throw new Error('invalid_totp_secret');
    for (const name of ['secret', 'algorithm', 'digits', 'period']) {
      if (uri.searchParams.getAll(name).length > 1) throw new Error('invalid_totp_secret');
    }
    if ((uri.searchParams.get('algorithm') || 'SHA1').toUpperCase() !== 'SHA1'
      || (uri.searchParams.get('digits') || '6') !== '6'
      || (uri.searchParams.get('period') || '30') !== '30') throw new Error('unsupported_totp_parameters');
    seed = uri.searchParams.get('secret') || '';
  }
  seed = seed.toUpperCase().replace(/[\s-]/g, '').replace(/=+$/, '');
  if (!/^[A-Z2-7]{16,128}$/.test(seed) || ![0,2,4,5,7].includes(seed.length % 8)) throw new Error('invalid_totp_secret');
  let bits = '';
  for (const character of seed) bits += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'.indexOf(character).toString(2).padStart(5, '0');
  if (/1/.test(bits.slice(Math.floor(bits.length / 8) * 8))) throw new Error('invalid_totp_secret');
  return seed;
}

export function generateTotp(value, timestamp = Date.now()) {
  const seed = normalizeTotpSecret(value);
  let bits = '';
  for (const character of seed) bits += 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'.indexOf(character).toString(2).padStart(5, '0');
  const key = Buffer.from(Array.from({ length: Math.floor(bits.length / 8) }, (_, index) => Number.parseInt(bits.slice(index * 8, index * 8 + 8), 2)));
  if (!Number.isFinite(timestamp) || timestamp < 0) throw new Error('invalid_time');
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(timestamp / 30000)));
  const digest = createHmac('sha1', key).update(counter).digest();
  key.fill(0);
  const offset = digest[digest.length - 1] & 15;
  return String((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).padStart(6, '0');
}
