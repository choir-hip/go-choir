// node --test scripts/reeng_tracer_checks_test.mjs
//
// The engineering scenario's acceptance predicates on a synthetic claim
// shaped like drill selfdev-jobs-0101: Texture Cast management in four
// rounds, management Cast four engineering assignments, the fourth froze;
// Verify moved the operation to a new bundle digest.
//
// Ways the checks could fail, written before the code
// (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md, T1-T6):
//  - T1 jobs read from an assignment other than the completing one;
//  - T2 a test job run through `sh -c "... go test ..."` is not recognized;
//  - T3 a Report naming the frozen digest fails because Verify re-digested;
//  - T4 a Report naming some other digest passes;
//  - T5 an engineering Cast whose parent is a later Texture round's Cast fails;
//  - T6 an engineering Cast whose parent is not a Texture Cast passes.

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  castOnRecords, reportNamesBundle, verifiedNamedBundle, completingInbox, doneJobs, isGoBuildOfAutoputer, isGoTest,
} from './reeng_tracer_checks.mjs';

const FROZEN = 'a'.repeat(64);
const FINAL = 'b'.repeat(64);
const OTHER = 'c'.repeat(64);
const OP = 'selfdev-op-1';

function claim() {
  return {
    texture_cast_to_management: { record_id: 'texture-cast-r1' },
    texture_casts_to_management: [{ record_id: 'texture-cast-r1' }, { record_id: 'texture-cast-r4' }],
    engineering_casts: [
      { inbox: 'engineering:assignment-1', record_id: 'mgmt-cast-1', parent_id: 'texture-cast-r1' },
      { inbox: 'engineering:assignment-4', record_id: 'mgmt-cast-4', parent_id: 'texture-cast-r4' },
    ],
    operation_states: [
      { operation_id: OP, state: 'executing', bundle_digest: null },
      { operation_id: OP, state: 'frozen', bundle_digest: FROZEN },
      { operation_id: OP, state: 'awaiting_approval', bundle_digest: FINAL, verifier_refs: ['v'] },
    ],
    verify: { verifier_refs: ['v'], bundle_digest: FINAL },
    jobs_by_inbox: {
      'engineering:assignment-1': [
        { state: 'stopped', argv: ['go', 'build', '-o', '/var/lib/artifact/release/bin/autoputer', './cmd/autoputer'], receipt_ref: null },
      ],
      'engineering:assignment-4': [
        { state: 'done', argv: ['sh', '-c', 'cd /workspace/platform && go test ./internal/drillmark/'], receipt_ref: 'capsule-exec:sha256:1' },
        { state: 'done', argv: ['go', 'build', '-o', '/var/lib/artifact/release/bin/autoputer', '-ldflags=-s -w', './cmd/autoputer'], receipt_ref: 'capsule-exec:sha256:2' },
      ],
    },
  };
}
const op = { operation_id: OP, bundle_digest: FINAL };
const report = { sender: 'engineering:assignment-4', operation_id: OP, bundle_digest: FROZEN };

test('T5/T6: a Cast on any Texture round is on records; a Cast on anything else is not', () => {
  const c = claim();
  assert.equal(castOnRecords(c, c.engineering_casts[1]), true);
  assert.equal(castOnRecords(c, c.engineering_casts[0]), true);
  assert.equal(castOnRecords(c, { record_id: 'mgmt-cast-x', parent_id: 'management-own-cell' }), false);
  assert.equal(castOnRecords(c, { record_id: 'mgmt-cast-y', parent_id: null }), false);
});

test('T3: the Report names the frozen digest after Verify re-digested', () => {
  const c = claim();
  assert.equal(reportNamesBundle(c, report, op), true);
  assert.equal(verifiedNamedBundle(c, report, op), true);
});

test('T4: a Report naming another digest or another operation fails', () => {
  const c = claim();
  assert.equal(reportNamesBundle(c, { ...report, bundle_digest: OTHER }, op), false);
  assert.equal(reportNamesBundle(c, { ...report, operation_id: 'selfdev-op-2' }, op), false);
  // A frozen state of another operation does not count.
  c.operation_states.push({ operation_id: 'selfdev-op-2', state: 'frozen', bundle_digest: OTHER });
  assert.equal(reportNamesBundle(c, { ...report, bundle_digest: OTHER }, op), false);
  // No verifier ref, no verification.
  assert.equal(verifiedNamedBundle({ ...c, verify: { verifier_refs: [] } }, report, op), false);
});

test('T1/T2: jobs come from the completing assignment; sh -c go test counts', () => {
  const c = claim();
  const inbox = completingInbox(c, report);
  assert.equal(inbox, 'engineering:assignment-4');
  const done = doneJobs(c.jobs_by_inbox[inbox]);
  assert.equal(done.some(isGoBuildOfAutoputer), true);
  assert.equal(done.some(isGoTest), true);
  // The first assignment's stopped build is not done.
  const first = doneJobs(c.jobs_by_inbox['engineering:assignment-1']);
  assert.equal(first.some(isGoBuildOfAutoputer), false);
  // No completing Report: the newest Cast's inbox, never the first.
  assert.equal(completingInbox(c, null), 'engineering:assignment-4');
  // A done job without a receipt does not count.
  assert.equal(doneJobs([{ state: 'done', argv: ['go', 'test', './...'], receipt_ref: null }]).length, 0);
  assert.equal(isGoTest({ argv: ['go', 'vet', './...'] }), false);
});
