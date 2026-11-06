# MA6.2 and MA6.3: worker and broker hardening (2026-11-05)

Class red, tier 2 (MA6.2); MA6.3 touches the same trust boundary. Rollback: revert.
Plan row: docs/design/modular-architecture-plan-2026-11-04.md, MA6.2 and MA6.3.

## Audit of the MA6.2 row against desk_cell_confinement_linux.go

| Item | Before | After |
| --- | --- | --- |
| Explicit seccomp denials | The filter is deny by default (allow list), so io_uring, keyctl, bpf, perf, userfaultfd and ptrace were denied implicitly; nothing named them, and a later addition to the allow list would have opened one silently | `deskCellDeniedSyscalls` names them (EPERM); the profile test fails if a denied name is also allowed; `clone` is admitted only with CLONE_THREAD and no CLONE_NEW* bit |
| Mount namespace without /proc, /sys, /dev | Absent (Landlock hid them by path only) | Spawner: CLONE_NEWNS and CAP_SYS_ADMIN as an ambient capability. Worker: `isolateCellMounts` builds a tmpfs root holding the allowed root (read-only), its tmp dir, /dev/null, /dev/urandom, the zone files and a reduced /proc (hidepid=2, subset=pid); pivot_root; unmount the old root; seal; then `dropAllCapabilities` on every thread. No /sys, no /dev tree, no /etc. **/proc is reduced, not removed** (residual below) |
| fd closure across spawn | Go's exec leaves only close-on-exec descriptors out; a descriptor inherited without FD_CLOEXEC from the daemon's parent would survive | `CloseInheritedFDs` at the top of the worker closes every non-CLOEXEC descriptor above fd 3 |
| rlimits | RLIMIT_AS only (and cgroup pids.max and memory.max) | plus CORE 0, NOFILE 256, FSIZE 256 MiB, MEMLOCK 0, MSGQUEUE 0 |
| Unique uid per activation | One uid per desk (siblings shared credentials) | A uid per live worker from a pool (200000 to 259999); the manifest uid still gates the spawn |
| Restart intensity | None: a crashing cell could be respawned without bound | 5 self-ended workers of a role in 60 s refuse the next spawn, naming the cause |

## Ways this could fail (written first)

1. The new deny group shadows an allowed call and the Go runtime dies (assembly test, helper cells).
2. A denied name sits in the allow list again (the profile test enumerates both).
3. The mount namespace step runs without the capability, or leaves capabilities on one thread (capabilities are per thread; the drop goes through psx and is read back).
4. A path the worker is handed no longer resolves after the pivot (the allowed root, the tmp dir and the device files are bound at their own paths).
5. A descriptor leaks across exec (a test leaks one and reads /proc/self/fd).
6. The pool hands one uid to two live workers (race test).
7. The crash ledger counts a host kill, or never ages out (fake clock test).
8. The self-test judge passes an unconfined worker (the mutation test stays).

## MA6.3 failure modes (written first)

