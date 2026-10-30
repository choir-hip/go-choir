# Live defects found by the dead-code census, 2026-10-12

The dead-code census ([docs/archive/reports/dead-code-census-2026-10-12.md](../archive/reports/dead-code-census-2026-10-12.md),
Part D) was looking for unused code. Along the way it found behavior that is
wrong today. This record names each defect and its evidence before any fix
(problem documentation first). The census agents read code, not production
traces, so any claim about what a model does is marked as a hypothesis.

## D1. Compaction points every desk at a tool no desk has

- **Evidence:** `internal/agentcore/run_memory.go:602, 652, 791`. The
  compaction prompt and the checkpoint text say "call get_run_memory_entry"
  and list "Compacted raw entry ids available via get_run_memory_entry".
- **Why it is wrong:** that tool is registered only on the typed-tool
  registries that are thrown away (census A5). The cell verb
  `choir.RunMemoryEntry` is exported to research only (`yaegikernel/choir.go`,
  the research module set). Texture, management and engineering have no
  way to retrieve compacted content.
- **Hypothesis, not confirmed by traces:** after a context rollover, a desk
  either tries a verb that does not exist or loses detail it was told it
  could recover. To confirm: read a compacted desk run's cells for
  attempted retrieval.
- **Owner:** Cut 1 (7), the prompt rewrite, or an earlier text fix.

## D2. The proxy delivers reader proposals to a deleted route

- **Evidence:** `deliverPublicationProposalToAuthor`
  (`internal/proxy/platform_public.go:327`) POSTs to the author's guest
  `/internal/texture/proposals`. `internal/apihandler/routes.go:49` records
  that the route was removed in R3d.
- **Effect:** every call fails, and every reader proposal is recorded as
  `recorded_for_author` through
  `/internal/platform/proposal-deliveries/state`. No proposal reaches the
  author.
- **Owner:** Cut 2 (publication), which should decide how reader proposals
  reach a story's Texture under the Cast and record model.

## D3. Research can Cast

- **Evidence:** the research module set exports `Cast`
  (`yaegikernel/choir.go:254`), and `castOpensEngineeringAssignment` does
  not check which desk is casting. Meanwhile `rlm_research_runtime.yaml:19`
  tells research it "does not cast work to other desks". Study G (E4, R2)
  reached the same reading.
- **Why it is wrong:** research holds read-only world authority (CLAUDE.md
  desk ontology). A research cell can still open engineering work.
- **Owner:** Cut 1 (authority, routing through management).

## D4. The gateway's ops routes do not check who is calling

- **Evidence:** `/provider/v1/search/health`, `/search/health/reset`,
  `/breakers` and `/breakers/reset` (`internal/gateway/handlers.go:1083-1097`)
  can be reached from any computer's tap network. Recorded first in
  [search-plane-cooldown-on-caller-cancel-2026-10-09.md](../archive/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md)
  (lines 84-98), which proposes `isAuthorizedCredentialCaller`.
- **Status: already repaired, not a live defect.** b445fd21 ("fix(gateway):
  search-health and breaker ops routes are loopback-operator only")
  landed before the census. All four routes call
  `isAuthorizedCredentialCaller`, and `internal/gateway/ops_routes_auth_test.go`
  covers the refusal. The census read the older problem doc rather than
  the code. Found by the R3 removal agent on 2026-10-12.

## D5. A run with an unknown profile gets full typed-tool authority

- **Evidence:** `InstallDefaultAgentTools`
  (`internal/agentcore/tool_profiles.go:447-545`) builds a typed-tool
  management registry and keeps it as `rt.toolRegistry`.
  `toolRegistryForRun` (`:547-558`) falls back to it for any profile that
  has no entry (`verifier`, `verifier_multimodal`, `processor`,
  `reconciler`).
- **Status:** latent. No run is created with those profiles today (no
  `AgentProfile:` writer).
- **Fix shape:** the fallback is nil, so an unknown profile fails closed.
  This lands with the removal of A5.

## D6. Part of the test suite never runs, and some of it does not compile

- **Evidence:** 14 `_test.go` files (6,014 lines) carry `//go:build
  comprehensive`, and no workflow or script sets the tag. `go test -c -tags
  comprehensive` fails for `internal/agentcore` (`content_prompt_test.go`:
  `handler.HandlePromptBar undefined`) and `internal/desktopstate`
  (`handler_test.go:250`: `h.CloneState undefined`).
- **The same problem in the frontend:** no workflow runs Playwright. About
  7,500 lines of specs in 17 files click UI removed on 2026-05-29
  (80823eba).
- **Effect:** the suite reports less than it appears to.
- **Owner:** the D0 removal step (census B11 and B12 rulings: delete what
  does not compile or targets removed UI; drop the tag from the rest so CI
  runs it).
