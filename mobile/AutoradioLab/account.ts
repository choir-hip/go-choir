// THE ACCOUNT BOUNDARY (docs/design/account-boundary-2026-11-21.md, "Accounts are partitions" and "Round 12"; docs/problems/account-boundary-nine-review-rounds-structural-assessment-2026-11-22.md). A process belongs to ONE account for its whole life.
// THE KEY IS EXACT'S (`secret.keep`), so the launch verdict is made HERE, once, from the store's key (api.ts `adoptStore`), two markers under `app:/data` and the sweep. Native holds no verdict.
//   LAUNCH   a pending sign-out (the ending mark) is finished first (revoke, forget the secrets, sweep every root, native forgets its own files); then the old pre-partition leftovers and orphan roots are swept; only a finished sweep lets a world open: the key's, or nobody's (anonymous).
//            No key but the marker says an account was signed in on this install: STARTING (the secret has not read yet, before the first unlock; Exact asks this answer again when its retry completes); after `LOST_AFTER` asks, anonymous and Sign in is offered.
//   T1       a key held while the world is anonymous (a sign-in stored it, or a late read found it): the anonymous world moves to the account's, once.
//   T2       SIGN-OUT ends the session: native drops its in-memory key, the ending mark is written, the key is revoked then forgotten; no world opens again. The card says "close and reopen"; the next launch wipes first.
import type { Storage, Store } from './app.contract.d.ts';
import { ACCOUNTS, adoptStore, currentWorld, dropHeld, endWorld, forgetSession, heldSession, keyDigest, openWorld, sessionEnded, nativeLater, revokeKey, withDeadline, rmTree, rejectedDigest, deviceStorage, markNotBaked, type World } from './api.ts';

/** A page answer refused because the account is not open (locked, or the session ended). The page shows its existing failure state; the card says why. */
export class AccountClosedError extends Error {
  state: string;
  constructor(state: string) { super(`the account boundary is closed (${state})`); this.name = 'AccountClosedError'; this.state = state; }
}

/** The old top-level paths of every account store (before the roots), removed at launch and never read: the owner's ruling that nothing is carried over from earlier builds. A test (account.test.ts L2) compares them with every
 * `app:/` literal the shared modules use, so a new store that is not re-rooted or listed as a device path fails there. `LEFTOVER_PREFIXES` is a folder and the file-name prefix its leftovers begin with. */
export const LEFTOVERS: string[] = [
  'app:/data/mail', 'app:/data/recent-files.json', 'app:/data/paper-front.json', 'app:/data/podcasts-library.json', 'app:/data/podcast-feeds', 'app:/data/takes.json', 'app:/data/notebook-cache.md',
  'app:/data/vectors', 'app:/data/content.db', 'app:/data/content.db-wal', 'app:/data/content.db-shm', 'app:/data/open-pages-shared.json', 'app:/data/demo-leftovers-cleared', 'app:/data/theme-textures',
  'app:/data/account-stamp', 'app:/data/signout-pending', 'app:/data/what-i-told-the-paper.json', 'app:/data/recent-pages.json', 'app:/data/steer-queue.json', 'app:/data/runsheet.json',
  'app:/data/imported-places.json', 'app:/data/edition-topics.json', 'app:/data/onboarding.json', 'app:/data/exports', 'app:/cache/files',
];
export const LEFTOVER_PREFIXES: [string, string][] = [['app:/cache', 'mail-']];
const SPACES = ['app:/data', 'app:/cache', 'app:/tmp'];

/** Deadlines, not limits. `nativeMs`: the native sign-out (it waits up to five seconds for the run sheet and stops the audio). Tests shorten it. */
export const timing = { nativeMs: 30000, hostMs: 1500, wipeMs: 8000 };

const codeOf = (e: unknown): string => String((e as { code?: unknown } | null)?.code ?? '');
const readdir = async (storage: Storage, p: string): Promise<string[]> => { try { return await storage.fs.readdir(p); } catch (e) { if (codeOf(e) === 'ENOENT') return []; throw e; } };

