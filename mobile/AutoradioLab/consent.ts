// What the person has agreed to on this phone, kept per account (app:/data/consent.json, wiped at sign-out with the rest, account.ts WIPES). The agreement here is to downloading model files from a third-party host (Hugging Face), asked for
// with the sizes before any byte is fetched (owner, 2026-11-23), and it is PER GROUP of the download step (the radio's voice, hearing you and the rest, search on this phone): a press on one group agrees to that group's files only, so the voice press
// can never unlock the search model's download (review, 2026-11-24). The agreement to outside AI companies handling a request is NOT here: it is given once, at account creation, on the create-account screen (lane native-passkey), and asking the radio needs an
// account (owner, 2026-11-23: "we dont need such a before the radio answers consent screen. thats not industry standard").
//   agreed  group -> 'agreed' (Wi-Fi only) or 'agreed-mobile' (the person pressed Download on mobile data, having been told the size). A Wi-Fi press never lowers a mobile agreement.
//   picks   job -> the model id the person chose for that job in the download step (absent: the list's recommendation for this phone).
//
// Failure list (each line has a test in consent.test.ts):
//  C1 A fresh account has agreed to nothing. A corrupt, empty or wrong-typed file is fresh, never a throw.
//  C2 Agreeing (Wi-Fi, mobile data) persists per group; agreement for one group is not agreement for another; unread is never yes; a Wi-Fi agreement never lowers a mobile one; an agreement that could not be written is not one.
//  C3 A sign-out removes the memory and the file; an account generation that moved while the file was read does not take that text.
//  C5 The radio's mode is the desk only for a present key; signed out, locked is listening only; a key read that threw is the desk (the request itself still refuses with no key).
//  C6 The words of the sign-in prompt are strings/en.json's.
//  C7 A pick of a model for a job (the download step) is kept across a restart, read by shape (only string ids), changes nothing when it is the same, and goes with the rest at sign-out.
import type { Storage } from './app.contract.d.ts';
import { askText, type AskText } from './consent-text.ts';
import { accountGeneration, keyState, writeCurrent } from './api.ts';
import { MIC_TEXT } from './micask.ts';

export const CONSENT_FILE = 'app:/data/consent.json';
export type Consent = { agreed: Record<string, string>; picks: Record<string, string> };
export type ConsentView = { rev: number } & AskText & { micPre: string; micOff: string; micContinue: string; micNotNow: string; micOk: string };
const fresh = (): Consent => ({ agreed: {}, picks: {} });

/** C1: the kept text, by shape. */
export function parseConsent(text: string | undefined): Consent {
  const c = fresh();
  if (!text) return c;
  try {
    const o = JSON.parse(text) as Record<string, unknown>;
    if (!o || typeof o !== 'object' || Array.isArray(o)) return c;
    if (o.agreed && typeof o.agreed === 'object' && !Array.isArray(o.agreed)) for (const [k, v] of Object.entries(o.agreed as Record<string, unknown>)) if (v === 'agreed' || v === 'agreed-mobile') c.agreed[k] = v;
    if (o.picks && typeof o.picks === 'object' && !Array.isArray(o.picks)) for (const [k, v] of Object.entries(o.picks as Record<string, unknown>)) if (typeof v === 'string' && v !== '') c.picks[k] = v;
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

const view = (m: Mem): ConsentView => ({ rev: m.rev, ...askText(), micPre: MIC_TEXT.pre, micOff: MIC_TEXT.off, micContinue: MIC_TEXT.continue, micNotNow: MIC_TEXT.notNow, micOk: MIC_TEXT.ok });

/** Source: what has been agreed, and the words of the sign-in prompt. */
export async function consentView(storage: Storage | undefined): Promise<ConsentView> { return view(await load(storage)); }

/** The person's picks, job -> model id (an unread file is no picks: the list's recommendations). */
export async function consentPicks(storage: Storage | undefined): Promise<Record<string, string>> { const m = await load(storage); return { ...m.c.picks }; }

/** Did the person agree to this group's downloads (`voice`, `voice-rest`, `search`), and over mobile data too? Unread is no. */
export async function modelsAgreement(storage: Storage | undefined, group = 'search'): Promise<{ ok: boolean; mobile: boolean }> {
  const m = await load(storage);
  const a = m.loaded ? m.c.agreed[group] : undefined;
  return { ok: a === 'agreed' || a === 'agreed-mobile', mobile: a === 'agreed-mobile' };
}

/** The press on a group's download button: record the agreement for that group, and say whether it is on disk (nothing may start when it is not). */
export async function consentAgree(group: string, mobile: boolean, storage: Storage | undefined): Promise<boolean> {
  const m = await load(storage);
  if (!m.loaded || group === '') return false;                                                    // never write over what was not read
  const next = mobile || m.c.agreed[group] === 'agreed-mobile' ? 'agreed-mobile' : 'agreed';
  if (m.c.agreed[group] === next) return true;
  const before = m.c.agreed[group];
  m.c.agreed[group] = next; m.rev += 1;
  if (storage && !(await writeCurrent(m.gen, storage, CONSENT_FILE, new TextEncoder().encode(JSON.stringify(m.c))))) { if (before === undefined) delete m.c.agreed[group]; else m.c.agreed[group] = before; return false; }
  return true;
}

/** The person's pick of one option for one job (the download step). Kept with the agreement; the caller has checked the job and option are on offer. A pick of the same thing changes nothing. */
export async function consentPick(job: string, id: string, storage: Storage | undefined): Promise<ConsentView> {
  const m = await load(storage);
  if (!m.loaded || job === '' || id === '' || m.c.picks[job] === id) return view(m);
  m.c.picks[job] = id; m.rev += 1;
  if (storage) await writeCurrent(m.gen, storage, CONSENT_FILE, new TextEncoder().encode(JSON.stringify(m.c)));
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
