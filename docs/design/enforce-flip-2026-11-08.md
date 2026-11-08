# The enforce flip for the built-in kinds (F6), 2026-11-08

Status: r3 (r0 sent back by Devin and a Sonnet seat, r1 and r2 by Devin; §6, §7 and §8 amend it and govern where they differ). Design for review. Goal: docs/definitions/choir-modular-architecture-2026-11-04.md, F6.
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
