#!/usr/bin/env node
// Diff the web and native projections of the golden documents.
//
//   node scripts/golden-docs-compare.mjs <out-dir>
//
// <out-dir>/web/<name>.json   from frontend/tests/golden-documents.spec.js
// <out-dir>/native/<name>.json from AutoradioKit GoldenDocumentsTests
// internal/texturedoc/golden/known-differences.json names every difference we
// know, each with a reason. Exit 0 only when every document has both sides, no
// difference is unlisted, and no listed difference has gone stale.
//
// Ways it can fail, written before the code:
//   1. one side is missing a document (a skipped test): that is a failure, not a pass;
//   2. a block dropped on one side shifts every later index: blocks are aligned by
//      longest common subsequence first, then the gaps are paired field by field;
//   3. a known difference stays in the file after the renderer is fixed: reported
//      as stale and fails, so the list cannot rot into a tolerance;
//   4. a known difference is entered without a reason: the file is refused.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const goldenDir = path.resolve(here, '..', 'internal', 'texturedoc', 'golden');
const out = process.argv[2];
if (!out) { console.error('usage: golden-docs-compare.mjs <out-dir>'); process.exit(2); }

const names = fs.readdirSync(goldenDir).filter((f) => f.endsWith('.json') && f !== 'known-differences.json').map((f) => f.slice(0, -5)).sort();
const known = JSON.parse(fs.readFileSync(path.join(goldenDir, 'known-differences.json'), 'utf8')).differences || [];
for (const k of known) {
  if (!k.doc || !k.key || !String(k.reason || '').trim() || !String(k.side || '').trim()) {
    console.error(`known-differences.json: entry needs doc, key, side and reason: ${JSON.stringify(k)}`);
    process.exit(2);
  }
}

// Canonical JSON: keys sorted, so the order a side wrote its fields in never counts.
const canon = (v) => (Array.isArray(v) ? `[${v.map(canon).join(',')}]` : v && typeof v === 'object' ? `{${Object.keys(v).sort().map((k) => `${JSON.stringify(k)}:${canon(v[k])}`).join(',')}}` : JSON.stringify(v));
const sig = canon;
const show = (v) => (v === undefined ? '(absent)' : canon(v));

function lcsAlign(a, b) {
  const n = a.length, m = b.length;
  const dp = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) dp[i][j] = sig(a[i]) === sig(b[j]) ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  const pairs = []; // [i, j] equal anchors
  let i = 0, j = 0;
  while (i < n && j < m) {
    if (sig(a[i]) === sig(b[j])) { pairs.push([i, j]); i++; j++; }
    else if (dp[i + 1][j] >= dp[i][j + 1]) i++; else j++;
  }
  return pairs;
}

function diffDoc(web, native) {
  const diffs = []; // {key, text}
  const pairs = lcsAlign(web, native);
  pairs.push([web.length, native.length]);
  let wi = 0, ni = 0;
  for (const [pi, pj] of pairs) {
    const gapW = web.slice(wi, pi), gapN = native.slice(ni, pj);
    const common = Math.min(gapW.length, gapN.length);
    for (let k = 0; k < common; k++) {
      const w = gapW[k], n = gapN[k];
      const fields = [...new Set([...Object.keys(w), ...Object.keys(n)])].sort();
      for (const f of fields) {
        if (canon(w[f]) !== canon(n[f])) {
          diffs.push({ key: `web#${wi + k}~native#${ni + k}.${f}`, text: `block web#${wi + k} (${w.kind}) vs native#${ni + k} (${n.kind}), field ${f}\n      web:    ${show(w[f])}\n      native: ${show(n[f])}` });
        }
      }
    }
    for (let k = common; k < gapW.length; k++) diffs.push({ key: `web#${wi + k}`, text: `only on web, block web#${wi + k}: ${show(gapW[k])}` });
    for (let k = common; k < gapN.length; k++) diffs.push({ key: `native#${ni + k}`, text: `only on native, block native#${ni + k}: ${show(gapN[k])}` });
    wi = pi + 1; ni = pj + 1;
  }
  return diffs;
}

let failures = 0;
const used = new Set();
const rows = [];
for (const name of names) {
  const wf = path.join(out, 'web', `${name}.json`), nf = path.join(out, 'native', `${name}.json`);
  if (!fs.existsSync(wf) || !fs.existsSync(nf)) {
    console.log(`FAIL ${name}: missing ${!fs.existsSync(wf) ? 'web' : ''} ${!fs.existsSync(nf) ? 'native' : ''} projection`);
    failures++; rows.push([name, 'MISSING']); continue;
  }
  const web = JSON.parse(fs.readFileSync(wf, 'utf8')).blocks, native = JSON.parse(fs.readFileSync(nf, 'utf8')).blocks;
  const diffs = diffDoc(web, native);
  const unlisted = [], listed = [];
  for (const d of diffs) {
    const k = known.findIndex((e) => e.doc === name && e.key === d.key);
    if (k >= 0) { used.add(k); listed.push({ ...d, reason: known[k].reason, side: known[k].side }); } else unlisted.push(d);
  }
  if (unlisted.length) {
    failures++;
    console.log(`FAIL ${name}: ${unlisted.length} unlisted difference(s) (${web.length} web blocks, ${native.length} native blocks)`);
    for (const d of unlisted) console.log(`  [${d.key}] ${d.text}`);
  } else console.log(`ok   ${name}: ${web.length} blocks${listed.length ? `, ${listed.length} known difference(s)` : ' identical'}`);
  for (const d of listed) console.log(`  known (${d.side}) [${d.key}]: ${d.reason}`);
  rows.push([name, unlisted.length ? `FAIL ${unlisted.length}` : listed.length ? `known ${listed.length}` : 'identical']);
}
known.forEach((e, k) => {
  if (!used.has(k) && names.includes(e.doc)) { console.log(`STALE known difference no longer observed: ${e.doc} [${e.key}] ${e.reason}`); failures++; }
  else if (!names.includes(e.doc)) { console.log(`STALE known difference names an unknown document: ${e.doc}`); failures++; }
});
console.log('\nsummary: ' + rows.map(([n, s]) => `${n}=${s}`).join('  '));
console.log(failures ? `GOLDEN COMPARE FAILED (${failures})` : 'GOLDEN COMPARE OK');
process.exit(failures ? 1 : 0);
