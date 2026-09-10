# Mission-2 Frozen V1→V2 Mapping Tables — 2026-09-10

Charter acceptance item 4. Code-free Define freeze. Source ref `main@121d03b8`.
Inventory artifact `docs/evidence/choir-rlm-v1-inventory-2026-09-10.json` (308 rows).
No mapping is implied: every equivalence below is written down; the writer
cutover (later item) verifies each row mechanically. Until owner ratification
of §7, the three owner rows stay `unknown` and `check-v1-inventory.sh --freeze`
stays red by design.

Conventions: `MAP` = forward under the frozen map with proven inverse;
`INV-CANON` = deterministic rollback to the canonical V1 representative
(`super`, `co-super`, `researcher`), proven semantically compatible with old
source; `INV-PROV` = exact restoration from retained per-row source-token
provenance (required where many-to-one collapse would lose the token, e.g.
`coagent`→engineering vs `cosuper`→engineering). `GEN` = generic carrier
transports versioned values, no map of its own.

## 1. Per-function V1 acceptor tables (frozen, never unioned)

### 1a. agentprofile.Canonical V1 — exactly these branches, zero more
| V1 input set | Returns | V2 live name |
|---|---|---|
| `super` | `Super` | `management` |
| `cosuper`, `co-super`, `coagent`, `co-agent` | `CoSuper` | `engineering` |
| `researcher`, `researchers`, `research`, `research-agent`, `web-research`, `web-researcher` | `Researcher` | `research` |
| `texture`, `texture-agent`, `document-agent` | `Texture` | `texture` (stays live) |
| `processor`, `news-processor`, `source-processor`, `universal-wire-processor` | `Processor` | `processor` (stays live) |
| `reconciler`, `news-reconciler`, `story-reconciler`, `corpus-reconciler`, `universal-wire-reconciler` | `Reconciler` | `reconciler` (stays live) |
| `email`, `email-agent`, `email-appagent`, `mail`, `mail-agent` | `Email` | `email` (stays live) |
| `conductor` (exact) | `Conductor` | `conductor` (stays live) |
| default passthrough | normalized input (LIVE LEAK, closed by fail-closed Canonical at activation item) | — |

Underscore inputs normalize `_`→`-` before matching, so `co_super` also
resolves to `CoSuper`; `cosuper_coding` / `co-super-coding` do NOT match
Canonical (they are NormalizeRole-only). `engineering` passthroughs today.

### 1b. modelpolicy.NormalizeRole V1 — exactly these branches, zero more
| V1 input set | Returns | V2 live name |
|---|---|---|
| `cosuper`, `co_super`, `co-super`, `cosuper_coding`, `co-super-coding` | `agentprofile.CoSuper` | `engineering` |
| `texture`, `texture-agent` | `agentprofile.Texture` | `texture` (stays live) |
| `verifier`, `verifier-text`, `verifier_text` | `VerifierRole` | `verifier` (stays live) |
| `verifier-multimodal`, `verifier_multimodal` | `MultimodalVerifierRole` | `verifier-multimodal` (stays live) |
| default passthrough (PINNED ABSENCE: no research branch; `researcher` and the other five research aliases pass through unknown) | input (LIVE LEAK, closed at activation item) | — |

### 1c. spawnRoleAllowed V1 (own table in rlm_reduce.go:76-89)
| Spawner | Children allowed |
|---|---|
| `super` | any |
| `co-super`, `cosuper`, `engineering` | `researcher`, `co-super`, `cosuper`, `engineering` |
| `researcher`, `research` | `researcher`, `research` |
| default | false |

### 1d. promptstore.normalizePromptRole — strict exact-canonical only, zero
aliases. promptRoles set: conductor, texture, researcher, processor,
reconciler, super, co-super (+`core` file, not a profile).

## 2. V2 live tables (frozen identity maps, fail-closed defaults)

V2 live set: `management`, `engineering`, `research` (lowercase wire tokens)
plus stays-live `texture`, `conductor`, `processor`, `reconciler`, `email`,
`verifier`, `verifier-multimodal`.

- **V2 Canonical**: identity over the live set, zero alias branches,
  fail-closed default (empty string + typed unknown-profile error).
- **V2 NormalizeRole**: identity over the live set plus `VerifierRole` and
  `MultimodalVerifierRole`, fail-closed default.
- **V2 spawnRoleAllowed**: `management`→any child; `engineering`→children
  `engineering`, `research`; `research`→child `research`; default false.
  No V1 token accepted in any position.

## 3. Three-desk forward map (one frozen map, never engineering alone)

| V1 token(s) | V2 token | Inverse |
|---|---|---|
| `super`, every Canonical branch returning `Super` | `management` | INV-CANON `super` |
| `co-super`, `cosuper`, `coagent`, `co-agent`, `co_super`, `cosuper_coding`, `co-super-coding`, V1 spawn synonym `engineering` | `engineering` | INV-PROV where collapsed (`coagent` vs `cosuper` vs `co_super` vs codings), else INV-CANON `co-super` |
| `researcher`, `researchers`, `research`, `research-agent`, `web-research`, `web-researcher` | `research` | INV-PROV where collapsed, else INV-CANON `researcher` |

