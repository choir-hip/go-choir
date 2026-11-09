# The enforce flip for the built-in kinds (F6), 2026-11-08

Status: r4 (r0 sent back by Devin and a Sonnet seat, r1 to r3 by Devin; §6 to §9 amend it and govern where they differ). Design for review. Goal: docs/definitions/choir-modular-architecture-2026-11-04.md, F6.
Inputs: the prerequisite map (scratchpad f6-prereqs, 2026-11-08), ledger 2026-11-05T17:30Z and
2026-11-06T01:50Z, and a fresh read of `GET /api/runtime/observability` on d@e at 6eb19408e
after a day of drills (two accepted package installs, one rejected, four engineering rounds).

## 1. What the flip is

Three custody gates run in observe mode for first-party kinds: the object-graph writer gate,
the object-graph reader gate and the disclosure gate. Each one has an in-code mode atomic with
observe as its default and a `Set*Mode` seam (`objectgraph/writer.go:135`, `reader.go:87`,
`disclosure/rules.go:331`). The act gate enforces package desks and observes first-party desks.
Package kinds (`choir.app.*`) already enforce.

The flip has two parts. Every gate enforces for every kind. The observe and off modes, and
their seams, leave the product code. Doctrine (CLAUDE.md, "No switches") forbids a mode that
can turn enforcement back off. Tests that need a gate off use a test-only constructor, never a
package variable that product code can reach.

The flip only tightens. Today the gates count and allow everything. After the flip they allow
what this design names and refuse or ask about the rest. Every allowance here already happens
today, uncounted or counted.

## 2. What the counters say (d@e, 6eb19408e, since its last boot)

The reader gate has seven crossings, all first-party:
- R1 `texture_document` read as management, at `planEngineeringCast` (engineering_assignment_runtime.go:301) and `openDelegatedCastAssignment` (:504). 5 reads each.
- R2 `texture_document` read with no declared reader, at `operationDocument` (selfdev_owner_act_wake.go:57). 5 reads.
- R3 `texture_document` and `texture_revision` read as engineering, at `commitEvidenceEmbeds` (textureowner/texture_evidence_embed.go:145, :165, :247). 3 reads each.
- R4 `citation_check` and `support_verdict` read as engineering, at `calibrationSnapshotFor` (calibration_feedback.go:518, :522). 2 reads each.
- R5 `engineering_lineage` read with no declared reader, at `buildLineageNotice` (engineer_lineage.go:208, :220). 11 reads.

The disclosure gate has 15 asks and no denies, all on the desk-message sink between built-in desks:
- texture to management, cast, owner_private:document (5);
- management to engineering, cast, document with texture, model-policy, computers and prompt-bar (5);
- engineering to management, complete, document with texture and model-policy (4);
- management to texture, report, computers, model-policy and prompt-bar (1).

The writer gate and the route gate have no counts.

Enforcing today would put every self-development cast on an ask. An ask goes to System 1 and,
on doubt, to the owner. That stalls the loop the mission exists to make reliable. It would also
empty R1 to R5. So the flip needs the rules below first.

## 3. Decisions

### D1. The owner-internal flow covers the owner's delegation chain

The owner's 2026-11-06 ruling made texture and research exchange captures and evidence with no
ask: "It's expected that private captures will be used by texture, no approval required or even
no need to notify users. Just keep the provenance."

The counted asks are the same shape one step further down the chain. The owner asks Texture,
Texture casts management, management casts engineering, and the reports come back. Every label
on those messages is the owner's own data. It moves between the owner's own built-in desks
inside the owner's own computer, to serve the owner's own request.

**Decision.** `OwnerFlow` covers desk messages between any two of the built-in texture,
research, management and engineering desks. The host sets `OwnerDesks` only when both ends are
built-in system desks, resolved through the registry by grant (AdmitterName, PerformerName,
ResearcherName) and Texture's kind. Only the verdict changes:
- labels still ride the message, the record and the run;
- publication, mail send, Library release, egress, every package desk and every other sink
  still follow the table.

The label set widens from capture and evidence to every owner_private atom. No public, secret
or credential class is covered. OwnerFlow lifts an Ask and never lifts a Deny.

**Why this is the conservative default.** The alternative is enforcing with asks. Each
self-development round would then wait on a System 1 call and often on the owner. Doctrine
treats too many model calls as a code problem, not a limit to impose. The owner can revoke the
extension by a ruling. The provenance the owner asked for is kept.

**Named edge.** A compromised first-party desk could move owner data to another first-party
desk without an ask. The custody boundary that matters is the computer's egress and its
publication sinks, which still enforce. Package desks never get OwnerFlow.

### D2. Host reads made on a desk's behalf are attributed correctly

R1 to R5 are host code reading a kind while the run context names a desk. Each gets the fix
that matches what the read is:
- **R1 (planning a cast).** Management plans the engineering cast from the document the owner's
  request names. The read is the custodian's: Texture's cast record carries the packet. Fix: the
  planner reads the document under the Texture custodian identity
  (`objectgraph.WithReader(WriterTexture)`), and only for the document the cast record names. A
  document the cast does not name stays refused.
- **R2 (`operationDocument`, an owner act).** The owner act names the operation, and the
  operation names its document. Fix: read as the Texture custodian, the same as R1.