/** Delete every root no live key names (`anonymous` stays) and the leftovers. Never reads a file. A failure on one path leaves it for the next launch. `live`: the digest of the live key, or '' for none. */
export async function sweep(storage: Storage, live: string): Promise<{ removed: number; failed: number }> {
  let removed = 0; let failed = 0; const listed = { data: 0, cache: 0, tmp: 0 }; const why: string[] = [];
  const rm = async (p: string): Promise<void> => { try { await rmTree(storage, p); removed += 1; } catch (e) { failed += 1; const c = codeOf(e) || 'error'; if (!why.includes(c)) why.push(c); } };
  for (const space of SPACES) {
    let names: string[] = [];
    try { names = await readdir(storage, `${space}/${ACCOUNTS}`); } catch { failed += 1; continue; }
    listed[space === 'app:/data' ? 'data' : space === 'app:/cache' ? 'cache' : 'tmp'] += names.length;
    for (const n of names) if (n !== 'anonymous' && n !== live) await rm(`${space}/${ACCOUNTS}/${n}`);
  }
  for (const p of LEFTOVERS) {
    let there = true; try { await storage.fs.stat(p); } catch (e) { there = codeOf(e) !== 'ENOENT'; }       // only what is there: a removal of nothing is not counted
    if (there) await rm(p);
  }
  if (!live) { let there = true; try { await storage.fs.stat('app:/tmp/picked'); } catch (e) { there = codeOf(e) !== 'ENOENT'; } if (there) await rm('app:/tmp/picked'); }      // signed out: a person's picked documents do not outlive the sign-out (a signed-in launch keeps them)
  for (const [dir, prefix] of LEFTOVER_PREFIXES) { let names: string[] = []; try { names = await readdir(storage, dir); } catch { failed += 1; continue; } for (const n of names) if (n.startsWith(prefix)) await rm(`${dir}/${n}`); }
  console.log(`account: sweep listed ${listed.data}/${listed.cache}/${listed.tmp} roots (data/cache/tmp), kept ${live ? 'the open account and anonymous' : 'anonymous'}, removed ${removed} paths, ${failed} failed${why.length ? ` (${why.join(', ')})` : ''}`);       // counts and reason words only, never a name
  return { removed, failed };
}

// ---- the process's boundary state. It holds no account data: fixed words, and a promise that settles when the launch has run.
export type Ended = '' | 'ending' | 'signedout' | 'unfinished' | 'blocked';
/** Why the process is LOCKED: the key has not read yet (the marker says an account was signed in) or a marker cannot be read. Fixed words, no account data. */
export type LockedWhy = '' | 'starting';
/** THE ENDING MARK (Exact's `app:/data/ending`): written by a sign-out before the key is deleted, removed last by the launch that has finished everything. */
export const ENDING = 'app:/data/ending';
/** THE SIGNED-IN MARKER (`app:/data/signed-in`): "an account was signed in on this install", written by a sign-in and removed by the launch that finishes a sign-out. It exists only to tell a secret that is absent from one that has not read yet. */
export const SIGNED = 'app:/data/signed-in';
/** The card's re-reads (three seconds apart, `tries` of account-state.ts) ran out with the marker set and the secret still not reading: from then on the launch is anonymous (the key was lost: a restored backup, a wiped Keychain) and Sign in is offered. A bound in time, not in answers. */
let lost = false;
let needForget = false;      // a turn that has the store must forget the three secrets (see `turn`)
let forgotten = false;       // ... and one did
let forgetFailed = false;
let revoked = false;         // the pending sign-out's key was revoked (or could not be) once
let lastWhy = '';      // log on change only: fixed words, never account data
const step = (w: string): void => { console.log(`account: ${w}`); };      // a step reached, in fixed words (the next drill run names where a sign-out or a launch stopped)
const why = (w: string): void => { if (w !== lastWhy) { lastWhy = w; console.log(`account: ${w}`); } };
export const declareLost = (): void => { lost = true; };
let endedWhy: Ended = '';
type Launch = { promise: Promise<'ready' | 'locked'>; resolve: (r: 'ready' | 'locked') => void; done: boolean };
let launching: Launch | null = null;      // the one launch of this process: ONE shared result, but any number of attempts (see `launch`)
let lockedWhy: LockedWhy = '';
let finishP: Promise<boolean> | null = null;
let markerWritten = false;
let swept = false;
let chain: Promise<unknown> = Promise.resolve();

