// node --test scripts/reeng_tracer_visual_test.mjs
//
// The --visual drill's acceptance predicates on synthetic listings and
// responses (docs/design/visual-evidence-slice-2026-10-15.md section 5).
//
// Ways the drill could falsely pass, written before the code:
//  - V1 a listing for another route than the drill's page passes;
//  - V2 a blank capture passes (a uniform image proves nothing rendered);
//  - V3 a capture whose bytes do not hash to the digest the listing names
//       passes (a swapped asset);
//  - V4 an asset served without nosniff or immutable caching passes;
//  - V5 a response that is 200 with the whole body for a Range request
//       passes the range check;
//  - V6 a capture of the wrong viewport passes;
//  - V7 a capture the store does not hold (resolvable false) passes;
//  - V8 a listing with no capture job receipt passes (hand-written evidence);
//  - V9 a body that is not a PNG but has the right hash passes.
// And a falsely failing one: the vision reading is observed, never a check.

import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { visualEvidenceChecks } from './reeng_tracer_checks.mjs';

const TOKEN = 'choir-drill-1a2b3c4d';
const PNG = Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), Buffer.from('synthetic image bytes for the drill')]);
const digestOf = (b) => createHash('sha256').update(b).digest('hex');
const RECEIPT = `capsule-exec:sha256:${'9'.repeat(64)}`;

function fixture(overrides = {}) {
  const digest = digestOf(PNG);
  const listing = {
    status: 200,
    body: {
      operation_id: 'op-1', capture_receipt: RECEIPT,
      captures: [{
        artifact: `artifact:sha256:${digest}`, digest, route: `/choir-drill/${TOKEN}.html`, width: 1280, height: 800,
        bytes: PNG.length, blank: false, resolvable: true, asset_url: `/api/assets/sha256/${digest}`,
      }],
    },
  };
  const asset = {
    status: 200, bytes: PNG,
    headers: { 'content-type': 'image/png', 'x-content-type-options': 'nosniff', 'cache-control': 'private, max-age=31536000, immutable', etag: `"sha256-${digest}"`, 'accept-ranges': 'bytes' },
  };
  const range = { status: 206, bytes: PNG.subarray(0, 8), headers: { 'content-range': `bytes 0-7/${PNG.length}`, 'content-type': 'image/png' } };
  return { token: TOKEN, listing, asset, range, ...overrides };
}

const failed = (checks) => Object.keys(checks).filter((k) => !checks[k]);

test('a vouched, resolvable, non-blank capture of the drill page served right passes every check', () => {
  assert.deepEqual(failed(visualEvidenceChecks(fixture())), []);
});

test('V1 a capture of another route does not satisfy the drill', () => {
  const f = fixture();
  f.listing.body.captures[0].route = '/';
  assert.ok(failed(visualEvidenceChecks(f)).includes('visual_drill_route_captured'));
});

test('V2 a blank capture fails', () => {
  const f = fixture();
  f.listing.body.captures[0].blank = true;
  assert.ok(failed(visualEvidenceChecks(f)).includes('visual_capture_not_blank'));
});

test('V3 and V9 served bytes must hash to the listed digest and be a PNG', () => {
  const swapped = fixture();
  swapped.asset.bytes = Buffer.concat([PNG, Buffer.from('x')]);
  assert.ok(failed(visualEvidenceChecks(swapped)).includes('visual_asset_bytes_match_digest'));

  const notPng = fixture();
  const text = Buffer.from('not an image at all, but it hashes right');
  notPng.listing.body.captures[0].digest = digestOf(text);
  notPng.listing.body.captures[0].artifact = `artifact:sha256:${digestOf(text)}`;
  notPng.asset.bytes = text;
  assert.ok(failed(visualEvidenceChecks(notPng)).includes('visual_asset_is_png'));
});

test('V4 the asset needs nosniff, an immutable private cache and its digest as validator', () => {
  for (const [name, mutate] of Object.entries({
    nosniff: (h) => { delete h['x-content-type-options']; },
    immutable: (h) => { h['cache-control'] = 'public, max-age=60'; },
    etag: (h) => { h.etag = '"something-else"'; },
    type: (h) => { h['content-type'] = 'text/html'; },
  })) {
    const f = fixture();
    mutate(f.asset.headers);
    assert.ok(failed(visualEvidenceChecks(f)).includes('visual_asset_headers'), name);
  }
});

test('V5 a Range request must answer 206 with the requested bytes', () => {
  const whole = fixture();
  whole.range = { status: 200, bytes: PNG, headers: {} };
  assert.ok(failed(visualEvidenceChecks(whole)).includes('visual_asset_range'));
  const wrong = fixture();
  wrong.range.bytes = PNG.subarray(1, 9);
  assert.ok(failed(visualEvidenceChecks(wrong)).includes('visual_asset_range'));
});

test('V6 and V7 the viewport is 1280x800 and the store holds the capture', () => {
  const small = fixture();
  small.listing.body.captures[0].width = 800;
  assert.ok(failed(visualEvidenceChecks(small)).includes('visual_capture_viewport'));
  const lost = fixture();
  lost.listing.body.captures[0].resolvable = false;
  assert.ok(failed(visualEvidenceChecks(lost)).includes('visual_capture_resolvable'));
});

test('V8 evidence with no capture job receipt fails', () => {
  const f = fixture();
  f.listing.body.capture_receipt = '';
  assert.ok(failed(visualEvidenceChecks(f)).includes('visual_capture_receipted'));
  const odd = fixture();
  odd.listing.body.capture_receipt = `artifact:sha256:${'9'.repeat(64)}`;
  assert.ok(failed(visualEvidenceChecks(odd)).includes('visual_capture_receipted'));
});

test('a missing or refused listing fails the whole group, with no throw', () => {
  for (const listing of [undefined, { status: 404, body: { error: 'x' } }, { status: 200, body: { captures: [] } }, { status: 200, body: null }]) {
    const checks = visualEvidenceChecks(fixture({ listing }));
    assert.ok(failed(checks).length > 0);
  }
  assert.doesNotThrow(() => visualEvidenceChecks({ token: TOKEN }));
});