- **R3 (`commitEvidenceEmbeds`).** This is Texture's own handler. The engineering attribution is
  inherited from the caller's run context. Fix: the handler reads as Texture. That is a
  correction, not a grant.
- **R4 (calibration snapshot).** Engineering's digest-arm run reads the calibration verdicts
  about its own work. Fix: add engineering as a supplemental reader of `citation_check` and
  `support_verdict` through the registry rule, so it is counted as declared.
- **R5 (`engineering_lineage`).** The kind has no declared reader. Engineering wrote it and
  reads it for its own lineage notice. Fix: declare engineering as its custodian reader in the
  registry.

### D3. The known prerequisite gaps close as mapped

These close as the prerequisite map says:
- `content_item` evidence refs: Texture is a supplemental reader. `content_item` is the shell
  custodian's text, which Texture already renders.
- `agent_evidence` refs: Texture is a supplemental reader.
- `owner_private:evidence` maps to `choir.agent_evidence`. Every label atom is checked against
  the registry by a test.
- The lease loop in `messageAdmitted` becomes `rt.leases.AdmitsKind(writer, runID, kind)`, so
  it is run-bound, expires and counts use.
- `capture_read_gate.go` uses the shared `labelObjectKinds` mapping.

### D4. The paper's publish of a publish-auto story

A story whose spec says `publish: auto` is a standing owner intent ("we need paper publishing
24/7"). Its captures are public web captures. They carry the owner_private:capture floor label
only because they pass through the owner's capture store.

**Decision.** The publication sink admits the document custodian's publish of a story whose
spec carries `publish: auto`, for the capture label only. Any other owner_private label on a
publish still asks.

This decision is the most owner-facing in the design, and the report names it for the owner.

### D5. The flip itself

- The writer, reader and disclosure gates enforce for every kind.
- `WriterGateOff`, the observe modes and the `Set*Mode` seams leave product code. Tests use a
  test-only fixture.
- The act gate enforces first-party desks as well.
- The observability route keeps its counters and reports `mode: enforce`. A refusal is counted,
  and its site is listed.

## 4. Order and proof

1. Land D2 and D3 with the gates still observing. Prove on d@e that a full owner round (Texture,
   research, management, engineering, with a package install review) leaves the reader gate
   empty.
2. Land D1 and D4. Prove on d@e that the disclosure gate shows no ask on that round.
3. Land D5. Re-run the round on d@e in enforce, and run a planted cross-read (a package desk
   reading another kind, which is already refused) to show refusal still bites. Rings carry it
   onward, with the paper last.

Each step is red. Steps 1 and 2 are tier 2, because Texture reads and disclosure decisions fail
silently. Step 3 is tier 2: it deletes modes and changes boot behaviour for every computer.
Rollback for each step is a revert. A revert of step 3 restores observe.

## 5. Failure modes the build must test first

- FM1: OwnerFlow is set for a package desk, or for a built-in desk whose grant moved to a package.
- FM2: OwnerFlow lifts a Deny, or covers a non-desk-message sink.
- FM3: R1 or R2 reads a document the cast or operation does not name.
- FM4: R3's Texture identity leaks into a non-Texture caller.
- FM5: D4 admits a publish that is not publish-auto, or a non-capture label.
- FM6: After D5, product code can still reach an observe mode.
- FM7: A lease admitted after its expiry or outside its run (D3 lease loop).
- FM8: An unmapped label atom. The registry test fails on it.

## 6. r1 amendments (Devin r0 and the Sonnet seat r0, both SEND BACK)

The r0 review found that F6 is larger than one flip. Paths d@e never exercised would ask or
refuse at enforce: the paper's intake delegation, every publish-auto revision, lease-admitted
messages, the act gate and several reads made on background contexts. The design is now staged
so that each stage is proved by the counters before the next one runs.

### 6.1 Scope

F6 lands three stages:
- **S1:** admissions and attributions, with the gates still observing.
- **S2:** a recorded clean round on d@e.
- **S3:** the flip of the reader, writer and disclosure gates.

Enforcing the act gate for first-party desks is **out of F6**. Under enforce, every I2 or higher
act of a first-party desk needs a fresh per-class trust record, or it waits on an owner tap.
`gateAct` hard-codes thirdParty=true, so I4 acts (capsule.mutate, platform.update, product.api)
would never be autonomous. That would stall self-development, and the trust records it needs do
not exist yet.

The act gate keeps its current mode: enforce for package desks, observe for first-party desks.
It is a named residual with its own design. The route gate is already enforced where it applies,
which is package kinds (routekinds.Enforced), so it needs no change here. Its observability
label, hard-coded to "observe" (api.go about line 1024), is corrected to report what it does.

d@e act-gate counts at 6eb19408e were empty, read from the same observability response as §2.

### 6.2 D1, amended: OwnerFlow is bounded

OwnerFlow lifts an Ask on the desk-message sink only. Before r1 it covered capture and evidence
between Texture and research. In r1 it covers exactly these directed pairs:
- texture to and from research (unchanged);
- texture to management, and management to texture;
- management to engineering, and engineering to management;
- management to its own task role (management-task), and the task back to management. This is
  the intake delegation. The pair is resolved through the manifest: a KindRole's parent desk is
  the admitter.
- management to and from research (added 2026-11-09, commit 060ab0f22: the owner ruled on
  2026-11-06 that owner-internal desk flow needs no approval, and a management cast for evidence
  parked on an ask).

It carries exactly these labels: capture, evidence, document, texture, model-policy, computers
and prompt-bar. These are the counted atoms plus the two the owner ruled on.

Nothing else is covered:
- not texture to engineering directly;
- not research to engineering;
- no other atom (mail, run memory, story and every future atom still ask).

Every end is resolved through the registry by grant and kind, never by a literal name. The rule
is an extension of the owner's 2026-11-06 ruling along the owner's delegation chain. It is named
as such in the owner report and is revocable by a ruling.

### 6.3 D4, amended: publish-auto admission, provenance-bound

The publication sink admits a publish only when all of these hold:
- the story's publish-auto spec traces to an owner-authorized origin. That is either an owner
  act, or the cast that opened the story was sent by the admitter desk (management) under an
  owner source schedule, an owner standing intent recorded on the tape. A publish-auto spec set
  by any other caster is not admitted;
- the publishing actor is the document custodian;
- the record's labels are a subset of {owner_private:capture, owner_private:document,
  owner_private:evidence, world_untrusted}. These are the labels a publish-auto Texture run
  carries by construction: its frame shows the story document, and its evidence derives from the
  captures. Any other label still asks.

