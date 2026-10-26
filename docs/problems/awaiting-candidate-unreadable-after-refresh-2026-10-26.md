# An Awaiting Candidate's Bundle Reads as a Binding Mismatch After a Refresh

Date: 2026-10-26. Computer: b@c's test computer
(computer-6c50d6e566143ae32cdb660faa60e24d). Problem first; no fix in the
commit that writes this record. Status: open, cause a hypothesis.

## What happened

- 00:21:41 FP0 submitted the Reader redesign (Texture doc 98d0a421,
  trajectory 4e7fd334). Engineering ran in a claude-mode capsule (the
  Claude delegate's first run on b@c; lifecycle network_mode "claude").
- 00:33:25 the deploy of a03d5266a restarted vmctl (the PD3 F6 label).
- 00:34:37 operation selfdev-aae30fb75aa73afab4e6897823facfdc created
  (frozen 00:34:59 by FP0's watcher), verify passed, awaiting_approval by
  00:36:00 (bundle_digest ed0ee483..., base_head fbf1c423...).
- 00:36:35 b@c booted onto a03d5266a (vmctl's deferred refresh: the
  computer was idle once the candidate awaited approval).
- About 00:37:30 `choir self-dev patch` and `choir self-dev visual` for
  the operation both answer 409 "frozen bundle binding mismatch"
  (internal/agentcore/api_self_development_patch.go,
  readBoundSelfDevelopmentBundle: the bundle fails to decode with
  DisallowUnknownFields, or its digest or its computer, trajectory,
  capsule and base head do not match the operation).
- The same boot reports the source projection as stack_base_moved,
  diverged true, for the old reader-6 operation selfdev-1c1f6265 (base
  6113c82cd), which reported upstreamed before.

## Why it matters

The owner's approver cannot read the candidate's diff or screenshots, so
the candidate cannot be judged, and approving blind is not allowed. Any
deploy that lands while a candidate awaits approval may do the same.

## Hypotheses (unconfirmed)

1. The refresh moves or drops the final bundle.json the patch API reads,
   so the reader falls back to bundle.draft.json, whose content digest is
   not the final file's digest.
2. A claude-mode bundle carries a field or binding the reader does not
   expect (first claude-mode candidate read on b@c).
3. The operation's base head or capsule id is rewritten at boot.

Next: a read-only code investigation of where bundleRoot points, what a
guest refresh does to it, and what a claude-mode Freeze writes.

## Investigation (00:5x, read-only, Sonnet; confirmed from code unless marked)

- bundleRoot is `/mnt/persistent/choir-updater/incoming/<BundleDigest>`
  (api_self_development_patch.go:65, api_self_development_visual.go:89;
  nix/autoputer-vm.nix:121,889), on the VM's persistent data.img. A
  refresh kills Firecracker (vmmanager/receipts.go:56, proc.Kill) and
  boots the same data.img (manager.go:1731-1747). The 409 branches sit
  after the existence and read checks, so the directory and a file
  existed; one message covers three different failures (strict decode,
  digest, identity), so the 409 cannot say which.
- No boot step touches `incoming/<digest>` for an awaiting_approval
  operation; no boot step rewrites BaseHead, CapsuleID or BundleDigest;
  claude-capable capsules write the same bundle struct (hypothesis 2 and
  3 have no code path).
- Leading hypothesis: bundle.json is written with a plain WriteFile and
  the directory renamed with no fsync (tools_capsule.go:533,542), and the
  VM was SIGKILLed 35 to 40 seconds later, about the ext4 dirty-expire
  window, so the file on disk is short, empty or stale. Not confirmable
  without reading the guest disk, which FP0 does not do.
- Approving would fail the same way: the materializer
  (self_development_materializer.go:320-333) reads bundle.json only and is
  stricter than the reader, and its reconcile re-arms a 60 s retry.

## Fix shapes

1. fsync bundle.json, the draft and source.patch, and their directory, in
   Freeze and finalize.
2. The guest syncs the filesystem after it writes the planned-restart
   marker, before vmctl kills it (a refresh is a hard kill; anything else
   written without fsync is exposed the same way).
3. The reader names which check failed (decode, digest, identity).
4. Optional: restore bundle.json from the tape artifact whose digest is
   the operation's BundleDigest.

Disposition of selfdev-aae30fb7: rejected with the reason, and the round
is repaired by a fresh freeze.
