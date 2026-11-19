// The seal on the phone's copy of the listener's mail. The cipher is the PLATFORM's, not ours: one native pair, `mailSeal` and `mailOpen`, on the
// app module (modules/apple/ModelsEntry.swift, Choir/MailSeal.swift): CryptoKit AES-GCM, with a key made and kept natively in the Keychain
// (this device only, after first unlock) that is never returned to shared code. Exact's `crypto.subtle` has digests and ECDSA only and refuses AES
// (docs/reference.md, data-module table), so a platform cipher is the one thing shared code cannot do; hand-made cipher code does not ship.
// Android owes the same pair in Kotlin (Keystore AES-GCM).
//
// No cipher, nothing written: when the native module is absent or refuses (no key could be made), `seal` answers null and mail-store writes no file.
// Format: "MS2" | the native sealed box (12-byte nonce, ciphertext, 16-byte tag), the whole as AES-GCM's combined form.

import { nativeLater } from './api.ts';

export async function sha256(data: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await crypto.subtle.digest('SHA-256', data as unknown as BufferSource));
}
export function hex(b: Uint8Array): string { return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); }

const MAGIC = new TextEncoder().encode('MS2');

/** Bytes as base64 for the native call (the bridge is JSON). In slices, so a long file does not build one huge argument list. */
function b64(bytes: Uint8Array): string {
  let s = '';
  for (let i = 0; i < bytes.length; i += 8192) s += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(s);
}
function unb64(text: string): Uint8Array {
  const s = atob(text); const out = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
  return out;
}

/** Is there a native cipher at all (so a write is worth trying)? */
export const canSeal = (): boolean => nativeLater() !== null;

/** The sealed bytes, or null when there is no cipher or it refused. */
export async function seal(plain: Uint8Array): Promise<Uint8Array | null> {
  const later = nativeLater(); if (!later) return null;
  try {
    const r = (await later({ op: 'mailSeal', b64: b64(plain) })) as { ok?: boolean; b64?: unknown } | null;
    if (!r || r.ok !== true || typeof r.b64 !== 'string') return null;
    const box = unb64(r.b64); const out = new Uint8Array(MAGIC.length + box.length);
    out.set(MAGIC); out.set(box, MAGIC.length);
    return out;
  } catch { return null; }
}

/** The plain bytes, or null when the data is not ours, was changed, or the key is another or gone. */
export async function open(sealed: Uint8Array): Promise<Uint8Array | null> {
  const later = nativeLater(); if (!later) return null;
  if (sealed.length < MAGIC.length + 28) return null;
  for (let i = 0; i < MAGIC.length; i++) if (sealed[i] !== MAGIC[i]) return null;
  try {
    const r = (await later({ op: 'mailOpen', b64: b64(sealed.subarray(MAGIC.length)) })) as { ok?: boolean; b64?: unknown } | null;
    return r && r.ok === true && typeof r.b64 === 'string' ? unb64(r.b64) : null;
  } catch { return null; }
}

/** Sign-out: forget the key natively (the files are then unreadable whatever happens to them). Never throws. */
export async function forgetKey(): Promise<void> {
  const later = nativeLater(); if (!later) return;
  try { await later({ op: 'mailForget' }); } catch { /* nothing to forget */ }
}