**Named edge.** An owner-initiated capture of a private page carries the same capture floor atom
as a public web capture. A publish-auto story whose source schedule polls such a page would
publish its captures. This is the owner's own schedule choice, and the report names it.

S2 must show a real publish-auto revision reaching the publication sink with no ask.

### 6.4 D2, amended: forced custodian reads

`withKindReader` and `asTextureReader` use WithReaderIfAbsent, so they do nothing under a desk's
run context. That is why R1, R3 and R4 counted the desk and not Texture.

S1 adds `objectgraph.WithCustodianReader(ctx, kind)`, a helper that always sets the reader. It
resolves the reader through CustodianOf(kind), with no desk literal. It is applied to the single
read call only:
- **R1:** the GetLifecycleDocument call in planEngineeringCast and in openDelegatedCastAssignment,
  never the whole OpenEngineeringAssignment;
- **R2:** operationDocument;
- **R3:** the reads inside commitEvidenceEmbeds;
- **R4:** the calibration snapshot reads of citation_check and support_verdict. This covers
  management too: management holds calibration.read and builds the same cached snapshot. There is
  no standing supplemental grant.
- **R5:** buildLineageNotice declares the engineering reader on its own context
  (withDeskReader), because engineering is already the custodian of engineering_lineage. There is
  no registry change.

### 6.5 D3, amended: the lease check matches the recipient

The lease check matches on the recipient desk, the owner, the expiry and the run-or-durable rule.
The sender's runID is in scope at messageAdmitted, but the lease is held by the recipient's run,
so matching the sender's run would never admit anything. FM7 gains the cross-run case.

The other D3 items stand as written in r0:
- the content_item and agent_evidence supplemental readers;
- the label atom mapping;
- the shared label-to-kind mapping in capture_read_gate.

FM8 is scoped to the atoms the host mints from the registry. productAPIObjectKind atoms are path
segments, and they fall to the table (they ask), not to OwnerFlow.

### 6.6 Capture reads

The built-in branch of capture_read_gate counts and serves (it has no mode). r1 makes it an
explicit read rule matching the owner's ruling: a built-in desk may read an owner-private
capture. The KindApp split stays, so package desks are refused. The branch stops counting
wrong_reader for built-in desks, because that read is allowed by rule, not merely observed.

### 6.7 Background-context reads

S1 audits every context.Background(), WithoutCancel and subCallContext consumer that reaches a
non-commons kind, and declares its reader. Known sites:
- the sub-call settler's GetEvidence (subcall_judge.go:142 and :397);
- ReleaseEngineeringSlotClaim (runtime.go:1758);
- the legacy wake fallback (actorruntime/adapter.go:311 and :316);
- rebaseRoot (rebase_notice.go:579), which must use the forced helper if it is called on a run
  context.

The audit's list goes into the S1 ledger line.

### 6.8 S3 test mechanism

Product code may not reach a gate mode. S3 does three things:
- It replaces the three package atomics with fixed enforce behaviour.
- It moves the fixture seam into a new package, internal/objectgraph/gatefixture. Its only export
  is `Observe(t testing.TB)`, which takes a testing.TB, sets the gates to observe for that test
  and restores them on cleanup.
- It adds a gate script, check-gatefixture-imports, that refuses any import of gatefixture from a
  non-_test.go file.

Fixtures that open a store with no reader declared either declare one or call
gatefixture.Observe. The migration is part of S3. The proof script
scripts/proofs/fp4-selfdev-admission.sh:194 and the three-mode test in
stream_seq_lease_fence_test.go are updated in S3.

### 6.9 Proof (supersedes §4)

**S1** lands with the gates observing (tier 2). Its proof on d@e is one round of each:
- an owner Texture request that casts research;
- a self-development round with a package install review;
- an intake round of a source schedule;
- a publish-auto story revision;
- a crash restart (d@e only).

S1 passes when the reader gate and the disclosure gate show no counts outside the rules above.
The act gate is read and reported.

**S2** is the counter read after S1's round: a recorded, clean round.

**S3** lands only after a clean S2 (tier 2). Its proof:
- the same round in enforce, with zero refusals;
- a planted cross-read by a package desk, which is still refused;
- a crash restart under enforce on d@e.

