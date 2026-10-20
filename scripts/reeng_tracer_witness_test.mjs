// node --test scripts/reeng_tracer_witness_test.mjs
//
// The release witness check of the engineering go-served scenario (S6-0).
// Ways `witness_joined` could fail, written before the code:
//  - W1 a witness missing one expected pair passes;
//  - W2 a pair the table does not know passes (the Go table and this list
//    drifted apart);
//  - W3 a pair whose compared_at_gate flag differs between two moments passes
//    (the flag is a constant of the gate, not of the moment);
//  - W4 a moment that was never fetched, or answered non-200, passes;
//  - W5 an unbound or unknown pair fails the check (the gap table is the
//    finding; the check is that the witness answered completely);
//  - W6 a duplicated pair, a missing reason or a non-boolean flag passes;
//  - W7 the gap table drops a pair or mislabels a moment.
// internal/agentcore/selfdev_release_witness_test.go reads WITNESS_PAIRS and
// fails when it differs from the Go table.

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  WITNESS_PAIRS, WITNESS_SCHEMA, WITNESS_TAGS, formatWitnessGapTable, witnessGapTable, witnessJoined,
} from './reeng_tracer_checks.mjs';

// Gate flags as the Go table has them for the recorded-only pairs.
const NOT_COMPARED = new Set(['test_receipts_exit', 'builder_receipt_patch', 'manifest_builder_receipt', 'frozen_surface_decision_heads', 'divergence_derived']);

const binding = (pair, bound = null) => ({
  pair, left: 'l', right: 'r', compared_at_gate: !NOT_COMPARED.has(pair), compared_at: NOT_COMPARED.has(pair) ? 'none' : 'decision',
  gate_ref: 'ref', bound, reason: `${pair} reason`,
});

const fetched = (state, bound = null) => ({
  status: 200, operation_state: state,
  body: { schema: WITNESS_SCHEMA, operation: { state }, bindings: WITNESS_PAIRS.map((p) => binding(p, bound)) },
});

const witnesses = () => ({
  frozen: fetched('awaiting_approval'), accepted: fetched('accepted'), applied: fetched('applied', true), restored: fetched('applied'),
});

test('a complete witness at all four moments joins', () => {
  const result = witnessJoined(witnesses());
  assert.equal(result.ok, true, JSON.stringify(result));
});

test('W5: unbound and unknown pairs do not fail the check', () => {
  const w = witnesses();
  w.applied.body.bindings.find((b) => b.pair === 'bundle_operation').bound = false;
  w.restored.body.bindings.find((b) => b.pair === 'manifest_decision').bound = null;
  assert.equal(witnessJoined(w).ok, true);
});

test('W1: a missing pair fails and is named', () => {
  const w = witnesses();
  w.accepted.body.bindings = w.accepted.body.bindings.filter((b) => b.pair !== 'route_event');
  const result = witnessJoined(w);
  assert.equal(result.ok, false);
  assert.deepEqual(result.missing, ['accepted:route_event']);
});

test('W2: an unknown pair fails', () => {
  const w = witnesses();
  w.frozen.body.bindings.push(binding('a_pair_the_table_lacks'));
  const result = witnessJoined(w);
  assert.equal(result.ok, false);
  assert.deepEqual(result.unexpected, ['frozen:a_pair_the_table_lacks']);
});

test('W3: a gate flag that differs between moments fails', () => {
  const w = witnesses();
  w.restored.body.bindings.find((b) => b.pair === 'test_receipts_exit').compared_at_gate = true;
  const result = witnessJoined(w);
  assert.equal(result.ok, false);
  assert.deepEqual(result.inconsistent, ['test_receipts_exit']);
});

test('W4: a moment never fetched or answered non-200 fails', () => {
  const never = witnesses();
  delete never.restored;
  assert.deepEqual(witnessJoined(never).tags_failed, ['restored']);
  const refused = witnesses();
  refused.applied = { status: 502, body: null, error: 'bad gateway' };
  const result = witnessJoined(refused);
  assert.equal(result.ok, false);
  assert.deepEqual(result.tags_failed, ['applied']);
  assert.equal(witnessJoined(undefined).ok, false);
  assert.equal(witnessJoined({}).ok, false);
});

test('W6: a duplicate pair, an empty reason, or a non-boolean flag fails', () => {
  const duplicate = witnesses();
  duplicate.frozen.body.bindings.push(binding('decision_operation'));
  assert.equal(witnessJoined(duplicate).ok, false);
  const noReason = witnesses();
  noReason.frozen.body.bindings[0].reason = '  ';
  assert.deepEqual(witnessJoined(noReason).malformed, ['frozen:decision_operation']);
  const flag = witnesses();
  flag.accepted.body.bindings[1].compared_at_gate = 'yes';
  assert.equal(witnessJoined(flag).ok, false);
  const wrongSchema = witnesses();
  wrongSchema.frozen.body.schema = 'other';
  assert.deepEqual(witnessJoined(wrongSchema).tags_failed, ['frozen']);
});

test('W7: the gap table has every pair and labels each moment', () => {
  const w = witnesses();
  const rows = witnessGapTable(w);
  assert.deepEqual(rows.map((r) => r.pair), WITNESS_PAIRS);
  const exit = rows.find((r) => r.pair === 'test_receipts_exit');
  assert.equal(exit.compared_at_gate, false);
  assert.deepEqual(Object.keys(exit.bound), WITNESS_TAGS);
  assert.equal(exit.bound.applied, true);
  assert.equal(exit.bound.frozen, null);
  const text = formatWitnessGapTable(rows);
  assert.equal(text.split('\n').length, WITNESS_PAIRS.length + 1);
  assert.match(text, /test_receipts_exit\s+NO \(none\)\s+-\s+-\s+Y\s+-/);
  assert.match(text, /decision_operation\s+yes \(decision\)/);
  const partial = witnesses();
  delete partial.restored;
  assert.equal(witnessGapTable(partial).find((r) => r.pair === 'route_event').bound.restored, undefined);
});
