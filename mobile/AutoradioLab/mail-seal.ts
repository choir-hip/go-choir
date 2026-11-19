// The seal on the phone's copy of the listener's mail: AES-256 in counter mode, then HMAC-SHA-256 over the whole (encrypt, then authenticate).
//
// WHY THIS IS HERE AND NOT A PLATFORM CALL (lead's default, not an owner ruling; the owner may strike it). The Swift store wrote every mail file with
// the file protection class "complete until first user authentication" and left the folder out of backups. Exact's `storage.fs` states no
// protection class on Apple (cutover file, X0 asks the owner's iPhone) and Android encrypts app files by the platform. Rather than depend on an
// unmeasured guarantee, the copy is sealed in shared code with a key held in Exact's secret store (Keychain on Apple, Keystore-wrapped files on
// Android): the bytes on disk are unreadable without the key, whatever the file's class. That is no weaker than the Swift store (the Keychain
// item is readable once the phone has been unlocked, as the file class was), and the wipe at sign-out forgets the key as well as deleting the files.
// Exact's `crypto.subtle` gives digests only (no AES, no HMAC), so the cipher is written here, once, and the tests check it against the FIPS-197
// vector and, under Bun, against Node's AES and HMAC for random inputs. If the secret store keeps nothing (a scripted drive never keeps secrets),
// there is no key, and nothing is written to disk at all (mail-store.ts): the mailbox works from memory.
//
// Format: "MS1" | 16-byte nonce (the counter's first block) | ciphertext | 32-byte tag. Keys: both derived from the one 32-byte master by SHA-256 with a label.

const SBOX = new Uint8Array(256);
(() => {
  // The S-box from its definition: the multiplicative inverse in GF(2^8), then the affine map.
  let p = 1, q = 1;
  do {
    p = (p ^ ((p << 1) & 0xff) ^ (p & 0x80 ? 0x1b : 0)) & 0xff;                 // p times 3
    q ^= q << 1; q ^= q << 2; q ^= q << 4; q &= 0xff; if (q & 0x80) q ^= 0x09;  // q divided by 3
    const x = q ^ ((q << 1) | (q >> 7)) ^ ((q << 2) | (q >> 6)) ^ ((q << 3) | (q >> 5)) ^ ((q << 4) | (q >> 4));
    SBOX[p] = (x ^ 0x63) & 0xff;
  } while (p !== 1);
  SBOX[0] = 0x63;
})();

const xtime = (b: number): number => ((b << 1) ^ (b & 0x80 ? 0x1b : 0)) & 0xff;

/** The 15 round keys of AES-256 (240 bytes). */
function expandKey(key: Uint8Array): Uint8Array {
  const w = new Uint8Array(240);
  w.set(key);
  let rcon = 1;
  for (let i = 8; i < 60; i++) {
    let t0 = w[4 * (i - 1)], t1 = w[4 * (i - 1) + 1], t2 = w[4 * (i - 1) + 2], t3 = w[4 * (i - 1) + 3];
    if (i % 8 === 0) {
      const r = t0;
      t0 = SBOX[t1] ^ rcon; t1 = SBOX[t2]; t2 = SBOX[t3]; t3 = SBOX[r];
      rcon = xtime(rcon);
    } else if (i % 8 === 4) {
      t0 = SBOX[t0]; t1 = SBOX[t1]; t2 = SBOX[t2]; t3 = SBOX[t3];
    }
    w[4 * i] = w[4 * (i - 8)] ^ t0; w[4 * i + 1] = w[4 * (i - 8) + 1] ^ t1; w[4 * i + 2] = w[4 * (i - 8) + 2] ^ t2; w[4 * i + 3] = w[4 * (i - 8) + 3] ^ t3;
  }
  return w;
}

/** One block, encrypted in place in `s` (16 bytes), with the round keys from expandKey. */
function encryptBlock(s: Uint8Array, w: Uint8Array): void {
  for (let i = 0; i < 16; i++) s[i] ^= w[i];
  for (let round = 1; round <= 14; round++) {
    for (let i = 0; i < 16; i++) s[i] = SBOX[s[i]];
    // ShiftRows (the state is column-major: s[4*c + r])
    let t = s[1]; s[1] = s[5]; s[5] = s[9]; s[9] = s[13]; s[13] = t;
    t = s[2]; s[2] = s[10]; s[10] = t; t = s[6]; s[6] = s[14]; s[14] = t;
    t = s[15]; s[15] = s[11]; s[11] = s[7]; s[7] = s[3]; s[3] = t;
    if (round < 14) {
      for (let c = 0; c < 16; c += 4) {
        const a0 = s[c], a1 = s[c + 1], a2 = s[c + 2], a3 = s[c + 3];
        const all = a0 ^ a1 ^ a2 ^ a3;
        s[c] = a0 ^ all ^ xtime(a0 ^ a1);
        s[c + 1] = a1 ^ all ^ xtime(a1 ^ a2);
        s[c + 2] = a2 ^ all ^ xtime(a2 ^ a3);
        s[c + 3] = a3 ^ all ^ xtime(a3 ^ a0);
      }
    }
    for (let i = 0; i < 16; i++) s[i] ^= w[16 * round + i];
  }
}