/** The one chain: a sign-out is the person's act and always runs, one at a time. */
export function job<T>(fn: () => Promise<T>): Promise<T | undefined> {
  const run = async (): Promise<T | undefined> => fn();
  const p = chain.then(run, run);
  chain = p.then(() => undefined, () => undefined);
  return p;
}
export async function idle(): Promise<void> { for (;;) { const c = chain; await c; if (c === chain) return; } }
/** For tests: a process that has just started. */
export function resetBoundary(): void { endedWhy = ''; launching = null; chain = Promise.resolve(); timing.nativeMs = 30000; timing.wipeMs = 8000; swept = false; lockedWhy = ''; markerWritten = false; finishP = null; lost = false; lastWhy = ''; needForget = false; forgotten = false; forgetFailed = false; revoked = false; pendingRevoke = null; }

type Mark = 'present' | 'absent' | 'unreadable';
/** One folder listing answers both markers (metadata only: a marker's content is never read). A listing that fails for another reason than a missing folder is unreadable. */
async function readMarks(storage: Storage): Promise<{ ending: Mark; signed: Mark }> {
  try { const names = await storage.fs.readdir('app:/data'); return { ending: names.includes('ending') ? 'present' : 'absent', signed: names.includes('signed-in') ? 'present' : 'absent' }; }
  catch (e) { const m: Mark = codeOf(e) === 'ENOENT' ? 'absent' : 'unreadable'; return { ending: m, signed: m }; }
}
async function readMark(storage: Storage, path: string): Promise<Mark> { const m = await readMarks(storage); return path === ENDING ? m.ending : m.signed; }
async function rmMark(storage: Storage, path: string): Promise<boolean> { try { await deviceStorage(storage).fs.rm(path); } catch (e) { if (codeOf(e) !== 'ENOENT') return false; } return (await readMark(storage, path)) === 'absent'; }
export async function writeSigned(storage: Storage): Promise<boolean> { try { await deviceStorage(storage).fs.atomicWriteFile(SIGNED, new TextEncoder().encode('1')); return true; } catch { return false; } }
async function writeEnding(storage: Storage): Promise<boolean> { try { await deviceStorage(storage).fs.atomicWriteFile(ENDING, new TextEncoder().encode('ending')); return true; } catch { return false; } }

/** The few native calls left (docs/design, "Round 12"): `holdKey` (the key in memory for the Swift clients that still call the server: pages still in Swift, deleted with the last of them) and `forgetAccountFiles` (deletion only). Bounded. */
async function native(request: { op: string; [k: string]: unknown }, ms: number): Promise<unknown> {
  const later = nativeLater(); if (!later) return null;
  try { return await withDeadline(later(request), null, ms); } catch { return null; }
}
/** THE STORE EXISTS ONLY IN THE SYNCHRONOUS FIRST TURN OF AN ANSWER (and the continuation of an effectful request): Exact's `store.get`, `store.set` and `store.forget` throw "no store here, in the continuation of an answer that was let go" after any other await
 * (js/src/door.rs:45-70 `between_answers`, the message at :366). So every answer that is handed the store calls `turn(store)` as its FIRST statement, before any await: it reads the key (api.ts `adoptStore`) and performs the forget a
 * previous turn asked for (`needForget`, set after the revoke that must come first). Nothing else in this file touches the store. */