Rings carry each stage onward, with the paper last. Rollback for each stage is a revert. A revert
of S3 restores observe.

The paper's intake and publish are proved on d@e under its own source schedule and story, never
on the paper itself.

## 7. r2 amendments (Devin r1 SEND BACK)

### 7.1 The S3 test seam, made consistent (replaces §6.8's mechanism)

The three gates keep an unexported mode. Product behaviour is enforce, with no default and no
setter on any product path. Each gate package exports exactly one seam,
`SetModeForTest(mode) (restore func())`. That seam calls `testing.Testing()` and panics when it
returns false. A product binary that calls it dies at the call; it cannot run with a gate off.

`internal/objectgraph/gatefixture.Observe(t)` wraps the three seams and registers the restore
with `t.Cleanup`. The gate script `check-gatefixture-imports` refuses two things in any non
_test.go file:
- an import of gatefixture;
- a call to any `SetModeForTest`.

The panic is the runtime guard, and the lint is the static one.

### 7.2 Capture reads, bound to the ruling (replaces §6.6)

The built-in branch of capture_read_gate allows the read of an owner-private capture without a
count only for desks that already hold capture authority:
- Texture's kind (the owner ruling);
- the research desk and its child roles (the exchange the ruling covers);
- any desk holding `captures.read` or a LeaseCaptures ceiling. Management-task is one.

Every other built-in desk, engineering included, keeps counting wrong_reader in S1 and S2. Those
reads are refused at S3. Package desks stay refused, as today.

### 7.3 Where a role's parent comes from (§6.2)

The manifest gains a `Parent` field on KindRole rows. Management-task names the admitter grant
(`admit.assignment`), and research child roles name `research.spawn`. Parentage is resolved by
grant holder, never by a name prefix. outwardSender keeps stamping a task's outward cast with the
parent desk. This is why §6.3's "sent by the admitter desk" holds on the delegated path.

### 7.4 The D4 provenance walk (§6.3)

The check reads only tape facts:
1. It starts from the story's trajectory: StoryRefOpeningRecord and StoryRefOriginDesk
   (types/story.go:35-38).
2. It then reads the opening cast record and its sender, which must be the admitter grant holder.
   From the cast it takes RelatedIDs and Cast.Captures, then the capture records' SourceID.
3. Last it reads the source schedule record. Its writer must be the owner, through an owner act
   recorded on the tape.

An owner act that sets publish-auto directly also qualifies. Any break in the walk refuses the
admission, and the publish asks as before.

### 7.5 Truncated unfiltered reads

At enforce, readableObjects drops non-admitted kinds from an unfiltered desk read, and each drop
is counted (reader.go:295 records a reader violation). The caller sees a shorter result, not an
error. S1 and S2 read those drop sites from observability like every other count. Any drop site
on the proof round blocks S3.

### 7.6 FM additions

- FM5 gains the caster case: a publish-auto spec set by a caster that is not the admitter and not
  on an owner schedule is not admitted.
- FM9: a built-in desk outside §7.2's set reads an owner-private capture. It is counted in S1 and
  S2, and refused in S3.
- FM10: product code calls SetModeForTest. The lint refuses it, and at runtime it panics.

## 8. r3 amendments (Devin r2 SEND BACK)

### 8.1 The provenance walk's terminal fact (replaces §7.4 step 3)

An object does not persist its writer, and the custodian rewrites a schedule on every poll. So
"its writer must be the owner" names no fact on the tape.

The terminal fact is this. A source schedule exists under the owner's canonical id, and a schedule
can be created only through the owner-authenticated schedule route (HandleSourceSchedules). Its
existence under that id is the owner-act evidence.

S1 hardens this. At creation the route stamps `created_by_owner_act: <request id>` and its time
into the schedule's metadata. Polls carry the stamp forward unchanged, and the walk requires it.
A schedule created before S1 carries no stamp. It is admitted by existence under the owner id
alone, the existing fact, and the report names that.

### 8.2 The Parent field joins the desk protocol lock

Parent decides an OwnerFlow pair, so it bears authority. It joins `deskprotocol.DeskRow` and its
Equal set. S1 writes lock 16.0.0. The lock bump is part of S1's scope and its review.

### 8.3 Staleness

§6.4's `WithCustodianReader` already exists (objectgraph/custody.go:154) with the always-sets
semantics. S1 uses it rather than adding a helper.

## 9. r4 amendment (Devin r3 SEND BACK): an owner act is told apart from a desk's product call

Management holds `product.api` and is the custodian of `choir.source_schedule`. A desk's
ProductAPI call reaches HandleSourceSchedules with the owner's id in X-Authenticated-User. So the
route alone cannot evidence an owner act.

The substrate fact that separates them already exists. An in-process ProductAPI call carries an
empty RemoteAddr: the guest's in-process marker (tools_product_api.go:76-80), read by
`server.InProcessCaller`. An owner request arrives through the proxy with a real remote address.
(`server.GuestHostCaller` is not the test: it also admits the host peer the proxied owner request
comes from, so it is true for the owner as well.)

Replaces §8.1's stamp rule:
- The schedule route writes `created_by_owner_act` only when the request is not an in-process
  caller (`server.InProcessCaller`: empty RemoteAddr). A desk-created schedule carries no stamp, and polls and desk updates never add one.
