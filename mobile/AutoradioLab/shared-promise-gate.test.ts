// The gate for one class of defect (docs/problems/shared-settle-promise-fails-every-request-2026-11-25.md): work shared between Exact answers by keeping a PROMISE where more than one answer can await it.
// Exact runs each step as the answer that started the work, refuses that answer's fetches, native calls and timers once it is let go, and drops its timers (what awaits them stays pending): one answer let
// go then fails or hangs every answer that awaits the kept promise. Share the resolved VALUE instead.
// Failure list: G1 every module-level binding, class field or type member of the shared modules that holds a Promise (or a Map or Set of them) is in ALLOWED below, with the kind of reason it is allowed;
// a new one fails here, by file, line and name. G2 every entry of ALLOWED still exists (a fixed site leaves the list).
import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';

// Kinds: `storage` (only storage steps: a let-go answer's storage steps run on, prelude.js `__exact_forget` "storage"), `unawaited` (no answer awaits it), `value` (resolve-only and raced with the
// waiter's own timer), `redo` (a read: a waiter whose shared read was refused reads itself, api.ts joinOrRedo), `residual` (NOT proven safe: named in the problem record, one failed attempt at worst).
const ALLOWED: Record<string, string> = {
  'account.ts:pendingQueue': 'storage: the pending-revoke file read-modify-writes',
  'account.ts:p': 'value: the settle wake, resolve-only, raced with the waiter\'s own timer (awaitSettled)',
  'api.ts:bootSent': 'unawaited: the boot announcement is sent first, not waited for; a refused one is sent again',
  'api.ts:p': 'redo: the computer discovery (a GET); a waiter whose shared discovery was refused discovers itself',
  'cards.ts:loading': 'storage: the told-the-paper file read',
  'cards.ts:saving': 'storage: the told-the-paper file writes',
  'clips.ts:asking': 'residual: the clip set GET; a refusal resets `asked`, but an answer waiting on it sees no clips once',
  'consent.ts:loading': 'storage: the consent file read (its native push, refused, is pushed again)',
  'content-run.ts:opening': 'storage: the content database open',
  'content-run.ts:p': 'residual: the content sync; a waiter on a refused run sees "offline" once (not kept)',
  'edition-picks.ts:loaded': 'storage: the picks file read',
  'edition-picks.ts:tail': 'storage: the picks file writes',
  'edition-picks.ts:mirrorTail': 'residual: the native copy pushes; a refused link reports "not sent" and is pushed again at the next open',
  'files.ts:resolved': 'residual: opening a file as Texture is a POST (not repeated); a waiter on a refused one gets one failure (not kept)',
  'imported-source.ts:reading': 'unawaited: docs() answers what is held; the read runs on its own',
  'onboarding.ts:loading': 'storage: the progress file read',
  'pages.ts:loading': 'storage: the open pages file read',
  'pages.ts:saving': 'storage: the open pages file writes',
  'paper.ts:kept': 'redo: the front list GET; a waiter whose shared list was refused asks itself',
  'podcast.ts:inflight': 'residual: a feed GET; a waiter on a refused load sees the feed it had (not kept)',
  'radio-bridge.ts:restored': 'residual: the radio (radio-pulse-rule.test.ts governs which radio sources may do work)',
  'radio-runtime.ts:chain': 'residual: the radio deck acts, run in the pumping answer (radio-pulse-rule.test.ts)',
  'radio-writer.ts:task': 'residual: the desk flight, awaited by the writing mutation only',
  'reader.ts:decoding': 'residual: one decode at a time; a link run by an answer let go fails that decode once',
  'reader.ts:inflight': 'redo: the story GET; a waiter whose shared fetch was refused fetches itself',
  'recent-pages.ts:loading': 'storage: the recent pages file read',
  'recent-pages.ts:saving': 'storage: the recent pages file writes',
  'retrieval-run.ts:loaded': 'storage: the vectors read',
  'retrieval-run.ts:running': 'unawaited: the indexing loops (awaited by tests only, waitIdle)',
  'retrieval-run.ts:answered': 'residual: one ask answered once; a waiter on a refused answer gets ok:false once (not kept)',
  'retrieval-run.ts:cold': 'value: raced with the waiter\'s own sleep (COLD_WAIT_MS)',
  'runsheet-store.ts:flushTask': 'residual: the run sheet post loop (radio)',
  'runsheet-store.ts:saving': 'storage: the run sheet file writes',
  'steer.ts:loading': 'storage: the steer file read',
  'steer.ts:flushing': 'residual: the steer POSTs; a waiter on a refused flush sees "offline" once, the next flush retries',
  'steer.ts:saving': 'storage: the steer file writes',
  'take-session.ts:queue': 'storage: the kept takes file read-modify-writes',
  'themes.ts:loading': 'storage: the themes files read',
  'themes.ts:saving': 'storage: the theme choice writes',
};