Per-carrier application (carrier column of every inventory row): tape event
field, mailbox/channel rows, run rows, lifecycle rows, assignment rows, grant
rows, agent rows, object-graph objects/edges, prompt/profile registry tags,
payload envelope markers (where role-bearing), computer-owned model-policy
TOML overlays. Classes with no carrier use unmarked-defaults-V1 only by
charter-relative exception with negative-sweep evidence (recorded in the
artifact's negative_sweeps).

## 4. research/research version collision (explicit)

`research` is a V1 alias (resolves to `Researcher`) AND the V2 canonical.
Live-authority eligibility is the row's version stamp after migration, never
token shape: a V1-stamped `research` row is unmigrated until stamped `v2`,
and the identity map `research`→`research` still requires the `v2` stamp.
Same rule covers `engineering` (V1 spawn synonym, V2 canonical).

## 5. Prompt path overlay map (owner-override migration, frozen at charter)

| V1 path | V2 path |
|---|---|
| `super_runtime.yaml` | `management_runtime.yaml` |
| `co_super_runtime.yaml` | `engineering_runtime.yaml` |
| `rlm_co_super_runtime.yaml` | `rlm_engineering_runtime.yaml` |
| `researcher_runtime.yaml` | `research_runtime.yaml` |

Overlay `role:` ids: `super_runtime_overlay`→`management_runtime_overlay`,
`co_super_runtime_overlay`→`engineering_runtime_overlay`,
`rlm_co_super_runtime_overlay`→`rlm_engineering_runtime_overlay`,
`researcher_runtime_overlay`→`research_runtime_overlay`. YAML role ids follow
the §3 lexeme map. Desk-name prose inside overlays becomes Management,
Engineering, Research. Tool identifiers stay frozen until the R7 successor.
Code filenames and package paths stay frozen; prompt file *paths* rename
(never bare filenames). `SOURCE_PANEL_MODEL_ROLES`: `researcher`→`research`,
`super`→`management`; `engineering` joins only if the census showed a live
selector (it did not — spawn synonym only — so it does not join).

## 6. Successor v2 identity/mapping receipt (fields the v2 receipt enumerates)

The mission-1 v1 classification receipt stays immutable. The successor v2
receipt records token, prefix, and digest-input equivalence for every
digest-bearing m1 predecessor field carrying desk vocabulary: the terminal
proposition V1 domain (`choir:terminal-proposition:v1`, frozen, never
recomputed modernized); co-super command request JSON (`co-super-open`,
`-bind`, `-report`, `-cancel*`, `-capsule`, `-restart-*`, `-system-cancel`,
all frozen byte-stable); `AgentRecord` Profile/Role; `WorkItemRecord`
AuthorityProfile; grant `Role`; verb-set digest inputs
(`choir.co_super_verb_set/v1`) and policy digest inputs
(`choir.co_super_grant_policy/v1`); command IDs and command digests;
attestation references (`co-super-grant:sha256:`, `co-super-execution:sha256:`,
`co-super-fate:sha256:`); schema and version strings
(`choir.co_super_assignment/v1` and attestation/fate siblings, frozen); and
all role-bearing identity prefixes (`super:`, `co-super:`, `researcher:`).
Historic V1 records verify with original bytes, tokens, and V1 digest rules;
post-cutover verification of historic records must equal the pinned bytes.
Where migration changes a digest-covered field, the live V2 representation
receives a V2-domain successor digest with the mapping receipt linking the
immutable V1 predecessor; a V1 digest remains only as named provenance and
never authenticates renamed V2 content. A row retains its digest only when the
inventory proves the renamed field stood outside that digest's canonical input.

## 7. Owner-token classification proposal (requires owner ratification)

Role-shaped `owner` tokens (inventory rows `restoreIntentOwner`,
`revisionRoleOwner`, `revisionFromOwner`):
**proposed: frozen non-desk protocol value**, same standing as `trusted-core`
and `PrivacyClass owner` (different namespace, stays frozen).

Rationale: `owner` denotes the human-owner authority origin
(`platform-control:restore`, revision authorship), never a desk. No desk
spawn/admission/policy semantics apply to it; the V2 desk set has no owner
desk; mapping it to management would grant live management authority to a
protocol marker. Frozen V1 decode cover: the frozen V1 decoder accepts `owner`
in exactly the three inventoried positions (history-only). Live writers may
continue emitting `owner` as a protocol value (it is not desk vocabulary, so
writer-purity is unaffected). Activation must not refuse `owner`, exactly as
it must not refuse `trusted-core`. Mailbox `owner` values flow through generic
carriers (`ChannelMessage.Role`, `ToDesk`/`FromDesk`); classifying the token
as frozen protocol covers those paths with no carrier version tag required.
If the owner instead ratifies `owner` as a named V2 profile, the three rows
take V1 decode cover under §3 with per-row carriers, and the writer cutover
waits for that cover.

Until ratification: the three rows stay `unknown` allowlisted, and
`check-v1-inventory.sh --freeze` stays red. The writer cutover never lands
while `owner` is unclassified.

## 8. Verifier rules per row (template)

- V1 verifier: value ∈ the §1 per-function acceptor set for its function;
  unknown values already fail closed at parse/normalize boundary or are
  recorded as the named passthrough leak closed at the activation item.
- V2 verifier: value ∈ the §2 identity map for its function; row carries a
  `v2` stamp where the carrier is versioned; unmarked rows never serve live
  authority after cutover (serving fence, decode item).
- Copy-forward of a V1 token into a V2-stamped row is a writer violation.
  V1→V2 migration occurs only through §3 with proven inverse, never by
  implicit canonicalizer pass-through.
