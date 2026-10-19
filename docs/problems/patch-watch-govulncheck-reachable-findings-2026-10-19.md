# Patch Watch: govulncheck reachable findings, 2026-10-19

Mutation class: green for this document; the fix that follows is orange
(dependency versions change runtime behavior of HTTP/2, gRPC, and OTel code).

## Evidence

- Scheduled workflow "Patch Watch" (`.github/workflows/patch-watch.yml`) has
  failed daily since 2026-10-15. Latest failing run: 39258251548
  (2026-10-19). The failing step is "govulncheck (reachable findings,
  production Go)": `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`
  exits non-zero (exit 3 locally) when findings are reachable.
- Reproduced locally at the same invocation and package scope (`./...`),
  Go 1.26.9 (go.mod `go 1.26.2`, `toolchain go1.26.9`, the declared
  production Go). Result: 9 reachable vulnerabilities, all in third-party
  modules. None are in the Go standard library, so no toolchain bump is
  needed.

## Findings

| # | ID | Module | Found in | Fixed in | Representative call path |
|---|----|--------|----------|----------|--------------------------|
| 1 | GO-2026-6617 (HTTP/2 server crash, HPACK encoder race) | golang.org/x/net | v0.55.0 | v0.60.0 | `proxy.Handler.HandleProtectedAPI` -> `httputil.ReverseProxy.ServeHTTP` -> `http2.Transport.RoundTrip` |
| 2 | GO-2026-6612 (double flow-control refund, HTTP/2 server streams) | golang.org/x/net | v0.55.0 | v0.60.0 | `agentcore.APIHandler.HandleModelPolicyVerify` -> `io.WriteString` -> `http2.responseWriter.WriteString` |
| 3 | GO-2026-6611 (CPU from repeated initial window changes) | golang.org/x/net | v0.55.0 | v0.60.0 | `actorruntime.Adapter.Start` -> `sync.Once.Do` -> `http2.Framer.ReadFrame` |
| 4 | GO-2026-6610 (malformed framing-related headers accepted) | golang.org/x/net | v0.55.0 | v0.60.0 | `store.init` -> `driver.init` -> `http2.ConfigureTransports` |
| 5 | GO-2026-6603 (HTTP/2 server memory exhaustion via Trailer headers) | golang.org/x/net | v0.55.0 | v0.60.0 | `textureowner.Handler.HandleTextureDocumentStream` -> `http2.responseWriter.Flush` |
| 6 | GO-2026-6505 (OTel exporter config may log endpoint URLs) | go.opentelemetry.io/otel/sdk | v1.43.0 | v1.45.0 | `store.init` -> `driver.init` -> `resource.init` |
| 7 | GO-2026-6348 (heap exhaustion via HTTP/2 DATA frame fragmentation) | google.golang.org/grpc | v1.79.3 | v1.83.1 | `store.init` -> `driver.init` -> `mem.NewBuffer` |
| 8 | GO-2026-6061 (xDS RBAC and HTTP/2 transport server flaws) | google.golang.org/grpc | v1.79.3 | v1.82.1 | `store.init` -> `driver.init` -> `transport.ClientStream.Close` |
| 9 | GO-2026-5970 (infinite loop on invalid input) | golang.org/x/text | v0.37.0 | v0.39.0 | `proxy.Handler.HandleProtectedAPI` -> `httputil.ReverseProxy.ServeHTTP` -> `norm.Form.String` |

The gRPC and OTel traces enter through the Dolt driver's `init` (Dolt pulls
them in transitively); the x/net and x/text traces are reached from our own
HTTP handlers and the reverse proxy.

govulncheck also reports 6 vulnerabilities in imported packages and 4 in
required modules that our code does not appear to call. Those do not fail the
step and are out of scope here.

## Why the workflow keeps failing

The step runs under GitHub's `bash -e`, so a non-zero govulncheck exit ends
the step before the "reachable findings" count is written to `GITHUB_OUTPUT`.
That is why the issue-upsert step sees an empty `GO_COUNT` and never opens a
"govulncheck reachable findings" issue. This is observed behavior, not part
of this fix: the workflow and the govulncheck invocation are deliberately left
unchanged. The cure for the red step is zero reachable findings.

## Planned fix

1. `go get` golang.org/x/net@v0.60.0, go.opentelemetry.io/otel/sdk@v1.45.0,
   google.golang.org/grpc@v1.83.1 (covers v1.82.1), golang.org/x/text@v0.39.0;
   then `go mod tidy`. Keep the diff to `go.mod`/`go.sum` plus whatever
   `go mod tidy` requires.
2. Re-run govulncheck with the workflow's exact invocation until reachable
   findings are 0.
3. Run focused tests for packages affected by the bumps.
4. If a bump forces a newer Go than go.mod/toolchain/nix provide, document that
   here instead of forcing it.

Rollback: revert the fix commit (dependency bump only).

## Not part of this fix

- Issue #72, "Patch SLA: Firecracker behind upstream (1.15.1->1.17.0)", comes
  from the same workflow run but is a separate decision (a VM-lifecycle
  change). It is untouched here.

## Heresy delta

