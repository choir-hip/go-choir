// The seal on the phone's copy of the listener's mail. The cipher is the PLATFORM's, not ours: one native pair, `mailSeal` and `mailOpen`, on the
// app module (modules/apple/ModelsEntry.swift, Choir/MailSeal.swift): CryptoKit AES-GCM, with a key made and kept natively in the Keychain
// (this device only, after first unlock) that is never returned to shared code. Exact's `crypto.subtle` has digests and ECDSA only and refuses AES
// (docs/reference.md, data-module table), so a platform cipher is the one thing shared code cannot do; hand-made cipher code does not ship.
// Android owes the same pair in Kotlin (Keystore AES-GCM).
//
// No cipher, nothing written: when the native module is absent or refuses (no key could be made), `seal` answers null and mail-store writes no file.
// Format: "MS3" | the native sealed box (12-byte nonce, ciphertext, 16-byte tag), the whole as AES-GCM's combined form.

import { nativeLater, sealingBlocked, refreshSealBlock } from './api.ts';

export async function sha256(data: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await crypto.subtle.digest('SHA-256', data as unknown as BufferSource));
}
export function hex(b: Uint8Array): string { return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); }

// MS3: the record's identity (`aad`: kind and id) is authenticated with the data. An MS2 file of the earlier build does not open ("bad") and is removed: the
// server's copy replaces it at the next sync, nothing is lost.
const MAGIC = new TextEncoder().encode('MS3');

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

let sealable: boolean | null = null;
/** Is there a native cipher, learned from ONE probe (`mailCan`) per session: a module that lacks the pair (Android today) answers no, so nothing is fetched
 * to be thrown away. False while a sign-out is under way, until a new session is established (api.ts `sealingBlocked`). */
/** A native call that never answers must not hang a source: a deadline, not a limit. */
const DEADLINE_MS = 15000;
const bounded = <T>(p: Promise<T>): Promise<T> => Promise.race([p, new Promise<T>((_, rej) => setTimeout(() => rej(new Error('no answer')), DEADLINE_MS))]);
export async function canSeal(): Promise<boolean> {
  await refreshSealBlock();
  if (sealingBlocked()) return false;
  const later = nativeLater(); if (!later) return false;
  if (sealable === null) {
    // Only an ANSWER is kept for the session: a call that threw, timed out or was refused (its answer let go) says nothing about the cipher, and is asked again next time.
    try { const r = (await bounded(later({ op: 'mailCan' }))) as { ok?: boolean } | null; sealable = !!r && r.ok === true; } catch { return false; }
  }
  return sealable;
}
/** Forget what the probe learned (sign-out, tests). */
export function forgetProbe(): void { sealable = null; }

/** The sealed bytes, or null when there is no cipher or it refused. `aad` names the record (kind and id). */
export async function seal(plain: Uint8Array, aad: string): Promise<Uint8Array | null> {
  if (sealingBlocked()) return null;
  const later = nativeLater(); if (!later) return null;
  try {
    const r = (await bounded(later({ op: 'mailSeal', b64: b64(plain), aad }))) as { ok?: boolean; b64?: unknown } | null;
    if (!r || r.ok !== true || typeof r.b64 !== 'string') return null;
    const box = unb64(r.b64); const out = new Uint8Array(MAGIC.length + box.length);
    out.set(MAGIC); out.set(box, MAGIC.length);
    return out;
  } catch { return null; }
}

/** `plain`: opened. `bad`: the data is not ours, was changed, or is another record's (the tag failed): it may be removed. `cannot`: nothing could be said
 * now (no native module, the Keychain locked, a bridge error): the file is KEPT. */
export type Opened = { plain: Uint8Array } | { why: 'bad' | 'cannot' };
export async function open(sealed: Uint8Array, aad: string): Promise<Opened> {
  const later = nativeLater(); if (!later) return { why: 'cannot' };
  if (sealed.length < MAGIC.length + 28) return { why: 'bad' };
  for (let i = 0; i < MAGIC.length; i++) if (sealed[i] !== MAGIC[i]) return { why: 'bad' };
  try {
    const r = (await bounded(later({ op: 'mailOpen', b64: b64(sealed.subarray(MAGIC.length)), aad }))) as { ok?: boolean; b64?: unknown; why?: unknown } | null;
    if (r && r.ok === true && typeof r.b64 === 'string') return { plain: unb64(r.b64) };
    return { why: r && r.why === 'bad' ? 'bad' : 'cannot' };
  } catch { return { why: 'cannot' }; }
}

/** Sign-out: forget the key natively (the files are then unreadable whatever happens to them). Never throws. */
export async function forgetKey(): Promise<void> {
  const later = nativeLater(); if (!later) return;
  forgetProbe();
  try { await later({ op: 'mailForget' }); } catch { /* nothing to forget */ }
}
