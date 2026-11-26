// Account wipe uses the explicit package send. It never overloads stop, and a package refusal does not leave the shared sheet behind.
import { afterEach, expect, test } from 'bun:test';
import { resetRadio, wipeRadio, SHEET_FILE, SHEET_BAD_FILE } from './radio-bridge.ts';
import { wipeAll, WIPES } from './account.ts';

const globals = globalThis as typeof globalThis & { __autoradioDeckPackage?: (name: string, args: Record<string, unknown>) => Promise<unknown> };
const original = globals.__autoradioDeckPackage;
afterEach(() => { if (original) globals.__autoradioDeckPackage = original; else delete globals.__autoradioDeckPackage; });
const empty = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };

test('the shared radio wipe sends deckWipe, not deckAct stop', async () => {
  const calls: string[] = [];
  globals.__autoradioDeckPackage = async (name) => { calls.push(name); return empty; };
  resetRadio(); await wipeRadio(undefined);
  expect(calls).toEqual(['deckWipe']);
});

test('a refused deckWipe still removes both shared sheet files', async () => {
  globals.__autoradioDeckPackage = async () => { throw Object.assign(new Error('absent'), { name: 'NotSupportedError' }); };
  const files = new Map<string, Uint8Array>([[SHEET_FILE, new Uint8Array([1])], [SHEET_BAD_FILE, new Uint8Array([2])]]);
  const storage = { fs: { rm: async (p: string) => { files.delete(p); }, readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async () => undefined } };
  await wipeRadio(storage as never);
  expect([...files.keys()]).toEqual([]);
});

test('the account wipe list includes the radio adapter that sends deckWipe', async () => {
  const calls: string[] = [];
  globals.__autoradioDeckPackage = async (name) => { calls.push(name); return empty; };
  const failed = await wipeAll(undefined, undefined, WIPES.filter((w) => w.name === 'radio'));
  expect(failed).toEqual([]); expect(calls).toEqual(['deckWipe']);
});