- discovered: the `bash -e` interaction above means govulncheck findings
  never produce the intended patch-sla issue.
- introduced: none.
- repaired: none yet.

## Resolution (fix commit)

- `go get` of the four modules. The module graph forced golang.org/x/text to
  v0.42.0 (golang.org/x/net v0.60.0 requires it), not the minimum v0.39.0;
  gRPC v1.83.1, OTel sdk v1.45.0, and x/net v0.60.0 as planned. Their
  dependencies moved by MVS (x/crypto, x/sys, x/sync, x/mod, x/oauth2, x/term,
  x/time, x/tools, google.golang.org/api, genproto, OTel contrib, and others).
  `go` and `toolchain` lines are unchanged.
- `flake.nix`: `commonGoArgs.vendorHash` updated, because the Nix build vendors
  from `go.mod`/`go.sum`. The new hash was computed with buildGoModule's own
  `goModules` fixed-output derivation over the same source filter; the same
  procedure reproduced the old hash exactly on the pre-bump `go.mod`/`go.sum`.
  The CI Nix build is the authoritative check.
- Result: `govulncheck@v1.9.0 ./...` reports "Your code is affected by 0
  vulnerabilities" (1 imported-package and 2 required-module findings remain
  that the code does not call).
- Tests: `go vet ./...` clean; `go test` passed for internal/store, proxy,
  auth, gateway/..., vmctl, toolregistry, modelpolicy, actorruntime, selfdev,
  and focused subsets of internal/agentcore and internal/textureowner.

Heresy delta: repaired the nine reachable findings above.

## Follow-up: platform-specific findings (same PR)

- After the first fix commit, the local darwin govulncheck reported 0, but the
  PR's "govulncheck (report only)" job (linux/amd64, same invocation) still
  failed (job 118266964676; the output is not logged because the step runs under
  `bash -e`). Hypothesis, not yet trace-confirmed: govulncheck only analyzes the
  host GOOS, and the linux build reaches code the darwin scan does not. The
  darwin scan's non-called list pointed at google.golang.org/grpc v1.83.1
  (GO-2026-6443, fixed in v1.83.2) and github.com/klauspost/compress v1.18.0
  (GO-2026-5841, fixed in v1.18.7).
- Second bump: grpc v1.83.1 -> v1.83.2, klauspost/compress v1.18.0 -> v1.18.7;
  `vendorHash` recomputed again. golang.org/x/crypto v0.57.0 has one
  GO-2026-5932 advisory with no fixed version (N/A); it is not reachable on
  darwin and is recorded as a named residual.
- A local linux scan was not possible (cgo cross-compile for Dolt/ICU fails on
  macOS; the Docker VM OOM-kills govulncheck). The PR's linux CI job is the
  confirming check.

## Follow-up 2: linux scan reproduced and confirmed

- A linux scan was possible after all: `golang:1.26.9` in Docker with
  `libicu-dev`, `GOFLAGS=-p=1 GOMEMLIMIT=4GiB GOGC=40` (the earlier OOM was
  memory pressure). On the second-bump tree it reported one reachable finding
  that darwin cannot see: GO-2026-6238, github.com/cilium/ebpf v0.16.0, fixed
  in v0.22.0, reached via `capsule.CreateCgroup` -> `cgroup2.NewManager` ->
  `btf.LoadKernelSpec` (linux-only code, so the hypothesis above is confirmed:
  govulncheck scans the host GOOS only, and the darwin scan under-reports for
  this repo. The linux scan is the one that matches the workflow's runner).
- Third bump: github.com/cilium/ebpf v0.16.0 -> v0.22.0 (logrus v1.9.3 ->
  v1.9.4 follows by MVS); `vendorHash` recomputed again.
- Linux scan after the third bump: "Your code is affected by 0
  vulnerabilities", and `go build ./...` succeeds on linux.

## Follow-up 3: the ci.yml "govulncheck (report only)" job stays red at 0 findings

- With linux/arm64 and linux/amd64 scans both at "Your code is affected by 0
  vulnerabilities" (Docker, `golang:1.26.9`), the PR's ci.yml job
  "govulncheck (report only)" still fails (runs 39312914634, 39315269948, and
  the next one). Hypothesis from reading `.github/workflows/ci.yml` (lines
  ~306-326), not trace-confirmed because the step output is never logged: the
  step runs under `bash -e -o pipefail`; with zero findings the summary
  pipeline `grep -E '^(Vulnerability #|    Found in:|...)' ... | head -200`
  matches nothing, `grep` exits 1, `pipefail` makes the pipeline fail, and `-e`
  ends the step with exit 1 before `cat` and `exit 0`. With findings, the
  `go run ... govulncheck` line itself exits 1 under `-e`. Either way the job
  cannot pass, and it is not a required gate ("Go Vet + Test + Build" is green).
- This is separate from Patch Watch: `patch-watch.yml` builds its summary with
  `echo` only and counts with `grep -c ... || true`, so with 0 findings its
  govulncheck step passes (status 0, count 0). The ci.yml job is left unchanged
  here (instruction: do not weaken workflows); it is recorded as a residual
  heresy: discovered, not repaired.