- The publish-auto walk requires the stamp. A desk-created schedule, and a schedule created before
  S1, are not owner provenance. Their stories' publishes ask.
- S1 adds an owner act, `POST /api/autopaper/sources/schedules/<id>/affirm`. It also refuses an
  in-process caller. It stamps an existing schedule, so the owner can affirm a pre-S1 schedule
  once. The paper's schedules need that affirmation before S3 reaches the paper's ring, and the
  report names it.
- FM11: a desk ProductAPI create or affirm of a schedule writes no stamp, and a story opened on it
  does not admit a publish-auto publish.

## 10. S1 build record: deviations from §1 to §9

- The stamp and the affirm refusal use `server.InProcessCaller` (empty RemoteAddr), not
  `!GuestHostCaller`: the proxied owner request is a host-peer caller, so the §9 wording as first
  written would never stamp an owner act. §9 is corrected above.
- The owner_private/publication table cell is `{Unadmitted: Ask, Admitted: Allow}`, so a host
  admission can lift it. No other cell changed.
- The store refuses any Cast to `texture:new` that is not from the admitter, so the walk's
  sender check is defence in depth, pinned by a direct test.
- R1 at `openDelegatedCastAssignment` reads inside the store's admission
  (`requireEngineeringRecordsAdmission`); the custodian is forced on that one
  `GetLifecycleDocument` call, and the outer `withKindReader` stays (it declares only when no
  actor is named).
- FM8 remaps `evidence` only. `story`, `run_memory`, `app_object`, `mail` and the product-API
  atoms map to no custodian and so still ask; a test pins that.
- `ReleaseEngineeringSlotClaim` is fixed inside `store.releaseEngineeringSlotGraph` (read as the
  slot custodian; write as it where no writer is declared), which also covers the boot
  reconciliation.
- The walk accepts an owner-emitted opening Cast as the origin, and resolves captures through the
  Cast's `RelatedIDs`, else by scanning the admitter inbox (2000 records), because a task's
  outward Cast carries no `RelatedIDs`.
- The pair check folds `-` to `_` after `canonicalDeskName`: the Delegate path addresses
  `management-task:<id>`, while the manifest role is `management_task`. The fold resolves only to
  registered roles.
- Seven existing tests asserted an ask or a count that the owner flow or the new readers remove;
  they were updated, not deleted.

## 11. Two gaps the S1 deployed proof found (d@e e27a0522c)

The reader, writer and act gates were clean. The disclosure gate still asked in two places.

- **The `story` atom.** `owner_private:story` is the label a run takes from reading the story
  index (ActionStories). It was in neither the OwnerFlow atom set (§6.2) nor the publish-auto
  subset (§6.3), so every story-flow message asked (management to texture cast, texture to
  research cast, research to texture report, management to engineering cast) and the one
  publish-auto publication asked on `story` alone, though the provenance walk passed. Fix: `story`
  joins both sets. It is the story document the owner's desks write and, under publish-auto, the
  content being published. FM8 still holds: the atom maps to no custodian in custody, so it is
  admitted only by the flow rule and the publish admission, never by custody.
- **Same-desk messages.** An intake task's outward messages are stamped with its parent by
  `outwardSender`, so the task's report to management reads as management to management, and
  `pair` failed closed on `from == to`. Fix: a message whose two ends resolve to the same built-in
  system desk (Texture, research, the admitter, the performer) discloses nothing to another
  custodian, and OwnerFlow covers it for its atoms only. Package desks, roles and unknown names
  never pair. The representative day gained the task's report to its parent and keeps its
  no-ask assertion.

## 12. Two more sites the S1 deployed proof found (d@e 847ad2417, run 2)

Run 2 passed on the story atom and the same-desk report. Two kinds of site remained.

- **Undeclared reads of `choir.management_task`.** `startQueuedManagementTasks`
  (management_tasks.go:105, :127) and `closeManagementTaskOfEndedRun` (:292) read the task kind on
  a host context with no reader. The §6.7 audit missed management_tasks.go. Fix: every host read
  of the kind in that file (`ListManagementTasks`, `GetManagementTask`) forces the custodian
  through `taskCustodianReads`, whichever desk or none the context carries; the two
  `CaptureDispositionOf` reads (management's kind) are forced the same way. Re-audit of the intake
  files (inbox_records.go, source_schedule.go, capture_arrival.go, event_layer.go,
  event_wakes.go, intake_*.go): the other non-commons reads already declare the custodian
  (`scheduleCustodian`, `withKindReader` on a poll's background context, which declares because
  none is named) or run under management's own run context (story index, where management is a
  supplemental reader; engineering assignments, likewise). `choir.agent` is a commons kind.
- **Management's decline to a source inbox.** The decline goes to `source:<id>`, an inbox class
  (address_resolver.go) that no desk reads; the host's source schedule and the capture's own
  path own it. Fix: a message from the admitter to a registered, addressable, prefixed inbox class
  the admitter may send to (`AdmitterSends`) counts as owner flow, for the OwnerFlow atoms only.
  The class is resolved from the registry of inbox classes, not by a name. The job class is not
  addressable and has no senders, and a fixed address (texture:new) resolves to its desk, so
  neither is touched. The source inbox is never a sender in the pair.

