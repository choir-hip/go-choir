// Ranks a fixture of front rows with the front page's own rankFrontStories
// (frontend/src/lib/autopaper.ts) and prints { caseName: [publication_id...] }
// as JSON. The Go parity test for the Autopaper daily digest
// (internal/maild/digest_rank_test.go) runs this under node and compares the
// order with the Go mirror (internal/maild/digest_rank.go), so the two copies
// of the rank cannot drift apart unnoticed.
//
// Usage: node frontend/scripts/rank-front-fixture.mjs <fixture.json>
import { readFileSync } from 'node:fs';
import { rankFrontStories } from '../src/lib/autopaper.ts';

const fixturePath = process.argv[2];
if (!fixturePath) {
  console.error('usage: rank-front-fixture.mjs <fixture.json>');
  process.exit(2);
}
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8'));
const out = {};
for (const c of fixture.cases || []) {
  out[c.name] = rankFrontStories(c.stories).map((s) => s.publication_id);
}
process.stdout.write(JSON.stringify(out, null, 1) + '\n');
