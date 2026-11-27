# Standing triage of 281 documents

Produced 2026-11-27 by a builder lane (station T6 of
[the docs transform](definitions/choir-docs-transform-2026-11-27.md)). **This list is the triage
record, not the source of truth: where a file carries a `Standing:` line, that line wins over
this table.** The list was made from a header-only skim of `docs/design`, `docs/definitions` and
the top level (buckets A governing, B built, C undecided, D superseded, E could not tell), then
each A, D and C file was opened and its basis checked. Buckets B and E are not headed in this
station.

## The first line

One line after the title (after the frontmatter and title for goal files; a YAML comment,
`# Standing: ...`, in the three old-format goal files whose whole body is YAML):

    Standing: <governing|superseded|undecided|built> | kind: <exists|decided|wanted|happened> | [scope: in part |] settled-by: <ruling or outcome and where it is quoted, or nothing> | [replaced-by: <path or none> |] [note: <text>]

Fields are separated by ` | `; the first word after `Standing:` is the standing, so
`grep -rn '^Standing: governing' docs` lists governing documents with their basis. `settled-by`
is an owner ruling (dated, with the line where the words are quoted, or the document that carries
them) or an outcome the author cannot write (a CI gate that reads the file, an entry in
`docs/current-architecture.md`); `nothing` for undecided. `scope: in part` marks a document
superseded only where the named successor conflicts. No `built` was written: that needs an
outcome recorded in `docs/current-architecture.md`, which this station did not check per file.

## Counts

Headed 87: governing 49, superseded 21 (7 of them in part), undecided 17. Unverified, no header:
A 3, D 8, C 10. Not assessed (B 126, E 47).

## Overlap groups

