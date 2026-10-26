// node --test scripts/checkpoint_pick_test.mjs
//
// Ways the proof tooling could fail after the manual checkpoint bind is gone,
// written before the code:
//  - a not_derivable checkpoint (restore refuses it) is picked as the target;
//  - the oldest listed checkpoint is picked, not the newest;
//  - an empty listing is read as a target;
//  - the fetched artifact is the response envelope, which restore does not
//    take as `checkpoint`, or a response without a checkpoint is accepted;
//  - the restore predicate demands the pre-apply head equal the pinned head
//    (a cadence checkpoint is usually a few events older), or passes a restore
//    of some other checkpoint, or one the apply did not move past.
import test from 'node:test';
import assert from 'node:assert/strict';
import { pickNewestCheckpoint, checkpointArtifact, restoredToPinnedCheckpoint } from './checkpoint_pick.mjs';

const row = (digest, status, extra = {}) => ({ checkpoint_digest: digest, accepted_sequence: 10, verdict: { status }, ...extra });

test('picks the newest checkpoint', () => {
  const got = pickNewestCheckpoint({ checkpoints: [row('b', 'verified'), row('a', 'pending')] });
  assert.equal(got.checkpoint_digest, 'b');
});

test('skips a not_derivable checkpoint, which restore refuses', () => {
  const got = pickNewestCheckpoint({ checkpoints: [row('c', 'not_derivable'), row('b', 'unverified'), row('a', 'verified')] });
  assert.equal(got.checkpoint_digest, 'b');
});

test('unjudged, pending and unverified checkpoints are targets', () => {
  for (const status of ['unjudged', 'pending', 'unverified', 'verified']) {
    assert.equal(pickNewestCheckpoint({ checkpoints: [row('x', status)] })?.checkpoint_digest, 'x', status);
  }
});

test('an empty or malformed listing is no target', () => {
  assert.equal(pickNewestCheckpoint({ checkpoints: [] }), null);
  assert.equal(pickNewestCheckpoint({}), null);
  assert.equal(pickNewestCheckpoint(null), null);
  assert.equal(pickNewestCheckpoint({ checkpoints: [row('c', 'not_derivable')] }), null);
});

test('the restore operand is the Checkpoint inside the response', () => {
  const checkpoint = { checkpoint_digest: 'a', request: { effective_event_head: 'h' } };
  assert.deepEqual(checkpointArtifact({ checkpoint, receipt: { signature: 's' } }), checkpoint);
  assert.equal(checkpointArtifact({ receipt: {} }), null);
  assert.equal(checkpointArtifact(null), null);
  assert.equal(checkpointArtifact({ checkpoint: {} }), null);
});

const claim = (over = {}) => ({
  restore: { status: 200, witness_matched: true, frontend_restaged: true, checkpoint_digest: 'a' },
  pre_apply_checkpoint: { checkpoint_digest: 'a', effective_event_head: 'h1' },
  applied: { effective_head: 'h2' },
  ...over,
});

test('the restore leg passes for a pinned checkpoint older than the pre-apply head', () => {
  assert.equal(restoredToPinnedCheckpoint(claim()), true);
});

test('the restore leg fails for another checkpoint, a mismatch, or an apply that did not move the head', () => {
  assert.equal(restoredToPinnedCheckpoint(claim({ restore: { ...claim().restore, checkpoint_digest: 'z' } })), false);
  assert.equal(restoredToPinnedCheckpoint(claim({ restore: { ...claim().restore, witness_matched: false } })), false);
  assert.equal(restoredToPinnedCheckpoint(claim({ restore: { ...claim().restore, frontend_restaged: false } })), false);
  assert.equal(restoredToPinnedCheckpoint(claim({ restore: { ...claim().restore, status: 409 } })), false);
  assert.equal(restoredToPinnedCheckpoint(claim({ applied: { effective_head: 'h1' } })), false);
  assert.equal(restoredToPinnedCheckpoint(claim({ pre_apply_checkpoint: null })), false);
  assert.equal(restoredToPinnedCheckpoint({}), false);
});
