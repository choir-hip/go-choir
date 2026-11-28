// What the bar's prompt is aimed at: a document, or one block of it. Shared code now (reader-aim.ts); the Swift `ReaderAim` and the ops `readerAim` and `readerAimSet` are gone.
// The failure list, written first:
//  A1  set then read gives the aim; clearing (an empty address) gives nothing.
//  A2  a block of a document is a different aim from the document: both are kept as given.
//  A3  setting a second aim replaces the first; nothing is kept of the first.
//  A4  a sign-out forgets it: the next account reads nothing, and its own aim is its own.
//  A5  the page it points at is closed (or never was open): the aim is dropped, and stays dropped when the page is opened again.
//  A6  the block it points at is gone from the document in front (an edit, a restore, a revision): the aim is dropped, not shown with a label for a block that is not there.
//  A7  the document in front is still loading (no blocks yet) or is another page: a block aim is NOT dropped (it cannot be judged yet).
//  A8  the document is replaced by another at the same address that still has the block: the aim stays; one that lacks it drops it.
//  A9  hostile input: a non-string address, block or label is refused or made a string; a very long label is cut.
//  A10 the contract wires it: the resource asks again on the open pages, the document's address, status and block ids and the sign-out tick; the mutation refreshes it; the package no longer names readerAim or readerAimSet.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { aimView, clearAim, forgetAim, setAim, LABEL_MAX } from './reader-aim.ts';
import { nextGeneration } from './api.ts';

const NONE = { address: '', blockId: '', label: '' };
const open = 'pub:a\npub:b';
const ids = 'b1\nb2\nb3';
const see = (o = open, docAddr = '', status = '', blocks = ''): unknown => aimView(o, docAddr, status, blocks);

describe('the reader aim', () => {
  beforeEach(() => forgetAim());

  test('A1 set then read; clear reads nothing', () => {
    expect(see()).toEqual(NONE);
    expect(setAim('pub:a', '', 'The title')).toEqual({ address: 'pub:a', blockId: '', label: 'The title' });
    expect(see()).toEqual({ address: 'pub:a', blockId: '', label: 'The title' });
    expect(setAim('', 'b1', 'x')).toEqual(NONE);
    expect(see()).toEqual(NONE);
    setAim('pub:a', '', 't'); clearAim();
    expect(see()).toEqual(NONE);
  });

  test('A2 and A3 a block aim is kept as given and a second aim replaces the first', () => {
    setAim('pub:a', 'b2', 'Para two');
    expect(see('pub:a', 'pub:a', 'ready', ids)).toEqual({ address: 'pub:a', blockId: 'b2', label: 'Para two' });
    setAim('pub:b', '', 'Doc B');
    expect(see()).toEqual({ address: 'pub:b', blockId: '', label: 'Doc B' });
  });

  test('A4 a sign-out forgets it', () => {
    setAim('pub:a', 'b1', 'old account');
    nextGeneration();
    expect(see()).toEqual(NONE);
    setAim('pub:a', '', 'new account');
    expect(see()).toEqual({ address: 'pub:a', blockId: '', label: 'new account' });
  });

  test('A5 a closed page drops the aim for good', () => {
    setAim('pub:a', '', 'A');
    expect(see('pub:b')).toEqual(NONE);                 // pub:a is not among the open pages
    expect(see(open)).toEqual(NONE);                    // opened again: still gone
  });

  test('A6 a block that is gone from the document in front drops the aim', () => {
    setAim('pub:a', 'b9', 'a block that was deleted');
    expect(see(open, 'pub:a', 'ready', ids)).toEqual(NONE);
    expect(see(open, 'pub:a', 'ready', 'b9')).toEqual(NONE);   // dropped for good, even if the block comes back
  });

  test('A7 a loading document or another page cannot judge a block aim', () => {
    setAim('pub:a', 'b9', 'L');
    expect(see(open, 'pub:a', 'loading', '')).toEqual({ address: 'pub:a', blockId: 'b9', label: 'L' });
    expect(see(open, 'pub:b', 'ready', ids)).toEqual({ address: 'pub:a', blockId: 'b9', label: 'L' });
    expect(see(open, 'pub:a', 'failed', '')).toEqual({ address: 'pub:a', blockId: 'b9', label: 'L' });
  });

  test('A8 the document replaced at the same address keeps a block it still has and drops one it lacks', () => {
    setAim('pub:a', 'b2', 'two');
    expect(see(open, 'pub:a', 'ready', 'b1\nb2')).toEqual({ address: 'pub:a', blockId: 'b2', label: 'two' });
    expect(see(open, 'pub:a', 'ready', 'c1\nc2')).toEqual(NONE);
  });

  test('A9 hostile input', () => {
    expect(setAim(42 as never, 'b', 'l')).toEqual(NONE);
    expect(setAim('pub:a', { x: 1 } as never, 7 as never)).toEqual({ address: 'pub:a', blockId: '', label: '' });
    const long = 'x'.repeat(LABEL_MAX + 500);
    expect((setAim('pub:a', '', long) as { label: string }).label.length).toBe(LABEL_MAX);
    expect(aimView(undefined as never, null as never, 3 as never, {} as never)).toEqual(NONE);
  });

  test('A10 the contract and the package', () => {
    const read = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
    const members = read('./members-reader.contract');
    expect(members).toMatch(/resource raim = readerAimShared\(([^\n]*)\) as shape/);
    const args = /resource raim = readerAimShared\(([^\n]*)\) as shape/.exec(members)?.[1] ?? '';
    for (const need of ['signedOutTick', 'acs.state', 'pg.rows', 'rdoc.address', 'rdoc.status', 'rdoc.blocks']) expect(args).toContain(need);
    expect(members).toMatch(/mutation raimm as shape ReaderAimInfo refreshes raim/);
    expect(members).toContain('readerAimSetShared(');
    expect(members).not.toMatch(/use [^\n]*readerAim\b[^\n]*from "@choir\/autoradio"/);
    const reader = JSON.parse(read('../exact-package/interface/reader.json')) as { names: { source?: string; send?: string }[]; shapes: Record<string, unknown> };
    const names = reader.names.map((n) => n.source ?? n.send);
    expect(names).not.toContain('readerAim');
    expect(names).not.toContain('readerAimSet');
    expect(reader.shapes.ReaderAimInfo).toBeUndefined();
    const app = read('./app.ts');
    expect(app).toContain('readerAimShared:');
    expect(app).toContain('readerAimSetShared:');
  });
});
