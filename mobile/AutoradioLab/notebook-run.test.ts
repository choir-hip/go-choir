// The notebook page's state in shared code (notebook-run.ts), under bun test. What the Swift NotebookStore did for the page (refresh, refreshIfStale, select, the
// steer count, the notebook text kept for the DJ) and the one narrow native call. Failure list, written before the code:
//  R1 a select (same arguments, a new `selected`) answers from what was read: no request at all; the same id again clears it
//  R2 a new tick reads the computer again (the notebook and the scorecard, once each); the same tick does not
//  R3 a page that is not on top reads nothing and answers the idle page; coming back to the top reads again (the Swift page asked once a minute and on open)
//  R4 the native fact: the steer taps waiting are shown; with no native module there is no line; a native that fails or never answers is 0, never a stuck page
//  R5 the notebook text read is handed to native once per change (the DJ's request is built natively and cannot read app:/data), never when there is none
//  R6 a sign-out (a new signedOutTick) reads again, and a read in flight for the old key never answers the new one
//  R7 the native call is asked to write the topics on a read (Swift writeTopics: the Texture write stays native), before the notebook is read
import { enqueue as steerEnqueue, forget as steerForget } from './steer.ts';
import { beforeEach, describe, expect, test } from 'bun:test';
import { notebookShared, forgetNotebook, selectedAfter, type Device } from './notebook-run.ts';
import type { Get } from './notebook.ts';
import { HTTPError } from './api.ts';
import notebookMd from '../fixtures/learning/notebook.md' with { type: 'text' };
import learning from '../fixtures/learning/learning.json';

const files = new Map<string, Uint8Array>();
const storage = { fs: { readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, writeFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;

let calls: string[] = [];
const get: Get = async (p) => {
  calls.push(p);
  if (p === '/api/autoradio/learning') return learning;
  if (p === '/api/texture/edition') return { found: true, doc_id: 'nb1' };
  if (p === '/api/texture/documents/nb1') return { document: { current_revision_id: 'r1' } };
  if (p === '/api/texture/revisions/r1') return { revision: { content: notebookMd } };
  throw new HTTPError(404, 'HTTP 404');
};
let steer = 0, order: string[] = [];
const device: Device = { state: async () => { order.push('native'); } };
const env = (d: Device | null = device) => ({ get, device: d });
const args = (o: Partial<{ selected: string; tick: number; active: boolean; signedOutTick: string }> = {}) => ({ selected: '', tick: 0, active: true, signedOutTick: '', ...o });

beforeEach(() => { steerForget(); files.clear(); calls = []; steer = 0; order = []; forgetNotebook(); });

describe('R1 to R3 the page state', () => {
  test('R1 a select answers from what was read, and the same id again clears it', async () => {
    const a = await notebookShared(args(), storage, env());
    expect(a).toMatchObject({ docState: 'ready', scoreState: 'ready' });
    const n = calls.length;
    const id = a.sections[1].lines[0].id;
    const b = await notebookShared(args({ selected: id }), storage, env());
    expect(calls.length).toBe(n);
    expect(b.selectedText).toBe(a.sections[1].lines[0].text); expect(b.evidence.length).toBe(3);
    expect(selectedAfter(id, id)).toBe(''); expect(selectedAfter('', id)).toBe(id); expect(selectedAfter('other', id)).toBe(id);
    expect((await notebookShared(args({ selected: selectedAfter(id, id) }), storage, env())).selectedText).toBe('');
  });
  test('R2 a new tick reads again, once for the notebook and once for the scorecard; the same tick does not', async () => {
    await notebookShared(args(), storage, env());
    expect(calls.filter((c) => c === '/api/autoradio/learning').length).toBe(1); expect(calls.filter((c) => c === '/api/texture/edition').length).toBe(1);
    await notebookShared(args(), storage, env());
    expect(calls.filter((c) => c === '/api/autoradio/learning').length).toBe(1);
    await notebookShared(args({ tick: 1 }), storage, env());
    expect(calls.filter((c) => c === '/api/autoradio/learning').length).toBe(2);
  });
  test('R3 a page not on top reads nothing and is idle; coming back reads again', async () => {
    await notebookShared(args(), storage, env());
    const n = calls.length;
    const idle = await notebookShared(args({ active: false }), storage, env());
    expect(calls.length).toBe(n); expect(idle).toMatchObject({ docState: '', scoreState: '', sections: [] });          // the EMPTY state: Exact keeps the last answer, and a non-empty one would hide the kept copy (members-notebook.contract)
    await notebookShared(args(), storage, env());
    expect(calls.length).toBeGreaterThan(n);
  });
});

describe('R4 to R7 the native fact and call', () => {
  test('R4 the steer taps waiting (steer.ts, shared) are shown, with or without a module; a failing or silent native does not hide the page', async () => {
    for (let i = 0; i < 3; i++) await steerEnqueue(storage, { id: `t${i}`, entryID: 'e', direction: 'more', stories: [], title: '', at: '' });
    expect((await notebookShared(args(), storage, env())).steerLine).toBe('3 more or less taps waiting to reach your computer.');
    forgetNotebook();
    expect((await notebookShared(args(), storage, env(null))).steerLine).toBe('3 more or less taps waiting to reach your computer.');
    forgetNotebook();
    const broken: Device = { state: async () => { throw new Error('native'); } };
    const p = await notebookShared(args(), storage, env(broken));
    expect(p.steerLine).toBe('3 more or less taps waiting to reach your computer.'); expect(p.docState).toBe('ready');
    forgetNotebook();
    const silent: Device = { state: () => new Promise(() => undefined) };
    const q = await notebookShared(args(), storage, { ...env(silent), deviceDeadlineMs: 20 });
    expect(q.docState).toBe('ready'); expect(q.steerLine).toBe('3 more or less taps waiting to reach your computer.');
  });
  test('R6 a sign-out reads again; an old read never answers the new key', async () => {
    await notebookShared(args(), storage, env());
    const n = calls.length;
    await notebookShared(args({ signedOutTick: '1' }), storage, env());
    expect(calls.length).toBeGreaterThan(n);
    let release: () => void = () => undefined;
    const slow: Get = async (p) => { if (p === '/api/autoradio/learning') await new Promise<void>((r) => { release = r; }); return get(p); };
    forgetNotebook();
    const first = notebookShared(args({ tick: 5 }), storage, { get: slow, device });
    await new Promise((r) => setTimeout(r, 5));
    const second = await notebookShared(args({ tick: 6 }), storage, env());
    release();
    await first;
    expect(second.scoreState).toBe('ready');
    expect((await notebookShared(args({ tick: 6 }), storage, env())).scoreState).toBe('ready');
  });
  test('R7 the topics are written by native before the notebook is read', async () => {
    const seen: string[] = [];
    const g: Get = async (p, q) => { seen.push(order.join(',')); return get(p, q); };
    await notebookShared(args(), storage, { get: g, device });
    expect(order).toEqual(['native']); expect(seen.every((s) => s === 'native')).toBe(true);
  });
});