export function turn(store?: Store): void {
  adoptStore(store);
  if (needForget && store) {
    needForget = false;
    try { forgetSession(store); forgotten = true; step('secrets forgotten (in a turn that had the store)'); } catch { forgetFailed = true; endedWhy = endedWhy === 'ending' || endedWhy === 'signedout' || endedWhy === 'unfinished' ? 'unfinished' : 'blocked'; if (!sessionEnded()) endWorld(); }
  }
}
export const handKey = (): Promise<unknown> => { const h = heldSession(); return native({ op: 'holdKey', key: h?.key ?? '', keyId: h?.keyId ?? '', computerId: h?.computerId ?? '' }, timing.hostMs); };

/** FINISH a sign-out the last process began (the ending mark is there). `wait`: the key was revoked (if it was held) and a turn that has the store is asked to forget the secrets (`turn`); `done`: everything is gone and both markers are removed (the ending mark last); `blocked`: a step failed and the log
 * says which. Native's own files go before the sweep: if native cannot, nothing is swept and the next launch tries again. */
async function finishEnding(storage: Storage): Promise<'wait' | 'done' | 'blocked'> {
  if (forgetFailed) { why('blocked: the store refused to forget the secrets'); return 'blocked'; }
  if (!forgotten) {
    const h = heldSession();
    if (h && !revoked) { revoked = true; const ok = await revokeKey(h.keyId, h.key); step(`launch: revoke ${ok ? 'answered' : 'not made (no key id, offline or refused)'}`); }
    needForget = true; return 'wait';
  }
  const f = (await native({ op: 'forgetAccountFiles' }, timing.wipeMs)) as { done?: unknown } | null;
  step(`launch: native forgot its files: ${nativeLater() ? String(f?.done === true) : 'no native module'}`);
  if (nativeLater() && !(f && f.done === true)) { why('blocked: native could not forget its files'); return 'blocked'; }
  try { const r = await sweep(storage, ''); if (r.failed > 0) { why('blocked: the sweep could not remove every root'); return 'blocked'; } swept = true; } catch { why('blocked: the sweep failed'); return 'blocked'; }
  if (!(await rmMark(storage, SIGNED))) { why('blocked: the signed-in marker could not be removed'); return 'blocked'; }
  const ended = await rmMark(storage, ENDING); step(`launch: the ending mark ${ended ? 'removed, the sign-out is finished' : 'could not be removed'}`); if (!ended) why('blocked: the ending mark could not be removed');
  return ended ? 'done' : 'blocked';
}