1. A decode panics or allocates without bound on hostile bytes (fuzz BrokerRequest, the frame reader, the worker mux reader loop).
2. A frame handler panics on a hostile payload (fuzz each verb's payload through the real handler).
3. A verb acts with the daemon's authority on a cell-supplied reference (audit of FrameAllowed and the verb handlers).

## Residual: /proc is reduced, not removed

libpsx (used by go-landlock and by the capability drop to act on every OS thread in a cgo build, which the nix build is by default) lists `/proc/<pid>/task` after each call and fails the call when it cannot (go-landlock bug 39; the library adds a Landlock rule for that one directory for the same reason). A root without /proc therefore fails `Restrict`. The worker's /proc is mounted `hidepid=2,subset=pid`: no `/proc/sys`, `meminfo` or any other process's directory, and the cell's Landlock domain grants only `/proc/<pid>/task`. Removing it outright needs either a worker built with CGO_ENABLED=0 (psx then uses `syscall.AllThreadsSyscall`, no /proc) or a two-stage start (stage 1 holds the capability on one thread, applies Landlock and execs stage 2). Either is a build or spawn-shape change; named for MA6.4, where gVisor's filesystem view replaces Landlock and this question goes away.

Not testable here: the production shape (a root spawner, `AmbientCaps` through `Credential`) runs only as root; the unit tests build the same root in a user, mount and pid namespace and the root-only tests (`TestDeskCellWorkerRunsAsTheDeskUser`, `TestDeskCellSelfTestPassesTheProductionShape`) were updated but could not run on the dev host. The proof is the boot self-test read through `POST /api/runtime/isolation-selftest` (judged by `JudgeDeskCellSelfTest` and `scripts/desk_cell_confinement_probe.py`).

## MA6.3: what was built and the audit result

Built: `internal/yaegikernel/broker_bounds.go` (`BrokerRequest.ValidateBounds`, `ValidatePayload`): the envelope fields have length and range limits, every payload is one valid JSON value of bounded depth (32), tokens (200,000), container size (4,096) and string length (3 MiB), and each action's top-level fields have their own limits (ids 256 bytes, a url 8,192, limits and counts ranged, research children at most 16, lease lists at most 256 entries). `admitFrame` calls it after the token, epoch and `FrameAllowed` checks, so a refusal is a trace event like any other, and the egress handler in `agentcore/tools_desk.go` calls `ValidatePayload` again (the second check on the handler itself). Fuzz targets in `broker_fuzz_test.go`: `FuzzBrokerRequestDecode`, `FuzzValidatePayload`, `FuzzReadFrame`, `FuzzWorkerMuxReadLoop`, `FuzzSearchFrame`, `FuzzSessionFrameDecode`, `FuzzAdmitFrame`. `scripts/fuzz-broker-protocol.sh` runs each for a fixed time, from the CI desk-protocol job (15 s each); the checked-in seeds under `testdata/fuzz` also run in every `go test`. `broker_bounds_test.go` has the per-action field-bound table test.

Fuzz findings (60 s a target, all seven; two crashers, both in the new validator, fixed and kept as seeds): a whitespace-only payload and a truncated `{` passed the token walk because the decoder reports EOF; `ValidatePayload` now requires one valid JSON value. The decode, the frame reader, the mux reader loop, the search store, the session frame decode and the admission check found nothing.

Audit of `FrameAllowed` and the verb handlers for confused-deputy use (a verb that acts with the daemon's authority on a cell-supplied reference):

- The role, epoch, run, owner, computer and agent come from the worker's spawn config and the host's execution context, never from a frame field. `admitFrame` takes the role from `cfg.Session.Role`; the handler re-checks `FrameAllowed(deskRole, action)`.
- `request_lease`, `system1`, `relate`, `relate_stories`, `stories`, `product_api`: holder desk, run, owner and computer are host values; payloads decode with `DisallowUnknownFields` where they carry structure; `product_api` serves with the run owner and refuses a write to a route without a kind.
- `get_run_memory_entry`: scoped to the calling run (X18, MA3.7b).
- Fixed: `research` had no cap on children (a cell could request an unbounded fan-out): 16. `list_evidence` took the cell's `limit` unclamped: now at most 200, also in the handler.
- Not changed, named: `read_evidence` reads any evidence record of the owner by id, and `list_evidence` lists the records of any agent id the cell names, within the owner. Neither crosses an owner, and a read taints the run with the record's labels, but evidence has no custodian desk, so a research cell can read what another desk saved. The fix is the evidence kind's custodian lease (MA3), not a bound here.
- Not changed, named: `exec`, `read_file`, `write_file`, `list_dir` are in the protocol's action list but no desk manifest row serves them on the host (they belong to the capsule broker); `FrameAllowed` refuses them for every desk, and the bounds table covers them for the day a row names one.

## MA6.3 second pass (2026-11-06): every field, every handler, every verb audited

Class orange (rollback: revert). What the first pass left: three wire actions (`texture_search`, `object_get`, `object_list`) were not in the fuzz list, and the "every action" test was a hand-written list that had drifted with them; the bounds tables did not cover every payload field, and nothing checked that a bounded field name is one the handler decodes; the fuzz targets reached the validators but not the host handlers themselves (failure mode 2 above); the audit was prose without line references.

Built:

- `TestEveryWireActionIsFuzzedAndValidated` reads every `BrokerAction` constant from `broker_protocol.go` with go/parser, so a new action fails until it is in the fuzz list and has a payload struct.
- `TestEveryPayloadFieldIsBounded`: every json field of every action's payload struct is in `payloadBounds`, `payloadListBounds`, or `boundElsewhere` (which names the handler validator that bounds it and why); a bound naming a field the struct lacks fails.
- Research children are bounded per child at admission (objective 32 KiB, model 256 bytes, role 32 bytes) and the handler runs `ResearchChildrenBounds` before any child starts; the cell SDK's whole-call check is the same function plus role, questions, p_success and weight (`ValidateResearchChildren`), so the two cannot drift.
- `FuzzHandlerPayloads` (yaegikernel): each handler's strict decode and validator (System1, Relate, RelateStories, Research), asserting what is accepted is inside the verb's bounds.
- `FuzzDeskEgressHandlers` (agentcore): eighteen verbs through the real host handler `rt.deskEgress`, on a real store seeded with another owner's evidence and another run's memory entry, from seven desks, with the run's role always management. On every input: research-only verbs serve only research; Stories and ProductAPI serve only a desk that holds the grant; Get and List read only a kind the session's desk writes; read_evidence never returns another owner's record; get_run_memory_entry never another run's entry; list_evidence never more than 200; and every path the product API handler is served is canonical, carries no escaped slash, and is not a blocked route. `choir.Research` is not driven (a valid call runs child model loops); its payload goes through `FuzzHandlerPayloads`.
- `scripts/fuzz-broker-protocol.sh` runs nine targets (eight yaegikernel, one agentcore) at the fixed time CI passes (15 s each).

Fixed (each a ledger line):

1. `choir.Stories` checked the `intake.read` grant against the run's role (`exec.Role`), not the session's desk: a desk whose cell ran in a context carrying a holder's role was served under the holder's grant. The check now takes the desk role the frame came from, as ProductAPI does.
2. ProductAPI checked the decoded, uncleaned path against its route lists and served it as given. `/api/autopaper/../leases` passed the allowlist (`/api/autopaper/` prefix) and reached the handler unchanged. The production router is the stdlib ServeMux, which answers an unclean path with a 307 that the in-process recorder does not follow, so no blocked route was served; but the check and the router read different paths. `normalizeProductAPIPath` now refuses a path that is not its own clean form, or that carries an escaped slash or a backslash.
3. `choir.System1` makes the judge read every capture a question names, with the owner's authority, and returned answers drawn from it without tainting the run. A capture ref now taints the run with the capture read floor (`owner_private:capture`, `world_untrusted`), as `read_capture` does; the cell's own passages add nothing.

### Confused-deputy audit, verb by verb (2026-11-06, at d930f16a8 plus this change)

Authority sources: the session's desk role (`deskRole`, from the worker's spawn config) and the run's execution context (`exec`, owner, computer, run, agent) are host values. Every frame passes `admitFrame` (token, epoch, `FrameAllowed(cfg.Session.Role, action)`, bounds; `yaegikernel/desk_session_worker.go`), and the session's Egress repeats `FrameAllowedAt(deskRole, deskEpoch, action)` and `ValidatePayload` (`agentcore/tools_desk.go:590-594`). The table is about what each handler does with the cell-supplied fields after that.

| Verb | Cell-supplied reference | Authority used | Verdict |
| --- | --- | --- | --- |
| system1 | question context_refs (capture ids), passages | owner from exec; research only (`system1_verb.go:112`); host: refs resolve against an empty map (`system1_verb.go:464`) | fixed: capture refs now taint the run (`run_labels.go:214`). Residual: the capture record's own labels are not read (floor only), see below |
| relate_stories | story ids, judgment_ref | research only (`story_relation_verb.go:72`); receipts must be this run's answered System1 events (`ev.RunID == runID`) | safe |
| relate | story, entities | research only (`story_entities_verb.go:119`); own story bound from exec.AgentID | safe |
| request_lease | kinds, capture ids, labels, purpose | holder is `deskRole`, run from exec (`leases.go:171`); the lease service decides | safe |
| object_get / object_list | kind, id | `resolveDeskObject(deskRole, kind)`; owner from exec (`desk_objects.go:265`, `:294`) | safe |
| product_api | method, path, body | `Holds(deskRole, "product.api")`; owner from exec (`tools_product_api.go:135`); route lists | fixed: non-canonical paths refused (`tools_product_api.go:190`, `canonicalProductAPIPath`) |
| stories | limit, before | grant check | fixed: grant read from deskRole, not exec.Role (`intake_view.go:74`) |
| texture_search | query, limit | owner from exec; served only under a texture.search lease in ctx (`research_children.go:552`, `store.TextureSearchUnderLease`) | safe |
| research | children (objective, role, model, questions) | parent run from exec; role per child (`research_children.go:51`); model from the catalog (`researchChildSelection`) | safe; bounds added (`research_children.go:187`, `ResearchChildrenBounds`) |
| save_evidence | kind, title, content, metadata | owner and agent from exec; labels in metadata discarded and restamped from the run (`tools_evidence.go:133`) | safe |
| read_evidence | evidence_id | owner from exec (`tools_evidence.go:169`) | residual (named in the first pass): any evidence of the owner, not only the desk's own; evidence's custodian lease is MA3's |
| list_evidence | agent_id, limit | owner from exec; limit clamped 200 (`tools_evidence.go:219`, `:222`) | residual (named in the first pass): any agent id within the owner |
| get_run_memory_entry | entry_id | owner and run from exec; another run's entry reads as absent (`tools_evidence.go:63`, X18) | safe |
| read | url | owner from exec; the address-guarded client (`agentcore/tool_profiles.go` binds `sourcefetch.Client` as deps.HTTP; `researchtools/read.go:97`: no loopback, private, link-local or metadata address, dials the checked address) | safe |
| poll_source | source_id, max_items | registry lookup; unknown ids refused | safe |
| source_api | provider, endpoint, params | the gateway holds the keys and its endpoint table (`researchtools/source_api.go:43`) | safe at this layer; params are structural only, the gateway's check is the bound |
| read_capture | capture_id, from_block, max_chars | the shared capture store, no owner (`researchtools/poll_source.go:309`); app desks gated by `gateCaptureRead` (`capture_read_gate.go:88`) | safe for app desks; system desks read any shared capture by design (captures are shared content, W1b2) |
| search_wire_corpus | query, limit | shared corpus | safe |
| search_start | query | disclosure gate (`tools_desk.go:603`); session-owned stream | safe |
| exec, read_file, write_file, list_dir | path, command | no desk row names them; `FrameAllowed` refuses every desk | safe (unreachable on the host) |

Residuals named, not fixed here: (a) System1's capture taint is the read floor; the record's own owner-private labels are not read, because the judge's capture read goes through `TextureCaptures.CaptureBlocks`, which does not return labels. A research desk that names an owner-private capture through System1 is tainted owner-private (`owner_private:capture`), so the disclosure gate still sees it, but not with the capture's specific label. (b) The read_evidence and list_evidence scope (first pass). (c) The production product API router redirects rather than serves unclean paths, so fix 2 is a second wall, not a live hole.