**Vaults.** None governs; all three are undecided. `design/secrets-vault-design-2026-10-28.md`
(draft rev 2, review round 2 pending; a line in its review notes says "design accepted; execution
held by the owner" but no owner ruling is quoted), `provider-vault-and-model-policy-design-2026-10-22.md`
(proposal, not owner-ratified) and `design/mac-device-key-release-2026-11-01.md` (accepted by a
review seat, Devin round 3, not by the owner; it releases a computer's key to the owner's Mac). The
owner said on 2026-11-27 "we need two vaults" (`docs/inbox.md` line 169; `docs/big-picture.md`
item 4): one for the platform's secrets, one for people's credentials. No design is written for the
second; which design, if any, serves the first is the owner's to say.

**Storage.** The mission `definitions/choir-storage-efficiency-2026-11-22.md` governs the work
(owner 2026-11-22, live in ACTIVE). Among designs: `design/one-tape-durability-2026-10-31.md`
governs the tape (owner 2026-10-31); `design/snapshot-one-pass-2026-11-22.md` replaces
`snapshot-without-scratch-copy-2026-11-20.md` (its own line 3; that file is headed superseded);
`design/density-d2-snapshots-survive-deploys-2026-10-26.md` replaces the L20 section of
`fp10-snapshot-levers-and-host-slab-2026-10-22.md` (headed in part); `restore-bases-store-once-2026-11-15.md`
was set aside after three review rounds (headed superseded, replaced by none). The remaining
storage designs are bucket B or E and were not headed, and `store-once-2026-11-14.md` is
unverified. No one storage design governs; the mission names only `paper-compaction` and
`capture-static-copy` among designs.

**The four SwiftUI-era app designs of 2026-11-10** (`autoradio-app-ui`, `autoradio-lab`,
`autoradio-turns`, `autoradio-voice-modes`; all bucket B, not headed). The rule that the phone app
is Exact and only voice is native (CLAUDE.md Product; owner 2026-11-18 and 2026-11-20) governs
where they put logic in Swift. The lab note itself carries an owner ruling of 2026-11-10 ("no
swiftui please. lets do all exact", its line 29). Whether each is superseded, wholly or in its
Swift parts, is not decided by any ruling found.

**Autopaper and engine.** `design/desks-and-engine-2026-11-24.md` governs (owner 2026-11-24; system
map section 10). `ideal-form-draft-2026-11-23.md` and `semantic-layer-2026-11-20.md` are headed
superseded in part by it; `first-design-2026-11-22.md` is governing except where the newer wins;
`first-design-path-2026-11-22.md` is superseded in part. `entity-layer-2026-11-20.md` and
`corpus-first-retrieval-and-research-2026-11-18.md` are headed undecided (their own status: not
ratified).

## For the owner

1. Which vault design, if any, is the platform-secrets vault; whether the people's-credentials
   vault gets a new design.
2. Does `design/store-once-2026-11-14.md` stand? ACTIVE lists it as a live station of 2026-11-14;
   its sibling restore-bases design was set aside; nothing says this one was.
3. Are the four SwiftUI-era app designs superseded by the Exact rule, wholly or only in their Swift
   parts?
4. Is `entity-layer-2026-11-20.md` superseded by the one-kind "subject" ruling (it reads so; not
   verified)? Same question for the entity half of `corpus-first-retrieval-and-research-2026-11-18.md`.
5. Is the apps feature-complete goal (on hold, resumed by wave P8 of the modular-architecture goal)
   still wanted?
6. Is `choir-autoradio-learning-engine-2026-11-14.md` (goal) superseded? ACTIVE lists it live; the
   learning-loop goal of 2026-11-18 was drafted and cleared.
7. The six intent goal files (voice pipeline, records, shipping, operations, desktop, standing
   metamission) are marked governing, kind wanted, on the owner's words in their authority fields.
   Confirm that "an intention recorded from the owner's words" counts as governing.
8. `docs/ACTIVE.md` is marked governing on CLAUDE.md alone; decision D3 of the transform may replace it.

## Where the skim was wrong

D: `choir-selfdev-apps-feature-complete` (on hold, not superseded); `density-d2` (it is the
superseding document); `entity-layer` (a proposal, not superseded by any text found);
`platform-os-app-state` (stale, not superseded); `store-once`, `autoradio-mission-handoff`,
`autoradio-swift-test-map`, `choir-autoradio-learning-engine` goal and `choir-selfdev-readiness`
(no statement of supersession). C: `choir-autoradio-in-the-desktop` carries the owner's words
(governing, wanted); `memo-diagonal-media-strategy` and `signal-is-sparse` are memos, not
proposals; `mac-device-key-release` is undecided only because no owner ruling backs a seat's
acceptance. A: `README`, `choir-prompting-invariants` and the security-density checklist have no
owner ruling or outcome behind them. In bucket A, `autoradio-in-exact` and `first-design` are
governing for the rulings they carry although their status lines say proposed or idea.

## All rows

Skim bucket, path, standing set here, basis, replaced-by, reason when not headed.

| Bucket | Path | Standing | Settled by | Replaced by | Reason |
| --- | --- | --- | --- | --- | --- |
| A | docs/ACTIVE.md | governing | named the live station list in CLAUDE.md Product; the campaign file may replace it (owner decision D3 open) |  |  |
| A | docs/README.md | unverified |  |  | a map of the docs tree; no owner ruling or outcome behind it, so it is not marked governing |
| A | docs/agent-product-doctrine.md | governing | named normative product rules in CLAUDE.md Desks ('Product rules: docs/agent-product-doctrine.md') |  |  |
| A | docs/changes.md | governing | CLAUDE.md Delivery, owner 2026-11-22: 'its hard to catch up on what claude has done'; every landing is entered here |  |  |
| A | docs/choir-doctrine.md | governing | CLAUDE.md: 'Architecture is normative in docs/choir-doctrine.md'; doctrine wins where the two conflict |  |  |
| A | docs/choir-prompting-invariants.md | unverified |  |  | its status line asserts an operating invariant; no owner ruling or CLAUDE.md line names it |
| A | docs/choir-vision.md | governing | named the product vision in CLAUDE.md Product; its status line (line 5) puts it under the doctrine |  |  |
| A | docs/computer-ontology.md | governing | CLAUDE.md Product: read before changing VM, candidate, promotion, package or persistent-state behavior |  |  |
| A | docs/current-architecture.md | governing | CLAUDE.md Product, owner 2026-11-16: read before saying what the system does; a landing that changes a row edits it |  |  |
| A | docs/definitions/choir-autoradio-on-android-2026-11-22.md | governing | owner 2026-11-25, quoted in this file's authority field (line 4); live goal in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md | governing | owner 2026-11-20, quoted in this file's authority field (line 5); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-first-design-2026-11-22.md | governing | owner 2026-11-22 ('great, this is the plan for the next throughline'), authority field line 4 |  |  |
| A | docs/definitions/choir-operations-podcasts-and-providers-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness intent |  |  |
| A | docs/definitions/choir-records-and-measurement-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness executable; live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-shipping-the-app-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-standing-metamission-2026-11-22.md | governing | owner 2026-11-22, quoted in this file's authority field (line 4); named as the run target in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-storage-efficiency-2026-11-22.md | governing | owner 2026-11-22, quoted in this file's authority field (line 4); live in docs/ACTIVE.md |  |  |
| A | docs/definitions/choir-voice-pipeline-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); readiness intent |  |  |
| A | docs/design/account-create-delete-2026-11-23.md | governing | owner rulings 2026-11-23 listed in this file's status note (line 5) |  |  |
| A | docs/design/autoradio-dj-programme-2026-11-11.md | governing | owner 2026-11-11, verbatim (line 24): 'a new desk, an auto radio' |  |  |
| A | docs/design/autoradio-in-exact-2026-11-18.md | governing | owner 2026-11-18, section 'Where data may go' (line 238); the rule is CLAUDE.md Product |  |  |
| A | docs/design/autoradio-learned-document-2026-11-17.md | governing | owner rulings 2026-11-17 (line 122) |  |  |
| A | docs/design/capsule-overlay-computer-state-2026-11-03.md | governing | owner 2026-11-03 (line 11): 'capsules should be an overlay over the exact computer state' |  |  |
| A | docs/design/design-session-2026-11-23.md | governing | the owner's words of 2026-11-23, quoted exactly in this file (line 7); a record, not a design |  |  |
| A | docs/design/desks-and-engine-2026-11-24.md | governing | owner 2026-11-24 (line 11): 'the new design superseded'; docs/system-map.md section 10 names it current |  |  |
| A | docs/design/first-design-2026-11-22.md | governing | owner 2026-11-22, 'this is the plan for the next throughline' (the first-design goal file, authority field) |  |  |
| A | docs/design/forced-modularity-build-2026-11-04.md | governing | owner rulings 2026-11-04, settled and quoted (line 5) |  |  |
| A | docs/design/jev-batched-questions-2026-11-10.md | governing | owner correction 2026-11-10, verbatim (line 12); also CLAUDE.md Delivery (owner 2026-11-15) |  |  |
| A | docs/design/library-app-store-security-intent-2026-11-04.md | governing | owner conversation 2026-11-04 recorded (line 5): not to build yet |  |  |
| A | docs/design/modular-architecture-plan-2026-11-04.md | governing | owner rulings R1 to R38 of 2026-11-04 (section 1, line 31); 'the one plan the owner reads' |  |  |
| A | docs/design/onboarding-voice-tour-2026-11-25.md | governing | owner direction 2026-11-25 (line 5) |  |  |
| A | docs/design/one-tape-durability-2026-10-31.md | governing | owner 2026-10-31 (line 8): 'one append-only tape' |  |  |
| A | docs/design/platform-operator-account-2026-11-06.md | governing | owner ruling 2026-11-06 (line 9) |  |  |
| A | docs/design/programmatic-model-switching-2026-11-09.md | governing | owner rulings 2026-11-09 (lines 13, 119) |  |  |
| A | docs/design/recoverable-changes-sr7-2026-10-24.md | governing | owner invariant 2026-10-24 (line 13): any release boots on any tape written by it or a later release |  |  |
| A | docs/design/resource-fabric-2026-10-31.md | governing | roadmap-2026-10-31.md line 25 names it the stage 2 end state (owner 2026-10-31: 'the canonical sequence') |  |  |
| A | docs/design/second-factor-for-sensitive-acts-2026-11-09.md | governing | owner direction 2026-11-09 (line 5) |  |  |
| A | docs/design/selfdev-apply-and-revert-2026-11-06.md | governing | owner rulings 2026-11-06 (section 0, line 12); docs/system-map.md says read first for self-development |  |  |
| A | docs/heresy-detectors.md | governing | executable: scripts/check-heresies.sh reads it in CI; named by docs/choir-doctrine.md line 39 |  |  |
| A | docs/inbox.md | governing | the owner's words, dated, as said (line 5); named an authority source in docs/definitions/choir-docs-transform-2026-11-27.md |  |  |
| A | docs/launch-checklist-2026-10-21.md | governing | owner direction 2026-10-21 (line 9); linked from docs/ACTIVE.md |  |  |
| A | docs/learning-engine.md | governing | written from the owner's statements of 2026-11-13 and 2026-11-14 (line 6); named in CLAUDE.md Desks |  |  |
| A | docs/memo-per-computer-frontend-2026-08-13.md | governing | listed as a source in docs/choir-doctrine.md lines 34 and 235; cited in docs/current-architecture.md line 190 |  |  |
| A | docs/memo-problem-documentation-first.md | governing | CLAUDE.md 'Problems come first' names it as the rule |  |  |
| A | docs/node-a-shared-host.md | governing | owner decision 2026-10-09 (line 5) |  |  |
| A | docs/operating-contract-rationale.md | governing | keeps the owner quotes, dates and reasons behind each CLAUDE.md rule (line 6) |  |  |
| A | docs/re-engineering-decisions-2026-10-12.md | governing | owner-agreed design direction (status line 5), agreed before any code change |  |  |
| A | docs/roadmap-2026-10-31.md | governing | owner 2026-10-31 (line 10): 'that's my mental roadmap'; named in CLAUDE.md Product |  |  |
| A | docs/security-density-architecture-checklist-2026-10-21.md | unverified |  |  | a living checklist linked from docs/ACTIVE.md; no owner ruling found in it |
| A | docs/standing-questions.md | governing | CLAUDE.md Missions: read before authoring or running a goal file |  |  |
| A | docs/system-map.md | governing | CLAUDE.md Product, owner 2026-11-22: 'each compaction you lose context on the complex system' |  |  |
| D | docs/NOW.md | superseded | this file's status (line 5): retired 2026-09-08; it points to docs/ACTIVE.md | docs/ACTIVE.md |  |
| D | docs/conjecture-assertion-ledger-2026-06.md | superseded | status (line 5): historical ancestor, no longer updated; CLAUDE.md Proof names the live ledger | docs/calibration/claude-ledger.md |  |
| D | docs/definitions/choir-autoradio-learning-engine-2026-11-14.md | unverified |  |  | no statement of supersession found; docs/ACTIVE.md lists it as a live station and choir-autoradio-learning-loop-2026-11-18.md does not say it replaces it (skim was wrong or unproved) |
| D | docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.md | superseded | docs/ACTIVE.md 'Superseded (2026-10-28): consolidate and scale', dropped by the owner; terminal receipt in the file | none |  |
| D | docs/definitions/choir-exact-search-entities-2026-11-20.md | superseded | the successor's supersedes field (its line 4) | docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md |  |
| D | docs/definitions/choir-guest-isolation-2026-11-01.md | superseded | this file's now.status (line 145); the successor names it as baseline source | docs/definitions/choir-store-header-body-split-2026-11-02.md |  |
| D | docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md | superseded | successor header line 6 'Supersedes'; docs/ACTIVE.md 'Superseded (2026-10-24)' | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md |  |
| D | docs/definitions/choir-memory-optimization-2026-11-01.md | superseded | owner 2026-11-01, this file's now.superseded_by (line 72); docs/ACTIVE.md says guest isolation supersedes it | docs/definitions/choir-guest-isolation-2026-11-01.md |  |
| D | docs/definitions/choir-night-exact-vectors-learning-2026-11-19.md | superseded | the successor's supersedes field (its line 4) and line 257 | docs/definitions/choir-exact-search-entities-2026-11-20.md |  |
| D | docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.md | superseded | owner 2026-11-04 ~20:55Z 'Let's supersede the current metamission' (line 173); successor line 12 | docs/definitions/choir-modular-architecture-2026-11-04.md |  |
| D | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md | unverified |  |  | skim was wrong: it is on hold (owner 2026-11-03) and choir-modular-architecture wave P8 resumes it; not superseded |
| D | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md | superseded | this file's readiness note (line 15) and now.status (line 155); successor line 7 | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.md |  |
| D | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.md | unverified |  |  | closed checkpoint_incomplete (docs/ACTIVE.md); choir-consolidate-and-scale supersedes only its ordering, and was itself dropped |
| D | docs/definitions/choir-t1a-app-self-development-2026-10-26.md | superseded | this file's readiness note (line 18): 'SUPERSEDED 2026-11-02 ... do not execute'; successor line 7 | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md |  |
| D | docs/design/autoradio-learning-engine-2026-11-14.md | superseded | this file's header (line 5) and the successor's line 6 | docs/design/autoradio-learned-document-2026-11-17.md |  |
| D | docs/design/autoradio-mission-handoff-2026-11-09.md | unverified |  |  | a handoff for the Swift era; no document says it is superseded (skim inferred it) |
| D | docs/design/autoradio-swift-test-map-2026-11-18.md | unverified |  |  | no document says it is superseded; it is a station record of the Exact cutover |
| D | docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md | unverified |  |  | skim was wrong: it is the document that supersedes fp10 lever L20, not superseded itself; standing not established |
| D | docs/design/entity-layer-2026-11-20.md | undecided | nothing; this file's status (line 5): a proposal, not ratified |  |  |
| D | docs/design/first-design-path-2026-11-22.md | superseded | this file's header (lines 5 to 7): 'the owner later ruled one kind, subject' | docs/design/first-design-2026-11-22.md |  |
| D | docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md | superseded | this file's line 5 and the successor's line 5 | docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md |  |
| D | docs/design/ideal-form-draft-2026-11-23.md | superseded | owner 2026-11-24 'the new design superseded' (header line 5; system-map section 10) | docs/design/desks-and-engine-2026-11-24.md |  |
| D | docs/design/library-package-propagation-2026-10-26.md | superseded | owner ruling 2026-11-03 'published releases mean published' (line 11) | docs/problems/published-release-visible-only-to-publisher-2026-11-03.md |  |
| D | docs/design/progressive-delivery-2026-10-26.md | superseded | owner 2026-11-03 'no switches at all' (line 9; rulings at line 785) | docs/definitions/choir-progressive-delivery-2026-11-03.md |  |
| D | docs/design/restore-bases-store-once-2026-11-15.md | superseded | header line 5: set aside 2026-11-15 after three review rounds (docs/problems/restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md) | none |  |
| D | docs/design/semantic-layer-2026-11-20.md | superseded | owner 2026-11-24 'the new design superceded' (header line 5; system-map section 10) | docs/design/desks-and-engine-2026-11-24.md |  |
| D | docs/design/snapshot-without-scratch-copy-2026-11-20.md | superseded | this file's header line 5; the replacing file's line 3 ('It supersedes') | docs/design/snapshot-one-pass-2026-11-22.md |  |
| D | docs/design/store-once-2026-11-14.md | unverified |  |  | its own header says design, nothing built; only the skim says it was set aside (restore-bases-store-once was, this one was not shown to be) |
| D | docs/platform-os-app-state.md | unverified |  |  | its status says canonical and stale as of 2026-09-22; stale is not superseded and nothing replaces it |
| D | docs/world-wire-mission-stack-2026-09-22.md | superseded | this file's status (line 6): SUPERSEDED roadmap, retained as a record (2026-10-14 prune) | docs/ACTIVE.md |  |
| C | docs/big-picture.md | undecided | nothing; this file's status (line 5): draft for the owner to cut down |  |  |
| C | docs/definitions/choir-autoradio-in-the-desktop-2026-11-23.md | governing | owner 2026-11-23, quoted in this file's authority field (line 4); deferred in docs/ACTIVE.md |  |  |
| C | docs/design/autopaper-learning-materiality-2026-11-04.md | undecided | nothing; this file's status (line 5): design note, not decided |  |  |
| C | docs/design/autoradio-on-device-voice-configurations-2026-11-11.md | undecided | nothing; this file (line 5): a research note with a proposal |  |  |
| C | docs/design/branching-tape-2026-10-31.md | undecided | nothing; this file's status (line 5): design only, section 10 doctrine delta needs owner ratification |  |  |
| C | docs/design/branching-tape-mission-proposal-2026-11-10.md | undecided | nothing; this file's status (line 11): proposal, needs owner ratification of the doctrine delta |  |  |
| C | docs/design/capsule-long-exec-2026-10-15.md | unverified |  |  | no status line saying proposal or ratified; may be built |
| C | docs/design/ci-runner-slots-2026-10-24.md | undecided | nothing; this file (line 8): proposal for FP0, not implemented |  |  |
| C | docs/design/coding-agent-desk-over-mcp-2026-11-09.md | undecided | nothing; this file (line 754): its shape ordering is a proposal, unratified |  |  |
| C | docs/design/computer-history-restore-audit-2026-10-30.md | undecided | nothing; this file's status (line 5): not ratified, nothing built |  |  |
| C | docs/design/corpus-first-retrieval-and-research-2026-11-18.md | undecided | nothing; this file's status (line 5): a proposal, not ratified |  |  |
| C | docs/design/delegate-first-for-source-changes-2026-11-02.md | unverified |  |  | carries an owner quote of 2026-11-02 but is an orange change record; no status line |
| C | docs/design/engineering-network-grants-2026-10-10.md | undecided | nothing; this file's status (line 6): design proposal, not settled |  |  |
| C | docs/design/leafrunner-seam-mission-proposal-2026-11-10.md | undecided | nothing; this file's status (line 5): proposal, readiness drafted |  |  |
| C | docs/design/library-app-store-2026-11-04.md | unverified |  |  | states rules after an owner review quote of 2026-11-04; reads as built, not a proposal |
| C | docs/design/mac-device-key-release-2026-11-01.md | undecided | nothing; accepted by a review seat (Devin round 3, this file line 5), no owner ruling found |  |  |
| C | docs/design/object-bodies-inline-on-the-tape-2026-11-15.md | unverified |  |  | design record; no status line on ratification |
| C | docs/design/outboard-capsules-arch-11-12-2026-10-22.md | unverified |  |  | design only, P2; its name is a proposal (OQ-1) but the design is not marked unratified |
| C | docs/design/paper-control-cli-2026-11-09.md | unverified |  |  | status is design, not built; no statement that it is unratified |
| C | docs/design/sdlc-audit-2026-11-10.md | undecided | nothing; this file's status (line 5): proposal for the owner, nothing adopted |  |  |
| C | docs/design/secrets-vault-design-2026-10-28.md | undecided | nothing; this file's status (line 5): draft for re-review, nothing authorized to execute |  |  |
| C | docs/design/texture-spec-follows-owner-2026-10-25.md | unverified |  |  | a change record (red, tier 1); no status line; probably built |
| C | docs/design/vsock-control-plane-and-nic-removal-2026-10-22.md | unverified |  |  | design, nothing built; no statement on ratification |
| C | docs/memo-diagonal-media-strategy-2026-09-17.md | unverified |  |  | skim was wrong: owner strategy direction recorded, not doctrine, predates Autopaper (header update 2026-10-14) |
| C | docs/operational-invariants-register-2026-10-08.md | undecided | nothing; this file's status (line 5): draft for owner review |  |  |
| C | docs/provider-vault-and-model-policy-design-2026-10-22.md | undecided | nothing; this file's status (line 5): proposal, not owner-ratified |  |  |
| C | docs/signal-is-sparse-not-the-learner-2026-08-01.md | unverified |  |  | skim was wrong: a foundational memo, not a proposal; its status says memo, not doctrine |
| B | docs/autopaper-sw-design-2026-10-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autopaper-improvement-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autopaper-overnight-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autopaper-overnight-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-full-experience-overnight-2026-11-15.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-learning-engine-2026-11-14.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-autoradio-overnight-2026-11-17.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-autoradio-pass1-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-architecture-selection-2026-10-28.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-system-build-2026-10-30.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-data-system-build-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-guest-isolation-2026-11-01.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-launch-fp0-density-bench-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp0-proof-scripts-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp0-vm-retention-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp1-a1-boot-alter-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-06-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-08-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-09-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-arch-11-12-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net-03-04-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net-05-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-net10-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-p2-small-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-08-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-10-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-13-22-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-14-2026-10-22.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res-20-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-res17-reaper-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp10-sec14-deploy-user-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp3-host-floor-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp6-network-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: sub-lane of the superseded launch first pass; executed |
| B | docs/definitions/choir-lean-system-2026-10-30.lanes.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-lean-system-2026-10-30.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-lean-system-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-mac-sync-stage2-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-memory-optimization-2026-11-01.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-modular-architecture-2026-11-04.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-modular-architecture-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-modular-architecture-2026-11-04.precommit.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-ordinary-prompt-path-2026-11-09.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-ordinary-prompt-path-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-overnight-pd-then-selfdev-metamission-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-progressive-delivery-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-progressive-delivery-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-selfdev-apps-feature-complete-2026-11-03.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-2-2026-11-12.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-2-2026-11-12.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-storage-solutions-3-store-once-2026-11-14.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-store-header-body-split-2026-11-02.ledger.md | not assessed |  |  | bucket B is not headed in this station; skim said: log/ledger of a finished mission (history, not design) |
| B | docs/definitions/choir-store-header-body-split-2026-11-02.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/choir-v-boot-vocab-pass-removal-2026-10-27.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/ci-test-cooptimization-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/definitions/corpus-first-search-one-verb-2026-11-19.md | not assessed |  |  | bucket B is not headed in this station; skim said: executed mission goal file |
| B | docs/design/agent-tree-budgets-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/app-design-bar-2026-10-26.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/app-motion-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: built (header); UI rule |
| B | docs/design/app-packages-replan-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/append-clustered-store-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/append-receipt-trim-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/apps-feature-complete-spec-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/autoradio-app-api-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-app-ui-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-desk-fast-turns-2026-11-11.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-l1-failure-list-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-lab-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-page-2026-11-17.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-queue-desk-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-system1-route-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-turns-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/autoradio-voice-modes-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/bounding-the-guest-store-2026-11-14.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/bundle-authenticity-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/choir-data-system-frame-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/ci-node-b-build-2026-10-24.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/coding-agent-desk-station-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/data-system-build-sheet-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/density-d6-page-cache-return-2026-10-26.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/derived-key-marks-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/desk-shots-and-review-2026-11-07.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/egress-grant-object-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/email-sign-in-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner 2014-10-14/10-31 ask; email sign-in code and deployed e2e exist (commit 0cff2f27a) |
| B | docs/design/enforce-flip-2026-11-08.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/engineer-reuse-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/intake-desk-pace-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/ma65-yaegi-go-modules-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/ma66-code-mode-import-sets-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/object-graph-redesign-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/object-storage-2026-10-30.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner Q1-Q4 accepted 2026-10-29; the data-system cutover landed 2026-10-30 |
| B | docs/design/object-verbs-quotas-ma38-ma39-2026-11-05.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/package-operation-decision-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: built per its header; apply-and-revert release 8a9f79cdf |
| B | docs/design/paper-compaction-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/podcast-clips-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: Autoradio 1.0 is built and in App Store review; queue desk first slice built; podcast clips landed |
| B | docs/design/research-more-goes-to-research-2026-10-21.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/ring-ordered-deploys-2026-11-03.md | not assessed |  |  | bucket B is not headed in this station; skim said: owner rulings 2026-11-03; internal/vmctl/ring_walk.go exists; CLAUDE.md Ring-Expedite rule is its outcome |
| B | docs/design/run-record-written-less-2026-11-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/s12-runtime-install-build-2026-11-04.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/sections-as-questions-2026-11-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/shot-verdict-reuse-2026-11-09.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/small-object-storage-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/design/store-header-body-split-2026-11-01.md | not assessed |  |  | bucket B is not headed in this station; skim said: landed (store split mission executed) |
| B | docs/design/tape-group-commit-2026-10-31.md | not assessed |  |  | bucket B is not headed in this station; skim said: tape group commit landed (abfab8754) |
| B | docs/design/tetramark-motion-2026-11-18.md | not assessed |  |  | bucket B is not headed in this station; skim said: built (header); UI rule |
| B | docs/design/visual-evidence-slice-2026-10-15.md | not assessed |  |  | bucket B is not headed in this station; skim said: header says built/landed |
| B | docs/model-pools-design-2026-10-12.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/precommitment-records-practice-2026-11-08.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/restart-subsystem-design-2026-10-13.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/semantic-registry.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| B | docs/vmctl-360-review-2026-10-10.md | not assessed |  |  | bucket B is not headed in this station; skim said: built / describes the live system |
| E | docs/definitions/choir-autoradio-learning-loop-2026-11-18.md | not assessed |  |  | bucket E is not headed in this station; skim said: readiness drafted; owner cleared the goal 2026-11-18 |
| E | docs/design/account-boundary-2026-11-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/account-switcher-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/app-desk-compute-capsule-2026-10-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/autopaper-next-mission-handoff-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/autopaper-step0-contracts-2026-11-10.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/autoradio-exact-integration-2026-11-19.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-experiment-e1-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-seed-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/calibration-wiring-permutations-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/candidate-on-de-before-main-2026-11-10.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/capsule-heavy-work-scheduling-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/capsule-overlay-s4-source-failure-modes-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/capture-static-copy-2026-11-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ci-prove-once-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/claude-delegate-slice1-2026-10-14.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/cli-agent-onboarding-2026-11-03.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/computer-update-architecture-rnd-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/delegate-one-round-build-2026-10-21.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/desk-install-review-2026-11-08.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/file-object-2026-10-24.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/first-design-offline-slice-plan-2026-11-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/gateway-handoff-without-a-gap-2026-11-20.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/gateway-streaming-for-every-model-2026-11-11.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/guest-tool-layers-2026-10-31.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/guest-tools-live-load-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/layered-guest-image-2026-10-31.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ma62-ma63-hardening-2026-11-05.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/ma64-gvisor-desk-workers-2026-11-06.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/model-pool-autoroute-2026-11-02.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/modularity-check-build-2026-11-04.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/replay-time-capture-with-unknown-cost-2026-11-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/sandbox-pause-resume-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/snapshot-one-pass-2026-11-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/snapshots-cost-the-change-2026-11-13.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/storage-pressure-frame-2026-11-12.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/texture-evidence-embeds-te-2026-10-25.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/texture-holds-the-trajectory-2026-10-25.md | not assessed |  |  | bucket E is not headed in this station; skim said: status unclear; owner request but no ratification line and no build evidence found |
| E | docs/design/trace-segment-log-2026-10-22.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/design/update-now-2026-11-09.md | not assessed |  |  | bucket E is not headed in this station; skim said: no clear status in header |
| E | docs/desk-system-harness-architecture-2026-10-01.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/mission-residues.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/runtime-invariants.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/source-external-data-publication.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/texture-agentic-invariants-2026-06-13.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/texture-live-supervision-architecture.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
| E | docs/why-texture-2026-06-15.md | not assessed |  |  | bucket E is not headed in this station; skim said: cannot tell |