/** THE LAUNCH, once per process (see the header); `locked` is asked again at the next answer. The result is shared, but the storage calls of an attempt belong to the ANSWER that made them: the host drops them with the answer (the card is asked again the moment the sign-in view changes, and its first answer is gone), and an attempt whose answer is gone never finishes. So the card (`restart`) starts a fresh attempt, with its own storage, while the shared result is still open; the first attempt to finish settles it and the others stand down. */
function launch(storage: Storage | undefined, store: Store | undefined, restart = false): Promise<'ready' | 'locked'> {
  let d = launching;
  if (d && (d.done || !restart)) return d.promise;
  if (!d) { let res!: (r: 'ready' | 'locked') => void; const promise = new Promise<'ready' | 'locked'>((r) => { res = r; }); const made: Launch = { promise, done: false, resolve: (r) => { if (!made.done) { made.done = true; res(r); } } }; d = made; launching = d; }
  const mine = d;
  runLaunch(mine, storage, store).then((r) => { mine.resolve(r); if (r === 'locked' && launching === mine && !sessionEnded()) launching = null; }, () => { mine.resolve('locked'); if (launching === mine) launching = null; });
  return mine.promise;
}
async function runLaunch(d: Launch, storage: Storage | undefined, store: Store | undefined): Promise<'ready' | 'locked'> {
  {
    if (!storage || !store) { why(`starting: ${storage ? '' : 'no storage'} ${store ? '' : 'no store'}`.trim()); lockedWhy = 'starting'; return 'locked'; }
    markNotBaked();
    const marks = await readMarks(storage); const ending = marks.ending;
    if (ending === 'unreadable') { why('starting: the ending mark cannot be read'); lockedWhy = 'starting'; return 'locked'; }
    if (ending === 'present') {
      const r = await finishEnding(storage);
      if (r === 'wait') { why('starting: finishing a sign-out'); lockedWhy = 'starting'; return 'locked'; }
      if (r === 'blocked') { endedWhy = 'blocked'; endWorld(); return 'locked'; }
      dropHeld(); marks.signed = 'absent';      // the launch that finished it removed both markers
    }
    let live = ''; const h = heldSession(); if (h) live = await keyDigest(h.key);
    if (!live) {
      const signed = marks.signed;
      if (signed === 'unreadable' || (signed === 'present' && !lost)) { why(`starting: the signed-in marker is ${signed} and no key reads`); lockedWhy = 'starting'; return 'locked'; }
    }
    lockedWhy = '';
    if (d.done) return 'ready';      // another attempt finished first
    if (!(await sweepOnce(storage, live))) { endedWhy = 'blocked'; endWorld(); return 'locked'; }
    if (sessionEnded() || d.done) return 'ready';
    if (currentWorld().kind !== 'none') return 'ready';      // another attempt opened it
    const opened = openWorld(live || 'anonymous');
    for (const space of SPACES) { for (const p of [`${space}/${ACCOUNTS}`, `${space}/${ACCOUNTS}/${opened.root}`]) { try { await storage.fs.mkdir(p); } catch { /* it exists */ } } }      // the first write of a module (onboarding) finds its root: a host's files need the parent folder
    await native({ op: 'ready' }, timing.hostMs);      // the launch is clean: native builds its module (a blocked launch never says it)
    if (live) await handKey();
    return 'ready';
  }
}
async function sweepOnce(storage: Storage, live: string): Promise<boolean> {
  if (swept) return true;
  try { const r = await sweep(storage, live); if (r.failed === 0) swept = true; return r.failed === 0; } catch { console.log('account: the sweep failed'); return false; }
}

export type State = 'open' | 'anonymous' | 'locked' | 'ended';

/** The state, for the gate and the card: launch if it has not run; a key held while the world is anonymous (a sign-in stored it, or a late read found it) moves the anonymous world to the account's, once (T1). */
async function settle(storage: Storage | undefined, store: Store | undefined, restart = false): Promise<{ state: State; world: World; digest: string }> {
  if (sessionEnded()) return { state: 'ended', world: currentWorld(), digest: '' };
  const launched = await launch(storage, store, restart);
  if (sessionEnded()) return { state: 'ended', world: currentWorld(), digest: '' };
  if (launched === 'locked') return { state: 'locked', world: currentWorld(), digest: '' };
  const w = currentWorld();
  if (w.kind === 'anonymous') {
    const h = heldSession();
    if (h && !sessionEnded()) { const d = await keyDigest(h.key); if (!sessionEnded() && currentWorld() === w) { openWorld(d); await handKey(); } }
  }
  const now = currentWorld();
  return { state: now.kind === 'account' ? 'open' : 'anonymous', world: now, digest: now.digest ?? '' };
}

/** The gate's one question, asked before every page answer that touches account data. OPEN and ANONYMOUS pass; LOCKED and ENDED refuse. */
export async function enter(storage: Storage | undefined, store?: Store): Promise<{ readable: boolean; state: State; world: World }> {
  turn(store);
  const s = await settle(storage, store);
  if (s.state === 'locked' || s.state === 'ended') why(`the gate refuses: ${s.state}`);
  return { readable: s.state === 'open' || s.state === 'anonymous', state: s.state, world: s.world };
}