The representative day gained the decline and the slot starter, and keeps its no-ask and
no-reader-site assertions.


## 13. S2a: intake runs clean by construction (S2 on d@e 672efd180, not clean)

### 13.1 What S2 found

The S2 round asked twice at the publication sink on `owner_private:computers`, on the
publish-auto story c1042c39. The provenance walk admitted every other label, so the walk works.
The label came from the substrate, not from a record:

- A desk session's label set only grows. A run is seeded with its session's union
  (tools_desk.go:785), each cell folds back into it (:729), and the session ends only on a restart, a failed
  cell or 30 idle minutes. Management's session key is `agent:management:<owner>`. On a busy
  computer the session never ends, so management's main run carries every label any of its runs
  ever touched. That includes `computers`, which comes from its own product reads of `/api/computers/...`
  for self-development, from Texture directives, and from engineering reports.
- A record carries its sending run's whole set (rlm_reduce.go:1588). A story Texture run reads its
  opening Cast, so it inherits management's accumulated set, and the publication sink reads the
  run's set.
- `choir.Delegate` does not help today. The task's opening Cast carries a model-written objective
  and is stamped with the parent's set. That stamp is correct: the objective is a channel.

On the paper, which is always busy and runs self-development, every publish-auto story would ask
at enforce. S3 would stop the paper. Widening the publish-auto subset to `computers` would admit
whatever management ever read into a public story. That makes the gate decorative on exactly the
computer it exists for, so it is rejected.

### 13.2 Decision

Capture intake does not run in management's main session. The host starts it from the captures
record alone.

1. **Host-started intake task.** When a `system:captures` record is committed to the admitter's
   inbox, the host starts one management task (role management_task) per captures record. The
   slot starter and the K queue are unchanged. No main management run is woken for a captures
   record.
2. **Host-composed opening.** The task's opening Cast has a constant objective, a fixed string in
   code. It also has `Captures` = the record's captures and a reference to the captures record. It carries no
   text a model wrote. Its provenance labels are `labelsOfRecord(captures record)`: the record's
   provenance labels, plus the capture floor, plus world labels for an external origin. It does
   not carry the labels of whatever run is current. Because no run wrote it, nothing can smuggle
   through it.
3. **Fresh session.** The task's desk session key is its own agent id, never
   `agent:management:<owner>`. The build verifies this and tests it. The task's run labels
   start empty. They then grow only by what the task reads: the captures, the story index
   (`story`), and story documents it feeds (`document`). Those are all inside the publish-auto subset.
4. **Delete the model's intake path.** Management's prompt and overlay no longer tell the main
   run to open, feed or decline captures. If intake is the only kind `choir.Delegate("intake")`
   serves, the intake kind is deleted from the verb, with a desk protocol bump. A captures record
   is still shown to the main run's `choir.Records()` as a fact, so management can see intake
   happen. The store's lease keeps the main run from disposing of a capture it does not hold.
   If the build finds the main run can still cast `texture:new` with `publish: auto`, that
   cast is not admitted at publication unless its labels fit the subset. That is the S1 rule,
   unchanged. It asks, correctly.
5. **Texture story sessions.** A story Texture run's session key is per document. The build
   verifies that a story's session is not shared with the owner's other Textures. If it is shared,
   the same creep reaches publication through Texture, and the fix is the same: a story
   document's session is its own.

### 13.3 What this does not cover (named)

- A host-started task that does a product read of `/api/computers` itself picks up
  `computers`, and its stories ask. That is correct.
- An owner directive into a publish-auto story carries the owner's run labels. If those are
  outside the subset, the publish asks. That is correct, and it is the owner's tap.
- Management's main session still creeps. Every other sink it reaches is covered by
  OwnerFlow, and desk messages among the built-in chain carry `computers` as an OwnerFlow atom.
  Per-run scoping of session labels would change the MA3.5 monotone rule, so it is out of F6.
- **The restart rule.** The host starter fires on commit of the captures record, not at boot. A
  crash closes an open task with "interrupted by a restart" and the capture is not re-dispatched.
  The schedule's next tick is new work. A planned restart may resume an open task, as for any
  task today. No boot-time resumer is added.

### 13.4 Proof

- **Representative day.** It gains a management session pre-tainted with `computers` before the
  captures arrive. The intake task's casts, the story Texture run, and its publication must show no ask.
- **FM11.** A management main run tainted with `computers` that casts a publish-auto story
  itself asks at publication. The test proves it.
- **S2 re-run on d@e.** It uses the same round, with a self-development step that leaves
  management's session tainted first. It is clean when the publication sink shows no ask.

Tier 2: this is red, touching the disclosure surface and intake routing. Rollback: revert.

### 13.5 r1 (Devin r0 and the Sonnet seat r0, both SEND BACK)

Both seats confirmed the substrate claims in 13.1. Both found that 13.2 item 4 deleted the only
re-dispatch path for a task's residuals. The Sonnet seat also found that host-start lost event
grouping, that no transaction or backlog rule was stated, and that a cheaper stamp-only option
exists. Devin found that 13.3 misstates restarts: a passivated run closes its task
`run_passivated` (types/management_task.go:168), and tasks are never resumed. On the paper,
which refreshes while busy, residuals are routine. Section 13.2 is replaced by the following.
Sections 13.3 and 13.4 stand, amended below.

