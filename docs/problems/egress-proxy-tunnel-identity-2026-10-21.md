# The egress proxy binds a tunnel to the name it was asked for, not to what flows inside

Date: 2026-10-21. Status: open; the first fix direction is built on
`lane/b8-network` (see "Chosen fix"), not yet deployed. Found by: gpt-6.1-sol on the panel for
[delegate-one-round-build-2026-10-21.md](../design/delegate-one-round-build-2026-10-21.md).
Severity: P1 for `ecosystem_proxy` capsules. Mutation class of the fix: red.

## Problem

The capsule egress proxy (`internal/capsule/egress.go`) admits a tunnel
by the host name in the request and the address it resolves to, then
relays the bytes. Nothing ties what is spoken inside the tunnel to that
name. So the allowlist limits which front doors a capsule can knock on,
not which services it reaches through them. The allowlist's suffix match
also admits subdomains of every listed name.

The working detail is kept out of this public repository until the fix
lands (the repo rule: keep security detail out until remediated). It is
in the panel output outside the repo.

## Exposure

Every `ecosystem_proxy` (L2) capsule, which is the default for
implementation capsules. `claude`-mode capsules allow one host and are
exposed to the same class through it. Capsules with no egress are not
exposed.

## Fix directions (not chosen yet)

- Inspect the start of the tunneled session and refuse a name that is
  not the admitted one; cheap, closes the plain variant.
- A fetch service per ecosystem that terminates and constrains the
  request (GET/HEAD of package artifacts); the strong form.
- Exact host matching where a suffix is not needed.

## Evidence

Panel run 2026-10-21 (outside the repo); the reviewer's live probe was
against public endpoints, not a deployed capsule. A deployed probe from a
test computer's capsule is the admissible evidence for the fix.

## Chosen fix (2026-10-21, station FP6)

Of the three directions above, the first is clear, testable now, and cannot
narrow what legitimate toolchains reach, so it is built first:

1. **Bind the tunnel to the name (built).** After the proxy answers a CONNECT
   it reads the first TLS record the client sends and requires a single-record
   ClientHello whose only server_name is exactly the CONNECT host. It
   refuses, and closes the tunnel before a byte reaches the upstream: no TLS,
   no name, more than one name, a name in another form, a hidden name
   (Encrypted Client Hello), a hello split across records, a malformed hello,
   and a client that stays silent for ten seconds. It applies to every
   policy that uses the proxy, the Claude delegate's included. Code:
   `internal/capsule/egress_tls.go`; tests: `internal/capsule/egress_tls_test.go`
   (a table of malformed hellos, a real `crypto/tls` hello, and six tests
   through the proxy). With the check disabled, six of those tests fail.
   Fail-closed consequences, stated here so nobody is surprised: a client that
   sends a GREASE Encrypted Client Hello is refused with a real one (the proxy
   cannot tell them apart), and a client that sends no SNI is refused. The go
   command, pip, npm, git, curl and huggingface_hub send neither by default.
2. **Exact host matching (not built, named).** The allowlist still matches
   suffixes, so a subdomain of a listed name is admitted. Replacing the
   suffixes with an enumerated host list needs the exact set of names the
   toolchains use (for example the package-file hosts behind `pythonhosted.org`,
   `hf.co` and `githubusercontent.com`), and a wrong list breaks builds. It
   waits for a measured list: the egress ledger already records every host
   (`internal/capsule/executor.go:592-605`), so a week of the test computer's
   ledger names them. Residual until then: a listed name's own subdomains.
3. **A fetch service per ecosystem (NET-02).** Still the strong form and the
   durable fix; it makes direction 2 unnecessary.

Deployed proof (written for the orchestrator, no secrets): on a test
computer, from an implementation capsule, `pip download` of a small package,
`npm view` of a package, `git ls-remote https://github.com/...` and one model
download through `huggingface_hub` each succeed with the egress ledger
showing outcome ok; a TLS client in the capsule that CONNECTs to an
allowlisted host and sends a different server name is refused (ledger
outcome refused, reason naming the SNI) and receives no bytes from the
upstream.