export type Peek = { state: State; handle: string; ended: Ended; why: LockedWhy; rejected: boolean };
/** The card's question: never waits on a job. */
export async function peek(storage: Storage | undefined, store?: Store, tries = 0): Promise<Peek> {
  turn(store);
  if (tries >= 5 && !sessionEnded()) { const was = lost; lost = true; if (!was && lockedWhy === 'starting') launching = null; }      // the bound (account-state.ts TRIES): a launch that is still starting becomes anonymous
  const s = await settle(storage, store, true);
  const w = currentWorld();
  return { state: s.state, handle: s.state === 'ended' || s.state === 'locked' ? '' : w.handle, ended: endedWhy, why: s.state === 'locked' ? lockedWhy || 'starting' : '', rejected: s.state === 'open' && rejectedDigest() !== '' && rejectedDigest() === w.digest };
}

// ---- sign-out

export type SignOutResult = { done: boolean; failed: string; message: string; epoch: number };
const WORDS_UNFINISHED = 'Sign-out did not finish. Close Autoradio, open it again and try again.';

/** STEP ONE of a sign-out, quick and with its own source so the card draws at once: the session ends (the world closes, no world opens again), native is told to DROP its in-memory key, and only then the ending mark is written. Idempotent. A mark that cannot be written stops the sign-out before the key is deleted. */
export async function accountEnding(storage: Storage | undefined, store?: Store): Promise<SignOutResult> {
  turn(store);
  if (!sessionEnded()) { endWorld(); endedWhy = 'ending'; step('sign-out: the session ended'); }
  const h = heldSession(); const id = h?.keyId ?? '', key = h?.key ?? '';
  pendingRevoke = pendingRevoke ?? (h ? { id, key } : null);
  dropHeld();
  await native({ op: 'holdKey', key: '', keyId: '', computerId: '' }, timing.hostMs);
  await native({ op: 'radioSet', signedOut: true }, timing.hostMs);      // the session has ended: the run sheet has nothing more to post, so native need not wait for it
  if (!markerWritten) { markerWritten = storage ? await writeEnding(storage) : false; step(`sign-out: the ending mark ${markerWritten ? 'written' : 'could not be written'}`); }
  if (!markerWritten && endedWhy === 'ending') endedWhy = 'unfinished';
  return { done: markerWritten, failed: '', message: '', epoch: 0 };
}
let pendingRevoke: { id: string; key: string } | null = null;

/** STEP TWO, the person's tap, never behind the gate: revoke the key on the server, then forget the three secrets. NOTHING is wiped in this process (late writers may write what they like): the next launch finishes it. The result follows from the secrets, not from a guess: a forget that threw leaves the mark and the card says it did not finish. */
export async function accountSignOut(storage: Storage | undefined, store?: Store): Promise<SignOutResult> {
  turn(store);
  const r = (await job(() => signOutBody(storage, store))) ?? { done: false, failed: '', message: 'Nothing was signed out.' };
  return { ...r, epoch: 0 };
}
function finishOnce(): Promise<boolean> {
  return (finishP ??= (async () => {
    const p = pendingRevoke; pendingRevoke = null;
    if (p && p.key) { const ok = await revokeKey(p.id, p.key); step(`sign-out: revoke ${ok ? 'answered' : 'not made (no key id, offline or refused)'}`); }
    needForget = true; step('sign-out: the secrets are forgotten by the next turn that has the store');      // not here: the store is gone after the awaits above
    return true;
  })().finally(() => { finishP = null; }));
}
async function signOutBody(storage: Storage | undefined, store: Store | undefined): Promise<Omit<SignOutResult, 'epoch'>> {
  if (sessionEnded() && endedWhy === 'signedout') return { done: true, failed: '', message: '' };
  await accountEnding(storage, store);
  if (!markerWritten) return { done: false, failed: '', message: WORDS_UNFINISHED };
  if (!(await finishOnce())) { endedWhy = 'unfinished'; return { done: false, failed: '', message: WORDS_UNFINISHED }; }
  endedWhy = 'signedout'; return { done: true, failed: '', message: '' };
}
