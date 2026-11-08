# The enforce flip for the built-in kinds (F6), 2026-11-08

Status: r0, design for review. Goal: docs/definitions/choir-modular-architecture-2026-11-04.md, F6.
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