1. **Both paths stay, and both are clean.**
   - **Host start on arrival** (Devin: it removes a model turn per arrival and cannot fail to
     fire). The task row and its opening Cast are minted in the same `CommitInboxRecords` batch
     that commits the `system:captures` arrival. The arrival request id is deterministic, so a
     replay collapses to a command conflict. There is no window between the record and the lease,
     and no boot-time resumer.
     - Records of one `EventID` go to one task, mirroring Delegate's grouping
       (choir.go:1026-1059).
     - `Held` arrivals mint nothing.
     - The task mints only captures that are not already disposed or leased, and at most
       `MaxTaskCaptures` per task, mirroring planDelegation (management_tasks.go:386-417).
     - The minted Cast keeps a non-system emitter, or the start wake is minted directly as
       `startNextQueued` does, so the task always starts.
     - A captures arrival that minted a task no longer wakes management's main run. In
       `RecordWakesReader`, the main run is not woken for it.
   - **`choir.Delegate` stays, with a clean stamp** (the Sonnet seat's B4). Management keeps its
     residual, `official_single_outlet`, `unresolved` and crash-closed paths: ask research, then
     re-delegate, or decline. `planDelegation` stamps the task Cast with the union of
     `labelsOfRecord` over the delegated records only, never `r.scope.Labels`. The Cast's
     objective becomes a constant: the model's objective text is dropped from the Cast, the
     prediction and the task row.
     - The remaining model channel is which host-supplied ids to delegate, and how to group them.
       Those are a few bits among uniformly labelled records. This is named in 13.3.
2. **Management never opens or feeds a story itself for a capture.** The prompt and overlay say
   that management disposes of a capture by Delegate or Decline. A main-run Cast to `texture:new`
   or Feed with publish auto still works, but its labels come from management's session, so the
   publication asks (FM11). It is the owner's tap, and correctly so.
3. **Owner settings.** `intakeSurfacesEnabled`, which reads `intake.labels` and
   `intake.event_wakes`, is honored by the host starter exactly as by planDelegation. With intake
   off, the arrival wakes management as today. These are pre-existing owner settings, not
   platform flags; their removal is outside F6 and is named.
4. **Sub-call scoring.** A host-minted Cast carries no `CommitmentSubCall`, so `scoreClosedTask`
   does nothing for it, and the judge observation for host-started intake is lost. A Delegate
   cast keeps its tag. Named.

**13.3 amended.**
- Strike "A planned restart may resume an open task, as for any task today." Tasks are never
  resumed: a planned restart closes an in-flight task `run_passivated`, and its residuals return
  to management, which re-delegates them with the clean stamp. A crash closes the task and counts
  against the capture's crash budget.
- **New named edge: the crash budget moves earlier.** The lease is now taken at arrival, so a
  crash between the arrival and the slot start spends one of the capture's two crashes
  (store/management_tasks.go:508-516).
- 13.2 item 3 already holds today (task session `agent:management-task:<id>`). Item 5 holds
  (`agent:texture:<doc>`). The task role has no `doc.read`, so `document` does not reach it.
  The S1 subset in code includes `story` (publish_auto_admission.go:44), which §6.3 omits.

**13.4 amended.** The representative day adds three cases:
- a residual re-delegated by a tainted management run, with no ask at publication;
- two captures of one event, which open one story;
- a host-started task closed `run_passivated`, whose residual re-delegates.

### 13.6 r2 (Devin r1 SEND BACK)

1. **Mint scope.** The host mints a task only for arrival records a task can fully dispose of:
   plain poll arrivals, corroborated, unresolved and released records. An `event-aged:` record
   (reason `official_single_outlet`, committed unheld at event_wakes.go:202) mints nothing and
   keeps waking management's main run. A task cannot ask research (store/management_tasks.go:333,
   inbox_records.go:532). Management asks research, then re-delegates with the clean stamp, or
   declines. `Held` records mint nothing, as in r1.
2. **No model text on a Delegate Cast.** Dropping the objective is not enough. The Delegate Cast
   also drops the cell's request payload: `in.Req.Questions` must not reach
   `cast.Prediction.Questions` through `applySubCallPrecommitment` (subcall_record.go:94-113) or
   `StampRequest` (request_record.go:358). `cast.SubCall` stays with weight zero, which
   `ValidateSubCallCast` accepts (subcall.go:226). The `firstNonEmpty(objective, spec.Objective,
   in.Objective)` fallback in `typeDelegateCast` (management_tasks.go:557) becomes the constant.
   The verb's surface text (yaegikernel/surface.go:64) drops the ignored `objective` and `req`
   arguments. With the questions gone, `buildSubCallSelfScore` emits nothing for a Delegate
   cast. That is named, beside item 4 of 13.5.
3. **Mechanics the build states.**
   - An arrival that minted a task carries a durable marker on the record, and
     `RecordWakesReader` reads that marker to skip the main run, because the predicate reads the
     record only.
   - A poll arrival over `MaxTaskCaptures` (polls reach 100 items) is chunked into tasks of at
     most `MaxTaskCaptures`, all in the same batch.
   - The minted `TaskID` is deterministic from the arrival's command id, and its provenance
     names the host (Origin, AgentID, RunID empty or host).
