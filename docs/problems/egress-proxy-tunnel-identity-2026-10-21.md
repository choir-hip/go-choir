# The egress proxy binds a tunnel to the name it was asked for, not to what flows inside

Date: 2026-10-21. Status: open; the first fix direction is replayed onto
main on `lane/fp6-sec06-on-main` in observe mode (see "Observe first",
2026-10-22), not yet deployed. Found by: gpt-6.1-sol on the panel for
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

## Observe first (2026-10-22, station FP6)

**Replayed onto main.** The first fix, built 2026-10-21 as `a38eb7f67` on a
local branch and never landed, is replayed onto `origin/main` `966d3f519`
with `git am -3` (it applied cleanly over FP2's second batch) as `e20c92b6c`,
commit message kept.

**Observe first.** As built, the check refuses traffic, and a wrong refusal
breaks builds: a GREASE Encrypted Client Hello cannot be told from a real one,
and a client may send no SNI. The metamission rule is that new red behaviour
ships off or in observe, so the check now has a mode
(`internal/capsule/egress_tls.go`):

- `observe` (the default): the proxy reads the tunnel's first record and
  judges it exactly as enforce does, writes the verdict on the capsule's
  egress ledger entry, logs a would-refuse once per capsule and host, and
  relays the record and the tunnel unchanged. The entry's `outcome` stays
  `ok`; `tunnel_identity` is `pass` or `would_refuse`, and
  `tunnel_identity_reason` names why (for a mismatch: `ClientHello SNI
  "pypi.org" is not the CONNECT host "github.com"`).
- `enforce`: refuses as `a38eb7f67` did: `outcome` `refused`,
  `tunnel_identity` `refused`, and the reason in both reason fields.
- `off`: no read; the two fields are absent.

Where it is set: the guest daemon's environment,
`CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY`, set to `observe` in
`nix/autoputer-vm.nix` beside `CHOIR_CAPSULE_MEMORY_POLICY`. The daemon reads
it at start (`internal/autoputer/capsule_executor_linux.go`) and the executor
writes it on every egress policy it builds, the Claude delegate's included
(`egressPolicyLocked`). Unset and unknown values are observe. A capsule never
supplies a policy, so the mode cannot be set from inside one. Known cost of
observe: a client that stays silent (a server-speaks-first protocol) is
relayed after the 10 s hello timeout instead of at once; TLS clients speak
first.

Commits on `lane/fp6-sec06-on-main`: `e20c92b6c` (the replay, red),
`d8d88ae5f` (tests first, yellow), `e0efea302` (the mode, red, full
ceremony in its message), and the docs commit carrying this section and
`scripts/capsule-tunnel-identity-probe.sh`.

**FP0's deployed proof (observe).** On a test computer, from an
implementation capsule (a capsule job):

1. Identity: staging reports the landed SHA; the guest log at boot has
   `capsule executor: egress tunnel identity observe`.
2. Run `bash scripts/capsule-tunnel-identity-probe.sh`. It runs `pip
   download` of `six==1.16.0` from `https://pypi.org/simple`, `npm view
   left-pad version` against `https://registry.npmjs.org/`, `git ls-remote
   https://github.com/golang/go HEAD`, a `huggingface_hub` install and one
   `hf_hub_download("openai-community/gpt2", "config.json")`, each pointed
   at the public host so a mirror grant does not route it around CONNECT.
   Expect every toolchain check `"got":"pass"` and `"failures":0`.
3. The mismatch, in the same script: a TLS client CONNECTs to github.com and
   sends the server name pypi.org. Under observe the line reads `HTTP/1.1
   200 Connection Established | SSLCertVerificationError` (the tunnel was
   relayed and github.com answered with its own certificate).
4. Read the capsule's egress ledger
   (`/mnt/persistent/capsule-artifacts/egress/<capsule>.jsonl` in the guest,
   or the `capsule-egress:` lines in the guest log) and count verdicts per
   host:
   `jq -s 'map(select(.method=="CONNECT")) | group_by(.host) | map({host:.[0].host, n:length, pass:(map(select(.tunnel_identity=="pass"))|length), would_refuse:(map(select(.tunnel_identity=="would_refuse"))|length)})'`.
   Expect `would_refuse` 0 for every host but github.com (pypi.org,
   files.pythonhosted.org, registry.npmjs.org, huggingface.co and any hf.co
   storage host), with every toolchain entry `pass` and outcome `ok`; and
   for github.com, `would_refuse` exactly 1 (git's tunnels are `pass`): the
   mismatch, outcome `ok`, with a `tunnel_identity_reason` naming the SNI
   `pypi.org` and the CONNECT host `github.com`. The guest log has one `tunnel identity
   (observe) would refuse capsule <id> host github.com` line.

If a toolchain records `would_refuse`, the reason says which (no SNI, an
Encrypted Client Hello, a split hello); record the tool and the reason here
before any flip.

**The enforce flip** is a later deploy: set
`CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY = "enforce"` in `nix/autoputer-vm.nix`
once the observe ledger shows zero `would_refuse` for real toolchains (this
probe, and the test computer's ordinary engineering traffic). Its proof is the
same script: the toolchains pass, the mismatch line ends in `SSLEOFError` or
`ConnectionResetError`, and the ledger entry is outcome `refused` with
`tunnel_identity` `refused` and no bytes down. Rollback at either stage:
`off` (or `observe`) in the same line and a deploy; `git revert e0efea302`
returns to enforce-always; reverting `e20c92b6c` as well removes the check.
SEC-06's CONNECT variant counts as repaired only at the enforce flip; until
then observe closes nothing.
