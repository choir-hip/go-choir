// node --test scripts/reeng_tracer_receipt_test.mjs
//
// The tracer's receipt across a kill
// (docs/archive/problems/tracer-killed-leaves-computer-running-2026-10-15.md).
// Ways the repair could fail, written before the code:
//  - K1 a SIGKILL leaves nothing: the receipt on disk must already hold the
//    minted computer id while the run is in progress;
//  - K2 a partial receipt reads as a pass (exit_code 0 or no marker);
//  - K3 an interrupt before mint names a computer it never had;
//  - K4 an interrupt records no signal or exit code (128 + signal number);
//  - K5 the startup report misses a computer left by a killed run (receipt
//    still in progress) or by an interrupted run whose stop failed;
//  - K6 the startup report names computers that were stopped, or runs
//    that finished, so an operator stops the wrong thing;
//  - K7 --keep-computer runs are reported as stranded (they are kept on
//    purpose);
//  - K8 a two-computer scenario's stop ({ a, b }) counts as stopped when
//    only one of the two stopped.

import assert from 'node:assert/strict';
import test from 'node:test';

import { inProgressReceipt, interruptedReceipt, signalExitCode, strandedTracerComputers } from './reeng_tracer_receipt.mjs';

const base = () => ({
  schema: 'choir.reeng_tracer_receipt.v1', label: 'go-served-x', started_at: '2026-10-15T22:00:00.000Z',
  attempts: [{ attempt: 1, computer_id: 'computer-abc', phases: [{ name: 'mint', at: '2026-10-15T22:01:00.000Z' }] }],
});

test('K1 in-progress receipt keeps the minted computer and the phases', () => {
  const r = inProgressReceipt(base(), '2026-10-15T22:02:00.000Z');
  assert.equal(r.in_progress, true);
  assert.equal(r.computer_id, 'computer-abc');
  assert.equal(r.attempts[0].phases[0].name, 'mint');
  assert.equal(r.updated_at, '2026-10-15T22:02:00.000Z');
});

test('K2 a partial receipt never reads as a pass', () => {
  const r = inProgressReceipt(base(), '2026-10-15T22:02:00.000Z');
  assert.equal(r.exit_code, null);
  const i = interruptedReceipt(base(), 'SIGTERM', '2026-10-15T22:03:00.000Z', { status: 201, state: 'stopped' });
  assert.notEqual(i.exit_code, 0);
  assert.equal(i.in_progress, false);
});

test('K3 an interrupt before mint names no computer', () => {
  const r = base();
  r.attempts = [{ attempt: 1, phases: [] }];
  const i = interruptedReceipt(r, 'SIGINT', '2026-10-15T22:03:00.000Z', null);
  assert.equal(i.computer_id, null);
  assert.equal(i.interrupted.computer_stop, null);
});

test('K4 an interrupt records the signal and 128 + its number', () => {
  const i = interruptedReceipt(base(), 'SIGHUP', '2026-10-15T22:03:00.000Z', { status: 201, state: 'stopped' });
  assert.deepEqual(i.interrupted, { signal: 'SIGHUP', at: '2026-10-15T22:03:00.000Z', computer_stop: { status: 201, state: 'stopped' } });
  assert.equal(i.exit_code, 129);
  assert.equal(signalExitCode('SIGINT'), 130);
  assert.equal(signalExitCode('SIGTERM'), 143);
  assert.equal(signalExitCode('SIGWHAT'), 1);
  // The stop is recorded on the attempt that held the computer, too.
  assert.deepEqual(i.attempts[0].computer_stop, { status: 201, state: 'stopped' });
});

test('K5 stranded: a killed run and an interrupted run whose stop failed', () => {
  const killed = inProgressReceipt(base(), '2026-10-15T22:02:00.000Z');
  const failedStop = interruptedReceipt({ ...base(), label: 'go-served-y', attempts: [{ attempt: 1, computer_id: 'computer-def' }] },
    'SIGTERM', '2026-10-15T22:03:00.000Z', { status: 502, state: null });
  const out = strandedTracerComputers([
    { label: 'go-served-x', receipt: killed },
    { label: 'go-served-y', receipt: failedStop },
  ]);
  assert.deepEqual(out.map((x) => [x.label, x.computer_id, x.reason]), [
    ['go-served-x', 'computer-abc', 'killed'],
    ['go-served-y', 'computer-def', 'stop_failed'],
  ]);
});

test('K6 not stranded: stopped, finished, or never minted', () => {
  const stopped = interruptedReceipt(base(), 'SIGTERM', '2026-10-15T22:03:00.000Z', { status: 201, state: 'stopped' });
  const finished = { ...base(), in_progress: false, exit_code: 0, attempts: [{ attempt: 1, computer_id: 'computer-abc', computer_stop: { status: 201, state: 'stopped' } }] };
  const noMint = inProgressReceipt({ ...base(), attempts: [{ attempt: 1, phases: [] }] }, '2026-10-15T22:02:00.000Z');
  const legacy = { schema: 'choir.reeng_tracer_receipt.v1', label: 'old', exit_code: 0, computer_id: 'computer-old', attempts: [] };
  assert.deepEqual(strandedTracerComputers([
    { label: 'a', receipt: stopped }, { label: 'b', receipt: finished }, { label: 'c', receipt: noMint }, { label: 'd', receipt: legacy },
    { label: 'e', receipt: null },
  ]), []);
});

test('K7 a kept computer is not stranded', () => {
  const kept = { ...inProgressReceipt(base(), '2026-10-15T22:02:00.000Z'), keep_computer: true };
  assert.deepEqual(strandedTracerComputers([{ label: 'k', receipt: kept }]), []);
});

test('K8 a two-computer stop is stopped only when both stopped', () => {
  const both = interruptedReceipt(base(), 'SIGTERM', '2026-10-15T22:03:00.000Z', { a: { status: 201, state: 'stopped' }, b: { status: 201, state: 'stopped' } });
  const half = interruptedReceipt(base(), 'SIGTERM', '2026-10-15T22:03:00.000Z', { a: { status: 201, state: 'stopped' }, b: { status: 502, state: null } });
  assert.deepEqual(strandedTracerComputers([{ label: 'both', receipt: both }]), []);
  assert.deepEqual(strandedTracerComputers([{ label: 'half', receipt: half }]).map((x) => x.reason), ['stop_failed']);
});
