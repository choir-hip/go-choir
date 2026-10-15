# No capture records when it first entered Store B relative to a computer's tape

Found 2026-10-15 while making L3's ordering witness concrete
([L3 station](../definitions/choir-ledger-l3-hypotheses-settled-2026-10-13.md),
witness (b); [review record](../evidence/l3-file-review-2026-10-15.md)).

## The problem

L3 scores a forecast only on evidence captured **after** the forecast was
committed. The forecast is an inbox record on the paper's hash-chained
computer event tape. Captures are not on that tape.

- Captures are shared content, written to Store B (corpus Dolt) by corpusd:
  - `POST /internal/platform/captures`;
  - `ObjectGraphHandler.HandleCaptures` (internal/platform/captures.go);
  - `sourcegraph.WriteCaptures`;
  - `CreateWebCaptureIfAbsent`, which does `INSERT IGNORE INTO og_objects`
    (internal/platform/objectgraph_store.go).
- The paper's tape holds only the later `CaptureArrival` announcement
  (internal/agentcore/capture_arrival.go). That arrival can lag the
  capture: a failed append leaves the capture on the schedule's pending
  list. Another computer, or an earlier poll, may also have captured the
  same document first.

Nothing in Store B witnesses when a capture first entered it, relative to
any computer's tape:
- `created_at` and `fetched_at` are `time.Now()` on whichever host wrote
  them. Timestamps may exclude evidence but never admit it.
- Store B's Dolt commits are debounced, not one per write
  (internal/platform/store.go `markDirty`). The planned weekly squash
  would also erase their order.

So today a settlement cannot show that its evidence postdates the forecast.
L3's forecast cohort cannot start until this exists.

## Decided design (panel 3 of 4, 2026-10-15)

The decision is D3, a per-computer causal anchor. The panel record is in
docs/evidence/l3-file-review-2026-10-15.md, "Witness (b) design". corpusd
already runs the computer event CAS in the same process
(`cmd/corpusd/main.go`: `svc.ComputerEventRuntime()`, issuer `corpusd`,
with a signing key), so the anchor needs no new network hop.

1. In `HandleCaptures`, after `requireInternalOrBoundGuest` authenticates
   the writer, read the writing computer's committed canonical head
   (`ComputerEventCAS.Head`). Read it **before** any insert, once per
   request.
2. Pass the anchor into `CaptureWriteConfig`. On the **winning** first
   insert only (`created == true`), store a host-signed witness in the same
   row: `choir.capture_first_creation.v1` carrying capture_id, doc_hash,
   writer_computer_id, anchor_sequence, anchor_event_head,
   anchor_receipt_digest, issuer, key_id and signature.
3. A repeat or a losing racer never gets a witness, and never replaces
   one.
4. Return the witness in `CapturedItem`. Keep it on the pending-arrival
   list, and carry it in `types.CaptureArrival`. That changes the frozen
   `CaptureArrival` declaration, so it needs a compatible-extension entry
   in the Cut 2 freeze receipt.
5. A settlement copies the witness and the chain witnesses it relied on.

The ordering argument is: Cast committed → corpusd reads a head that has
the Cast as a strict ancestor → first insert → CaptureArrival. A capture
counts only when:
- the writer is the paper;
- the Cast is a strict ancestor of the anchor head;
- the first arrival follows the Cast (witness a).

Reading the head *after* the insert is unsound. The sequence insert → Cast
→ read → delayed arrival would admit old evidence.

## Ways it could fail (write the tests against these)

- **W1:** the head is read after the insert, or the anchor is attached to
  a repeat or a losing racer.
- **W2:** a failed head read or signature fails the capture write. It
  must store the capture without a witness: unscorable, never filled in
  later.
- **W3:** a later `PutObject` upsert or `DeleteObject` followed by a
  re-insert gives a capture a newer witness. The witness must be
  immutable, and a tombstone must survive deletion.
- **W4:** the witness is not bound to its capture and writer, so it can
  be replayed onto another capture.
- **W5:** the pending list or `CaptureArrival` drops the witness on a
  retry.
- **W6:** `CaptureArrival` changes, so a retried arrival's command
  digest changes. The witness must be a pure function of the first
  write, carried as returned.
- **W7:** the off-host copy keeps hashes but not the signatures,
  verification keys or chain artifacts needed to verify offline.
- **W8:** replay or checkpoint rebuild mints a witness from today's head.
  Witnesses come only from live host writes.

## Mutation class

Red: Store B capture writes, host signing, the frozen `CaptureArrival`
declaration. It needs the full ceremony and a code review panel before it
lands. The capture write path must stay available: a witness failure is
never a capture failure.

## Owner decision needed before building: the signing authority

corpusd holds one signing key, the event CAS's `platform-control` key
(`NewComputerEventCAS` requires `SignerDomain == "platform-control"`;
`computerevent.SigningKey`, internal/computerevent/receipt.go). The
witness needs a host signature. Two choices:

- **(i) Reuse that key under a distinct statement domain**, such as
  `capture-first-creation`, carried inside the signed bytes. That means no
  new key material, but one key then attests two kinds of statement. A
  verifier must check the domain, or one statement could pass for the
  other.
- **(ii) Mint a dedicated capture-witness key**, with its own key id and
  rotation. That widens the trust surface less, but adds key material,
  provisioning on Node B, and a public key to publish with the off-host
  copy.

This is a credential and authority choice, which the operating contract
reserves for the owner. It blocks only witness (b)'s code. The L3-1 clock
is already live, and the forecast cohort waits on witness (b) either way.

## Status

Documented. Not built. Waiting on the signing-authority decision above.
