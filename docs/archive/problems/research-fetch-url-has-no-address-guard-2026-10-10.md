# Research's fetch_url reaches any address — no private-address guard (2026-10-10)

Found while designing engineering network grants
(docs/design/engineering-network-grants-2026-10-10.md §2.6, D1); confirmed in
source, not exercised on staging.

- `InstallDefaultAgentTools` builds research's fetch client as a plain
  `&http.Client{Timeout: 30 * time.Second}` (internal/agentcore/tool_profiles.go).
- `newFetchURLTool` (internal/researchtools/researchtools.go) trims the URL,
  charges the egress budget, and GETs it. No scheme, host or resolved-address
  check.
- A guarded policy already exists: internal/sourcefetch/policy.go refuses
  loopback, private, link-local, multicast and unspecified addresses. It is not
  on this path.

Consequence: a research cell, or content that prompt-injects one, can GET guest
loopback routes (the runtime listens on :8085) and any host or private address
the guest VM can route to. The response, up to 256 KiB, comes back into model
context, where it can be relayed or written into a document.

D2 (same doc): sourcefetch checks the resolved address and then dials by
hostname, so DNS rebinding can pass its check. The fix must dial the address
it checked.

Fix order: wire the sourcefetch guard into fetch_url with dial-the-checked-IP.
That fix is red (egress) and comes before Phase 1 of the grants design.

## Adjacent finding while fixing (2026-10-10, not fixed)

`Dependencies.egressToolTable` (internal/researchtools/host_egress.go) caches
the per-deps tool table under `key := &d`, but `d` is a value receiver, so
the key is a fresh pointer on every call. The cache never hits and
`egressTables` gains one entry of closures per research egress call for the
life of the guest process. Effect: unbounded memory growth proportional to
research egress calls, and every call rebuilds its tool table. Fix shape:
key on the `*Dependencies` the runtime holds (`rt.researchDeps`), or drop
the cache. Mutation class `orange`.
