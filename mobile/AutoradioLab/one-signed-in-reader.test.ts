// Station E6.2 (docs/design/phone-sign-out-and-server-client-in-exact-2026-11-27.md): "signed in" has ONE native reader, the one the voice turn's gate needs
// (TurnGate.keyPresent), and shared code is where pages learn it (choirAuth, account-state.ts). Failure list, written before the change:
//  R1 no contract, interface or shared file names the native `choirRefresh`; the ChoirPlayer and BarFacts/BarInfo shapes carry no `signedIn` (nothing read it).
//  R2 no native Swift file reads "signed in" for the app's pages: ChoirSignIn has no `signedIn`, ChoirApp's refresh and playerView do not read it, the module's
//     `keystate` op and its Keychain read are gone.
//  R3 the host probe ("is the app module here") does not need a key read: a handle that answers `mailCan` is native; one that answers only the old `keystate` is not.
//  R4 a host with no store-backed credentials and a native that answers nothing for the key is "unreadable" (locked), never "absent": no flap to signed out.
// These are source checks (the same kind as android-auth.test.ts A1.4) plus behaviour; nothing here runs on a device.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { hostMode, resetHost } from './host.ts';
import { adoptNative, bindNative, keyState, resetSeam } from './api.ts';

const dir = dirname(fileURLToPath(import.meta.url));
const read = (...p: string[]): string => readFileSync(join(dir, ...p), 'utf8');
const kit = (f: string): string => read('..', 'AutoradioKit', 'Sources', 'AutoradioKit', f);
const pkg = (f: string): string => read('..', 'exact-package', f);

beforeEach(() => { resetSeam(); resetHost(); });

describe('E6.2 one native reader of signed in', () => {
  test('R1 no choirRefresh, no signedIn on the native player or bar facts', () => {
    expect(read('account.contract')).not.toContain('choirRefresh');
    expect(read('app.contract')).not.toContain('refreshRadio');
    expect(pkg('exact.json')).not.toContain('choirRefresh');
    expect(read('..', 'scripts', 'gen-exact-interface.py')).not.toContain('choirRefresh');
    expect(pkg('apple/AutoradioModule.swift')).not.toContain('choirRefresh');
    const iface = JSON.parse(pkg('exact.json')) as { shapes: Record<string, Record<string, string>> };
    expect(Object.keys(iface.shapes.ChoirPlayer)).not.toContain('signedIn');
    expect(iface.shapes.ChoirAuth.signedIn).toBe('bool');                                 // the one shared reader keeps its shape
    expect(JSON.parse(pkg('interface/bar.json')).shapes.BarFacts).not.toHaveProperty('signedIn');
    expect(read('bar-types.contract')).not.toContain('signedIn');
    expect(pkg('apple/AutoradioModule+Bar.swift')).not.toContain('signedIn');
    expect(read('bar.ts')).not.toContain('signedIn');
  });

  test('R2 no native file reads "signed in" for the pages; the Keychain probe op is gone', () => {
    expect(kit('Choir/ChoirSignIn.swift')).not.toContain('signedIn');
    const app = kit('Choir/ChoirApp.swift');
    expect(app).not.toContain('signIn.signedIn');
    expect(app).not.toContain('var signedIn');
    const models = read('modules', 'apple', 'ModelsEntry.swift');
    expect(models).not.toContain('keyState');
    expect(models).not.toContain('"keystate"');
    expect(kit('TurnGate.swift')).toContain('keyPresent');                                // the voice's gate reader stays: the one native reader
  });

  test('R3 the host probe asks mailCan, which reads no key', async () => {
    adoptNative({ available: true, later: async (r: { op: string }) => (r.op === 'mailCan' ? { ok: true } : r.op === 'keystate' ? { state: 'present' } : {}) } as never, 'x');
    expect(await hostMode(20)).toBe('native');
    resetHost(); resetSeam();
    adoptNative({ available: true, later: async (r: { op: string }) => (r.op === 'keystate' ? { state: 'present' } : {}) } as never, 'y');     // only the old probe answers: not a module of ours
    expect(await hostMode(40, 20)).toBe('none');
  });

  test('R4 a native that says nothing about the key is unreadable, never absent', async () => {
    bindNative({ available: true, later: async () => ({}) } as never);
    expect((await keyState()).state).toBe('unreadable');
  });
});