4. **Named in 13.3**, correcting r1's stale reference. The id-selection channel on a Delegate is
   which host-supplied capture ids management delegates, and how it groups them: a few bits among
   uniformly labelled records. Backlog: arrival records committed before the deploy mint nothing,
   and they dispose through the main run's Delegate, unchanged. A crash-closed task's residuals do
   not wake the parent, which is a pre-existing behaviour.

### 13.7 S2a build record

Built per 13.5 and 13.6. Deviations and findings, each with its reason:

1. **`choir.Delegate` signature.** The verb is now `Delegate(kind, refs, grouping)`. The surface
   lists exactly those arguments, and `TestSurfaceDocsMatchExportsAndSource` requires the doc args
   to equal the Go parameters, so the ignored `objective` and `req` could not be dropped from the
   text alone. The old 4/5-argument cells no longer compile in the cell. The host-side
   `DelegateSpec.Objective` and `StagedIntent.Req` remain for crafted intents and are ignored by
   `planDelegation`; `TestDelegateCastDropsObjectiveAndRequestQuestions` covers that path, so the
   cell-level test "a cell passed req and an objective" is a staged-intent test.
2. **Weight zero, no stamp.** A Delegate Cast takes no `PSuccess`, `Weight`, `Settlement` or
   questions: `planDelegation` clears `Req` before `stampRequestRecord`, and `typeDelegateCast`
   reads none. `buildSubCallSelfScore` emits nothing for it; the judge still scores "did the
   Report answer the constant objective". Two subcall tests were rewritten to that contract.
3. **Marker.** `CommitmentCaptures.StartedTasks` is a distinct field (never `Held`), and
   `RecordWakesReader` returns `!Held && len(StartedTasks) == 0`. The sweep of held captures
   (`sweepHeldCaptures`) is unaffected.
4. **Mint scope.** Tasks are minted for plain poll arrivals and for corroborated, unresolved and
   released records. `official_single_outlet` (event-aged) and held records mint nothing. A
   `TaskID` derives from the arrival's command id (`managementTaskID("arrival:"+commandID,
   "intake", chunk)`); the Cast's provenance is `Origin: system`, `AgentID`/`Desk` the management
   inbox, `RunID` empty, no `SubCall`, no prediction.
5. **Grouping across commits (design finding).** 13.5 says records of one `EventID` go to one
   task "as Delegate does". Each captures record is its own commit, so records of one event that
   arrive in different flushes (a corroboration record, then a held growth record that management
   delegates later) become two tasks. Grouping applies only inside one commit. One story per event
   now rests on the growth record's `Covered`/`Declined` fields and the story-library head, which
   refuses a stale open. `TestRecordsOfOneEventAreDelegatedToOneTask` now expects two tasks. If
   the owner wants one task per event across commits, a task would have to accept leases while
   open, which the store does not allow.
6. **Residual labels.** `taskReportLabels` no longer folds the delegating run's labels (the
   Cast's `Provenance.RunID`) into the Report. Without that the Report of a Delegate task would
   carry management's taint back into the next delegation. The Cast stamp already carries the
   delegated records' labels.
7. **Existing tests moved.** Tests that assumed an arrival wakes management (`living_stories`,
   `event_wakes`, `capture_arrival_pending`) now assert the host start (`startsOrWakes`), and
   those that feed or decline minted captures cancel the minted tasks first
   (`cancelMintedTasks`).
8. **Not proved in process.** The deployed proof is the S2 re-run on d@e. The in-process
   "no ask" assertion is the publication sink's admission of every owner-private label of the
   story Cast (`publishAutoAdmission`), not a judge call.
9. **Growth pages start their own task (Sonnet seat note 1).** A `grew` page is held and wakes
   no one, and the sweep skips any record with an `Event`. Before this build the corroboration
   record woke management, whose cursor then picked up the growth pages; now corroboration starts
   a task and wakes no one, so a growth page would wait for an unrelated wake. `hostStartsIntake`
   therefore also mints for a held record whose reason is `grew`. The record stays held (the
   `StartedTasks` marker, not `Held`, is what skips the main run), so the sweep is unchanged. The
   task feeds the story named in the page's `Covered`, or opens one, with the library-head check
   as the backstop. No research need, so it is inside the r2 mint scope.
10. **Task ids across groups (Sonnet seat note 2).** The chunk index restarted at 0 per group, so
    a command with two groups minted one id twice. A running counter now spans the groups. A
    planning error (a store read) already falls back to the plain arrival with a log line
    (`commitCapturesRecord`); a refusal at the store's commit still fails the commit, which the
    next poll retries.

## 14. The schedule stamp leaves publication (owner, 2026-11-08)

Owner, on the request to affirm the paper's schedules before S3 reached it: "What is this
complication? The point of the paper is to publish sourced stories asap."

The publish-auto walk's last step, that every source has a schedule carrying the owner-act stamp,
is deleted, along with the affirm route. The stamp answered who set up the polling. The leak the
gate exists for is owner-private data in a public story, and the labels already guard that: the
subset admits only capture, document, evidence, story and world labels. Since S2a, an intake
task's labels are its captures' own, so nothing else management read can ride into a story. The
walk now ends at the captures: every capture the opening Cast names must resolve through the
arrival records to a source, so the story is sourced. An unsourced capture still asks.

The stamp is still written on an owner-route create as a provenance fact. Nothing reads it for
admission. The change ships with Ring-Expedite, so it reaches the paper before S3's own ring walk
does.