/** A turn's worth of work is 100 ms on the phone: a long input gives the turn back now and then. Bun has no scheduler, and there it is free. */
const BLOCKS_PER_TURN = 2048;
async function turn(): Promise<void> {
  const sch = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler;
  if (sch && typeof sch.yield === 'function') await sch.yield();
}

/** AES-256-CTR: the same call encrypts and decrypts. `nonce` is the first counter block (16 bytes), incremented as a 128-bit big-endian number. */
export async function ctr(key: Uint8Array, nonce: Uint8Array, data: Uint8Array): Promise<Uint8Array> {
  const w = expandKey(key);
  const counter = Uint8Array.from(nonce);
  const block = new Uint8Array(16);
  const out = new Uint8Array(data.length);
  let blocks = 0;
  for (let off = 0; off < data.length; off += 16) {
    block.set(counter);
    encryptBlock(block, w);
    const n = Math.min(16, data.length - off);
    for (let i = 0; i < n; i++) out[off + i] = data[off + i] ^ block[i];
    for (let i = 15; i >= 0; i--) { counter[i] = (counter[i] + 1) & 0xff; if (counter[i] !== 0) break; }
    if (++blocks % BLOCKS_PER_TURN === 0) await turn();
  }
  return out;
}

/** The AES block cipher alone (for the known-answer test). */
export function aes256EncryptBlock(key: Uint8Array, block: Uint8Array): Uint8Array {
  const s = Uint8Array.from(block);
  encryptBlock(s, expandKey(key));
  return s;
}

export async function sha256(data: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await crypto.subtle.digest('SHA-256', data as unknown as BufferSource));
}

const cat = (...parts: Uint8Array[]): Uint8Array => {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let o = 0;
  for (const p of parts) { out.set(p, o); o += p.length; }
  return out;
};
const utf8 = (s: string): Uint8Array => new TextEncoder().encode(s);

/** HMAC-SHA-256 (RFC 2104) from the digest Exact has. */
export async function hmacSha256(key: Uint8Array, data: Uint8Array): Promise<Uint8Array> {
  const k = key.length > 64 ? await sha256(key) : key;
  const ipad = new Uint8Array(64).fill(0x36), opad = new Uint8Array(64).fill(0x5c);
  for (let i = 0; i < k.length; i++) { ipad[i] ^= k[i]; opad[i] ^= k[i]; }
  return sha256(cat(opad, await sha256(cat(ipad, data))));
}

function equal(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a[i] ^ b[i];
  return d === 0;
}

const MAGIC = utf8('MS1');
export const OVERHEAD = MAGIC.length + 16 + 32;

export function hex(b: Uint8Array): string { return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); }
export function unhex(s: string): Uint8Array { const out = new Uint8Array(s.length / 2); for (let i = 0; i < out.length; i++) out[i] = parseInt(s.slice(2 * i, 2 * i + 2), 16); return out; }

async function derive(masterHex: string): Promise<{ enc: Uint8Array; mac: Uint8Array }> {
  const master = unhex(masterHex);
  return { enc: await sha256(cat(utf8('choir-mail-enc'), master)), mac: await sha256(cat(utf8('choir-mail-mac'), master)) };
}

/** A fresh 32-byte key, as 64 hex characters. */
export function newKey(): string { return hex(crypto.getRandomValues(new Uint8Array(32))); }
export const validKey = (k: unknown): k is string => typeof k === 'string' && /^[0-9a-f]{64}$/.test(k);

export async function seal(plain: Uint8Array, masterHex: string): Promise<Uint8Array> {
  const { enc, mac } = await derive(masterHex);
  const nonce = crypto.getRandomValues(new Uint8Array(16));
  const body = cat(MAGIC, nonce, await ctr(enc, nonce, plain));
  return cat(body, await hmacSha256(mac, body));
}

/** The plain bytes, or null when the data is not ours, was changed, or the key is another. */
export async function open(sealed: Uint8Array, masterHex: string): Promise<Uint8Array | null> {
  if (sealed.length < OVERHEAD) return null;
  for (let i = 0; i < MAGIC.length; i++) if (sealed[i] !== MAGIC[i]) return null;
  const { enc, mac } = await derive(masterHex);
  const body = sealed.subarray(0, sealed.length - 32);
  if (!equal(await hmacSha256(mac, body), sealed.subarray(sealed.length - 32))) return null;
  return ctr(enc, body.subarray(3, 19), body.subarray(19));
}
