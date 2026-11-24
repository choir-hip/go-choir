// What the person has agreed to on this phone, kept per account (app:/data/consent.json, wiped at sign-out with the rest, account.ts WIPES). There is ONE agreement here: downloading model files from a third-party host
// (Hugging Face), which the owner wants asked for with the sizes before any byte is fetched (owner, 2026-11-23). The agreement to outside AI companies handling a request is NOT here: it is given once, at account
// creation, on the create-account screen (lane native-passkey), and asking the radio needs an account (owner, 2026-11-23: "we dont need such a before the radio answers consent screen. thats not industry standard").
//   models  '' (never asked), 'agreed' (Wi-Fi only), 'agreed-mobile' (the person pressed Download on mobile data, having been told the size) or 'declined' (kept, so nothing asks again at launch; agree later in Settings).
//
// Failure list (each line has a test in consent.test.ts):
//  C1 A fresh account has agreed to nothing. A corrupt, empty or wrong-typed file is fresh, never a throw.
//  C2 Agreeing (Wi-Fi, mobile data) and declining persist; a decline can be agreed later; `modelsAgreed` is true for the two agreements only; unread is never yes.
//  C3 A sign-out removes the memory and the file; an account generation that moved while the file was read does not take that text.
//  C4 An op that is not one of the three changes nothing.
//  C5 The radio's mode is the desk only for a present key; signed out, locked is listening only; a key read that threw is the desk (the request itself still refuses with no key).
//  C6 The words of the sign-in prompt are strings/en.json's.
import type { Storage } from './app.contract.d.ts';
import { askText, type AskText } from './consent-text.ts';
import { accountGeneration, keyState, writeCurrent } from './api.ts';

export const CONSENT_FILE = 'app:/data/consent.json';
export type Consent = { models: string };
export type ConsentView = { models: string; mobileOk: boolean; rev: number } & AskText;
const fresh = (): Consent => ({ models: '' });

/** C1: the kept text, by shape. */
export function parseConsent(text: string | undefined): Consent {
  const c = fresh();
  if (!text) return c;
  try {
    const o = JSON.parse(text) as Record<string, unknown>;
    if (!o || typeof o !== 'object' || Array.isArray(o)) return c;
    if (o.models === 'agreed' || o.models === 'agreed-mobile' || o.models === 'declined') c.models = o.models;
  } catch { /* fresh */ }
  return c;
}

type Mem = { gen: number; c: Consent; loaded: boolean; loading?: Promise<void>; rev: number };
let mem: Mem | null = null;
/** For tests: a new process. */
export const forgetConsent = (): void => { mem = null; };

async function load(storage: Storage | undefined): Promise<Mem> {
  if (!mem || mem.gen !== accountGeneration()) mem = { gen: accountGeneration(), c: fresh(), loaded: false, rev: 0 };
  const m = mem;
  if (m.loaded) return m;
  if (!m.loading) {
    m.loading = (async () => {
      let text: string | undefined, failed = false;
      try { if (storage) text = new TextDecoder().decode(await storage.fs.readFile(CONSENT_FILE)); }
      catch (e) { if ((e as { code?: string } | null)?.code !== 'ENOENT') failed = true; }       // absent is fresh; any other failure is read again next time
      if (m.gen === accountGeneration() && !failed) { m.c = parseConsent(text); m.loaded = true; }
      m.loading = undefined;
    })();
  }
  await m.loading;
  if (m.gen !== accountGeneration()) return load(storage);                                         // C3
  return m;
}

const view = (m: Mem): ConsentView => ({ models: m.c.models, mobileOk: m.c.models === 'agreed-mobile', rev: m.rev, ...askText() });

/** Source: what has been agreed, and the words of the sign-in prompt. */
export async function consentView(storage: Storage | undefined): Promise<ConsentView> { return view(await load(storage)); }

/** Did the person agree to model downloads, and over mobile data too? Unread is no. */
export async function modelsAgreement(storage: Storage | undefined): Promise<{ ok: boolean; mobile: boolean }> {
  const m = await load(storage);
  return { ok: m.loaded && (m.c.models === 'agreed' || m.c.models === 'agreed-mobile'), mobile: m.loaded && m.c.models === 'agreed-mobile' };
}
export const modelsAgreed = async (storage: Storage | undefined): Promise<boolean> => (await modelsAgreement(storage)).ok;

/** The three presses. */
export async function consentAct(op: string, storage: Storage | undefined): Promise<ConsentView> {
  const m = await load(storage);
  if (!m.loaded) return view(m);                                                                   // never write over what was not read
  const before = m.c.models;
  switch (op) {
    case 'agreeModels': m.c.models = 'agreed'; break;
    case 'agreeModelsMobile': m.c.models = 'agreed-mobile'; break;
    case 'declineModels': m.c.models = 'declined'; break;
    default: return view(m);                                                                       // C4
  }
  if (m.c.models !== before) {
    m.rev += 1;
    if (storage) await writeCurrent(m.gen, storage, CONSENT_FILE, new TextEncoder().encode(JSON.stringify(m.c)));
  }
  return view(m);
}

/** Sign-out (account.ts WIPES): the memory and the file. Throws when the file could not be removed and is still there. */
export async function wipeConsent(storage: Storage | undefined): Promise<void> {
  mem = null;
  if (!storage) return;
  try { await storage.fs.rm(CONSENT_FILE); } catch (e) { try { await storage.fs.stat(CONSENT_FILE); } catch { return; } throw e; }
}

/** The radio writer's mode (radio-writer.ts `mode`): the desk only for a signed-in person; otherwise the radio plays the paper (listening only). A key read that fails is the desk (the request itself still refuses with no key). */
export async function deskMode(): Promise<'desk' | 'listening'> {
  const k = await keyState();
  if (k.error) return 'desk';
  return k.state === 'present' ? 'desk' : 'listening';
}