const DIR = new URL('.', import.meta.url).pathname;
const IDENT = /^[A-Za-z_$][\w$]*/;
/** Split at commas outside brackets. */
function topLevel(text: string): string[] {
  const out: string[] = []; let depth = 0, from = 0;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if ('([{<'.includes(c)) depth++;
    else if (')]}>'.includes(c) && !(c === '>' && text[i - 1] === '=')) depth--;
    else if (c === ',' && depth === 0) { out.push(text.slice(from, i)); from = i + 1; }
  }
  out.push(text.slice(from));
  return out;
}
const holdsPromise = (text: string): boolean => /\bPromise\b/.test(text);
const isFunctionValue = (init: string): boolean => /^\s*(async\b|function\b|\(|<[^>]*>\s*\()/.test(init);
/** `new Promise`, or a Map or Set whose elements are promises (not functions that return one). */
const makesPromises = (init: string): boolean => { if (/^\s*new\s+Promise\b/.test(init)) return true; const g = /^\s*new\s+(?:Map|Set)\s*<(.*)>\s*\(/.exec(init); return !!g && holdsPromise(g[1]) && !g[1].includes('=>'); };

type Hit = { key: string; where: string };
function scan(): Hit[] {
  const hits: Hit[] = [];
  const files = readdirSync(DIR).filter((f) => f.endsWith('.ts') && !f.endsWith('.test.ts') && !f.endsWith('.d.ts'));
  for (const f of files) {
    const lines = readFileSync(DIR + f, 'utf8').split('\n');
    lines.forEach((line, i) => {
      const where = `${f}:${i + 1}`;
      const code = line.replace(/\/\/.*$/, '');
      // module-level bindings
      const decl = /^(?:export\s+)?(?:let|var|const)\s+(.*)$/.exec(code);
      if (decl) {
        for (const d of topLevel(decl[1].replace(/;\s*$/, ''))) {
          const name = IDENT.exec(d.trim())?.[0]; if (!name) continue;
          const eq = d.indexOf('=');
          const typePart = eq >= 0 ? d.slice(0, eq) : d, init = eq >= 0 ? d.slice(eq + 1) : '';
          if (isFunctionValue(init)) continue;
          if (holdsPromise(typePart) || /^\s*Promise\./.test(init) || makesPromises(init)) {
            // a record type holding a promise: name the member too
            const member = /([A-Za-z_$][\w$]*)\??\s*:\s*Promise\b/.exec(typePart);
            hits.push({ key: `${f}:${member && member[1] !== name ? member[1] : name}`, where });
          }
        }
        return;
      }
      // one-line type literals: their members
      if (/^(?:export\s+)?(?:type|interface)\s/.test(code)) {
        for (const m of code.matchAll(/([A-Za-z_$][\w$]*)\??\s*:\s*([^;}]*)/g)) if (holdsPromise(m[2]) && !/[()]|=>/.test(m[2])) hits.push({ key: `${f}:${m[1]}`, where });
        return;
      }
      // class fields and members of multi-line types
      const field = /^\s+(?:(?:private|public|protected|readonly|static|declare)\s+)*([A-Za-z_$][\w$]*)\??!?\s*(?::\s*([^=;(]*?))?\s*(?:=\s*(.*))?;\s*$/.exec(code);
      if (field && !/^\s*(return|throw|await|yield|break|continue)\b/.test(code)) {
        const [, name, type = '', init = ''] = field;
        if (holdsPromise(type) || makesPromises(init) || (holdsPromise(init) && /Promise\.resolve\(\)\s*$/.test(init) && /^\s+(private|public|protected|readonly)/.test(code))) hits.push({ key: `${f}:${name}`, where });
      }
    });
  }
  return hits;
}

describe('no promise kept where several answers can await it', () => {
  const hits = scan();
  test('G1 every kept promise in the shared modules is on the allow-list, with its reason', () => {
    const unknown = hits.filter((h) => !(h.key in ALLOWED)).map((h) => `${h.where} ${h.key.split(':')[1]}`);
    expect(unknown).toEqual([]);
  });
  test('G2 every allow-list entry still exists', () => {
    const seen = new Set(hits.map((h) => h.key));
    expect(Object.keys(ALLOWED).filter((k) => !seen.has(k))).toEqual([]);
  });
  test('G3 the scan finds what it is for (a module promise, a record holding one, a Map of them, a class field)', () => {
    const keys = new Set(hits.map((h) => h.key));
    for (const k of ['pages.ts:loading', 'api.ts:p', 'paper.ts:kept', 'radio-runtime.ts:chain', 'edition-picks.ts:loaded']) expect(keys.has(k)).toBe(true);
  });
});
