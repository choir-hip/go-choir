import { test, expect } from './helpers/fixtures.js';
import { resolvePublication } from '../src/lib/texture.js';

// The published reader polls the bare link every minute only to learn
// whether a newer version exists. A poll that rebuilt the whole bundle cost
// about a second of server time and up to 394 KB on the paper
// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md). Ways it can
// fail: the poll still asks for the bundle; the article load asks for the
// nav alone and renders nothing.
test('a nav resolve asks for view=nav; a plain resolve asks for the bundle', async () => {
  const seen = [];
  const prev = globalThis.fetch;
  globalThis.fetch = async (url) => {
    seen.push(String(url));
    return new Response(JSON.stringify({ nav: { latest: 2 } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
  };
  try {
    await resolvePublication('/pub/texture/harbor');
    await resolvePublication('/pub/texture/harbor', { view: 'nav' });
  } finally {
    globalThis.fetch = prev;
  }
  expect(seen[0]).toContain('/publications/resolve?route=%2Fpub%2Ftexture%2Fharbor');
  expect(seen[0]).not.toContain('view=');
  expect(seen[1]).toContain('view=nav');
});
