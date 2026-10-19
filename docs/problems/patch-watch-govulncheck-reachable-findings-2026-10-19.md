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
