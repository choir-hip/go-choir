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
//
// The requested go test may run as a check cell, not a job: the engineering
// prompt and Freeze admit a receipted cell that panics when the test fails
// (docs/archive/problems/go-served-1143-first-full-selfdev-loop-test-job-not-done-2026-10-15.md).
// Ways the cell check could fail, written before the code:
//  - G1 a check cell whose go test failed (the cell panicked) counts;
//  - G2 a cell that runs go test but never panics counts (its exit code
//    says nothing about the test);
//  - G3 a cell from another assignment's inbox counts;
//  - G4 a cell result with no capsule-go-eval receipt counts;
//  - G5 the `go -C /workspace/platform test` and `sh -c "cd ... && go test"`
//    forms are not recognized;
//  - G6 a cell that only runs go vet counts as a test;
//  - G7 a payload served as a JSON string (as /api/runs/{id}/events does)
//    is not read.

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  castOnRecords, reportNamesBundle, verifiedNamedBundle, completingInbox, doneJobs, isGoBuildOfAutoputer, isGoTest,
  goTestCells,
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

function cellEvents(inbox, callID, source, result, { stringPayload = false } = {}) {
  const wrap = (p) => (stringPayload ? JSON.stringify(p) : p);
  return [
    { agent_id: inbox, kind: 'tool.invoked', payload: wrap({ tool: 'capsule_go_eval', call_id: callID, arguments: { source } }) },
    { agent_id: inbox, kind: 'tool.result', payload: wrap({ tool: 'capsule_go_eval', call_id: callID, is_error: false, output: JSON.stringify(result) }) },
  ];
}

const SH_TEST = 'r, err := choir.Exec("sh", []string{"-c", "cd /workspace/platform && go test ./internal/selfdevprobe/"})\nif err != nil || r.ExitCode != 0 {\n    println(r.Stderr)\n    panic("go test failed")\n}';
const C_TEST = 'r, err := choir.Exec("go", []string{"-C", "/workspace/platform", "test", "./internal/selfdevprobe/"})\nif err != nil || r.ExitCode != 0 { panic(r.Stderr) }';
const OK = { exit_code: 0, receipt_ref: 'capsule-go-eval:sha256:' + 'd'.repeat(64) };
const PANICKED = { exit_code: 1, error: 'go test failed', stdout: 'FAIL', receipt_ref: '' };
const ENG = 'engineering:assignment-4';

test('G1-G7: a passing go test check cell of the completing assignment counts', () => {
  // The go-served-1143 shape: a failed cell, then a passing one.
  const events = [...cellEvents(ENG, 'c1', SH_TEST, PANICKED), ...cellEvents(ENG, 'c2', SH_TEST, OK)];
  const cells = goTestCells(events, ENG);
  assert.deepEqual(cells.map((x) => x.call_id), ['c2']);
  assert.equal(cells[0].receipt_ref, OK.receipt_ref);
  // G1: only the failed cell.
  assert.equal(goTestCells(cellEvents(ENG, 'c1', SH_TEST, PANICKED), ENG).length, 0);
  // G2: no panic, no evidence.
  const noPanic = 'r, _ := choir.Exec("sh", []string{"-c", "cd /workspace/platform && go test ./..."})\nprintln(r.Stdout)';
  assert.equal(goTestCells(cellEvents(ENG, 'c3', noPanic, OK), ENG).length, 0);
  // G3: another assignment's cell.
  assert.equal(goTestCells(cellEvents('engineering:assignment-1', 'c4', SH_TEST, OK), ENG).length, 0);
  // G4: no receipt, or a job receipt on a cell result.
  assert.equal(goTestCells(cellEvents(ENG, 'c5', SH_TEST, { exit_code: 0 }), ENG).length, 0);
  assert.equal(goTestCells(cellEvents(ENG, 'c6', SH_TEST, { exit_code: 0, receipt_ref: 'capsule-exec:sha256:x' }), ENG).length, 0);
  // G5: the go -C form.
  assert.equal(goTestCells(cellEvents(ENG, 'c7', C_TEST, OK), ENG).length, 1);
  // G6: go vet is not a test.
  const vet = 'r, err := choir.Exec("go", []string{"-C", "/workspace/platform", "vet", "./internal/apihandler/"})\nif err != nil || r.ExitCode != 0 { panic(r.Stderr) }';
  assert.equal(goTestCells(cellEvents(ENG, 'c8', vet, OK), ENG).length, 0);
  // G7: string payloads.
  assert.equal(goTestCells(cellEvents(ENG, 'c9', SH_TEST, OK, { stringPayload: true }), ENG).length, 1);
  // A result with no matching invocation does not count.
  assert.equal(goTestCells([cellEvents(ENG, 'c10', SH_TEST, OK)[1]], ENG).length, 0);
});

test('G8: tool.invoked is a bus-only line (journal cut R3); the result carries the cell source', () => {
  // The shape a new build writes: no tool.invoked row, the arguments on the result.
  const resultOnly = (callID, source, result) => [
    { agent_id: ENG, kind: 'tool.result', payload: { tool: 'capsule_go_eval', call_id: callID, arguments: { source }, started_at: '2026-10-22T19:00:00Z', duration_ms: 40, is_error: false, output: JSON.stringify(result) } },
  ];
  assert.deepEqual(goTestCells(resultOnly('r1', SH_TEST, OK), ENG).map((x) => x.call_id), ['r1']);
  // The same rules hold on the new shape: a failed cell, a cell with no panic, a job receipt.
  assert.equal(goTestCells(resultOnly('r2', SH_TEST, PANICKED), ENG).length, 0);
  const noPanic = 'r, _ := choir.Exec("sh", []string{"-c", "cd /workspace/platform && go test ./..."})\nprintln(r.Stdout)';
  assert.equal(goTestCells(resultOnly('r3', noPanic, OK), ENG).length, 0);
  assert.equal(goTestCells(resultOnly('r4', SH_TEST, { exit_code: 0, receipt_ref: 'capsule-exec:sha256:x' }), ENG).length, 0);
  // A tape that holds both shapes (an old run, then a new one) counts each cell once.
  const both = [...cellEvents(ENG, 'o1', SH_TEST, OK), ...resultOnly('n1', SH_TEST, OK)];
  assert.deepEqual(goTestCells(both, ENG).map((x) => x.call_id), ['o1', 'n1']);
});
