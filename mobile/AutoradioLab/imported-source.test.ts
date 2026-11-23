// C-3 of the train review: a voice answer must never wait on the network listing of the Imported documents folder (up to 120 s on a dead link).
//  X1 docs() answers at once with what the source last read, whatever the loader is doing; the first call starts one read in the background.
//  X2 one read at a time: calls while a read is in flight start no second one; a read that finds a change calls `changed` once.
//  X3 a read that fails keeps what was held; a read that lands after the account moved is dropped.
import { describe, expect, test } from 'bun:test';
import { ImportedSource } from './imported-search.ts';
import type { Doc } from './search.ts';

const d = (id: string): Doc => ({ id, kind: 'document', title: id, text: '', date: 1, source: 'Imported documents', address: `imported:${id}`, key: id });
const tick = (): Promise<void> => new Promise((r) => setTimeout(r, 0));

describe('the imported documents source', () => {
  test('X1 a loader that never answers does not hold the answer', async () => {
    let started = 0;
    const s = new ImportedSource(() => { started++; return new Promise<Doc[]>(() => undefined); }, () => 0, () => undefined);
    const t = performance.now();
    expect(await s.docs()).toEqual([]);
    expect(await s.docs()).toEqual([]);
    expect(performance.now() - t).toBeLessThan(50);
    expect(started).toBe(1);
  });
  test('X2 one read at a time, and a change is announced once', async () => {
    let n = 0, changed = 0; let release: (v: Doc[]) => void = () => undefined;
    const s = new ImportedSource(() => { n++; return new Promise<Doc[]>((r) => { release = r; }); }, () => 0, () => { changed++; });
    await s.docs(); await s.docs();
    release([d('a')]); await tick();
    expect(changed).toBe(1);
    expect((await s.docs()).map((x) => x.id)).toEqual(['a']);        // held now; this call also starts the next read
    expect(n).toBe(2);
    release([d('a')]); await tick();
    expect(changed).toBe(1);                                         // nothing changed
  });
  test('X3 a failed read keeps what was held; a stale account drops the read', async () => {
    let gen = 0; let mode: 'ok' | 'fail' = 'ok';
    const s = new ImportedSource(async () => { if (mode === 'fail') throw new Error('down'); const g = gen; await tick(); gen = g === 0 ? gen : gen; return [d('a')]; }, () => gen, () => undefined);
    await s.docs(); await tick(); await tick();
    expect((await s.docs()).length).toBe(1);
    mode = 'fail'; await tick(); await tick();
    expect((await s.docs()).length).toBe(1);
    await tick(); await tick();
    const late = new ImportedSource(async () => { await tick(); return [d('z')]; }, () => gen, () => undefined);
    await late.docs(); gen++; await tick(); await tick();
    expect(await late.docs()).toEqual([]);
  });
});
