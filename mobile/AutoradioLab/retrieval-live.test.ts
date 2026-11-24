// The live search pass may download its model only on the search group's own agreement (review, 2026-11-24, finding 2): a voice press must leave it false.
import { beforeEach, expect, test } from 'bun:test';
import { live } from './retrieval-live.ts';
import { consentAgree, forgetConsent } from './consent.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
beforeEach(() => { files.clear(); forgetConsent(); live.storage = storage; });

test('R1 the search pass downloads only after the search group was agreed to, not after any other group', async () => {
  expect(await live.mayDownload()).toEqual({ ok: false, mobile: false });
  await consentAgree('voice', true, storage); await consentAgree('voice-rest', false, storage);
  expect(await live.mayDownload()).toEqual({ ok: false, mobile: false });
  await consentAgree('search', false, storage);
  expect(await live.mayDownload()).toEqual({ ok: true, mobile: false });
});
