# Decision: How Engineering Capsules Get Go Modules

Date: 2026-10-14. For the owner. Found by the C1e B1 engineering drill
(docs/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md).

## The problem

An engineering capsule cannot build Go, and this predates C1e. The capsule's
L2 egress allowlist (`internal/capsule/egress.go`, `ecosystemHosts`) has
`proxy.golang.org` and `sum.golang.org`, but the Go module proxy answers
module zip requests with a redirect to `storage.googleapis.com`, which is
not allowed. So `go build ./cmd/autoputer` fails in every capsule. A
self-development change to Choir itself needs `bin/autoputer` in the
release tree before Freeze, so no Choir change can be frozen today. That
includes frontend-only changes, because the release moves the binary and
the frontend together. npm builds work (registry.npmjs.org is allowed).

Earlier freezes (M11, 2026-10-10) got past this only because the desk
found another way to stage a binary. Tonight's prompt fix (9874ebc5) tells
engineering to build the real binary, so the drill now ends `blocked` at
the build step.

## Options

**A. Allow `storage.googleapis.com` in L2 egress.** One line, unblocks today.
- Cost: a capsule could then reach any public Google Cloud Storage bucket.
  Uploads stay capped at 16 MiB and downloads at 8 GiB per capsule. The
  allowlist already has hosts that are as broad (`githubusercontent.com`,
  `huggingface.co`), so the marginal exposure is small but real.
- Class: red (egress policy). Rollback: delete the line.

**B. A host-side module cache, no egress for Go.** The Nix build already
fetches every module in `go.sum` (the `goModules` derivation behind
`vendorHash`). Publish that set as a read-only GOPROXY tree, mount it into
engineering capsules, and set `GOPROXY=file:///deps/goproxy,off` and
`GOFLAGS=-mod=mod`.
- Gains: builds are offline, deterministic and faster; no new egress. It
  matches the existing design note's `GOPROXY=file:///deps/goproxy`.
- Cost: a Nix output and a capsule mount (a day, not an hour). A change
  that adds a new module still needs the network, so it would need A or a
  per-request fetch.

**C. `GOPROXY=direct`** (fetch modules from their source repositories).
github.com is allowed, but golang.org/x and google.golang.org modules come
from other hosts (go.googlesource.com and others), so it needs more hosts
than A and is slower. Not recommended.

## Recommendation

B as the real fix, because the platform's own builds should not depend on
the network. A only if you want self-development drills to reach Freeze
before B lands. Nothing has been changed; this needs your call because it
is egress policy.
