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
   and a client that has not sent a complete first record within ten
   seconds (one deadline, not renewed by partial reads). It applies to every
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
  relays the record's bytes and the tunnel unchanged (but not at once: see
  the known cost below). The entry's `outcome` stays
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
observe: nothing is relayed either way until the first record is judged, so
a client that has not sent a complete first record (a silent client, or one
waiting for a server that speaks first) is relayed only after the 10 s hello
timeout; TLS clients speak first and are judged as soon as the hello arrives.
`TestEgressObserveDelaysAServerFirstGreetingUntilTheTimeout` pins it.

Commits on `lane/fp6-sec06-on-main`: `e20c92b6c` (the replay, red),
`d8d88ae5f` (tests first, yellow), `e0efea302` (the mode, red, full
ceremony in its message), and the docs commit carrying this section and
`scripts/capsule-tunnel-identity-probe.sh`.

The deployed proof first written here is superseded by the runbook in
"Review round and per-computer enforce (2026-10-22)" below, which adds the
mirror precondition, the enforce computer and the Claude delegate gate.

## Review round and per-computer enforce (2026-10-22, station FP6)

The panel on `3b5c9e982` (Flash accept; DeepSeek and Sonnet accept with
edits, outputs outside the repo) and the owner rule of 2026-10-22 ("a new
enforcing rule ships in observe everywhere, plus enforce on one selected test
computer in the same deploy, where a per-computer selector exists") changed
the slice as follows. Commits: `824a3f09e` (tests, yellow), `e97b1b0db`
(code, red, ceremony in its message), then the docs commit with this section.

**Per-computer enforce.** vmctl's `VM_CAPSULE_TUNNEL_ENFORCE_ONLY`
(`internal/vmmanager/capsule_tunnel_identity.go`) takes a list of computer
ids and puts the kernel boot argument
`choir.capsule_egress_tunnel_identity=enforce` on the launch of each computer
it names. The guest boot script in `nix/autoputer-vm.nix` copies only that
exact value into `/run/go-choir-autoputer.env`, and that environment file
overrides the unit's `observe`. No guest process or capsule can set its own
boot arguments. Boot arguments are part of the launch-config hash, so a
newly named computer cold boots once; the boot arguments of computers not
named stay byte-identical. `nix/node-b.nix` sets the list empty; FP0 names one
test computer at landing. Name computer ids only. The shared selector
(`computer_select.go`) also takes `class:<class>`, but a `class:` token
misses the platform computer's boot and the resume config, which never set
the warmness class. `*` is the global flip, which goes through the gate below
instead. Rollback: empty the list and redeploy. The named computer's launch
config hash moves back, so it cold boots into observe
(`TestCapsuleTunnelEnforceSelectionRefusesAnObserveSnapshot` pins the hash
link).

**Other edits.** The name comparison lowercases ASCII only (it was
`strings.EqualFold`; Unicode folding matched `ſ.github.com` to
`s.github.com`). The daemon logs a WARNING at start for a set value other
than off, observe or enforce. Only a checked CONNECT gets the full-record
reader (16901 bytes); other requests keep 4 KiB. New tests kill the
reviewers' surviving mutants. M32: an upstream-side byte counter shows 0 bytes
on every enforce refusal. M30: valid 9 KiB and 16384-byte hellos pass in both
modes. M13, M20, M28: a duplicate extension, bytes after the extension block,
and bytes after the SNI list are all refused. The image line and the boot-script
case are pinned by tests.

**Scope.** The check binds name to name. It does not see an HTTP Host
header inside the TLS session, so even under enforce it closes only the
plain variant of this problem (a ClientHello naming another host). The
durable fix is the fetch service per ecosystem (NET-02). Exact host matching
is still the named residual.

**Runbook for FP0's deployed proof (run on deploy S, ddd20de0e, 2026-10-22 16:36 UTC: steps 1 to 4 pass; see "Deployed proof" below).**
Precondition: each computer used has no package-mirror grant, or its grant
is not `mirrors_only`. Under `mirrors_only` the proxy refuses CONNECT to
pypi.org, registry.npmjs.org and github.com with 403, and no tunnel is judged.

1. Identity: staging reports the landed SHA. vmctl's environment has
   `VM_CAPSULE_TUNNEL_ENFORCE_ONLY=<one test computer id>`. That computer has
   cold booted since; its guest log says `capsule executor: egress tunnel
   identity enforce`. Every other computer's guest log says `... observe`
   and carries no WARNING line.
2. Observe, on a second test computer (not the one selected), from an
   implementation capsule (a capsule job): `bash
   scripts/capsule-tunnel-identity-probe.sh observe`. The script runs `pip
   download six==1.16.0` from `https://pypi.org/simple`, `npm view left-pad
   version` against `https://registry.npmjs.org/`, `git ls-remote
   https://github.com/golang/go HEAD` with the mirror's `GIT_CONFIG_*`
   insteadOf rewrite dropped for that command, a `huggingface_hub` install,
   and `hf_hub_download("openai-community/gpt2", "config.json")`. Expected:
   every check `"got":"pass"`; the `sni-mismatch` line shows `HTTP/1.1 200
   Connection Established | SSLCertVerificationError` (the tunnel was relayed
   and github.com answered with its own certificate); `"failures":0`.
3. Read that capsule's egress ledger: either
   `/mnt/persistent/capsule-artifacts/egress/<capsule>.jsonl` in the guest,
   or the `capsule-egress:` lines in the guest log. Count verdicts per host:
   `jq -s 'map(select(.method=="CONNECT")) | group_by(.host) | map({host:.[0].host, n:length, pass:(map(select(.tunnel_identity=="pass"))|length), would_refuse:(map(select(.tunnel_identity=="would_refuse"))|length)})'`.
   Expected on a computer with no mirror grant:
   - `would_refuse` 0 on every host except github.com (pypi.org,
     files.pythonhosted.org, registry.npmjs.org, huggingface.co, and any hf.co
     storage host), with every toolchain entry `pass` and outcome `ok`;
   - on github.com, git's tunnels `pass` and exactly one `would_refuse`:
     outcome `ok`, with a `tunnel_identity_reason` naming the SNI `pypi.org`
     and the CONNECT host `github.com`;
   - one `tunnel identity (observe) would refuse capsule <id> host
     github.com` line in the guest log.
4. Enforce, on the selected computer, from an implementation capsule: `bash
   scripts/capsule-tunnel-identity-probe.sh enforce`. Expected: the
   toolchain checks pass, and the `sni-mismatch` line shows `HTTP/1.1 200
   Connection Established` followed by `SSLEOFError`, `SSLZeroReturnError`,
   `ConnectionResetError` or `BrokenPipeError`. The ledger's toolchain
   entries are `pass`. The mismatch entry is outcome `refused` and
   `tunnel_identity` `refused`, with the SNI reason, `bytes_up` 0 and
   `bytes_down` 0.
5. If a toolchain records `would_refuse` (or `refused` on the selected
   computer), the reason says which case it is: no SNI, an Encrypted Client
   Hello, a split hello, or an incomplete first record. Record the tool and
   the reason here, and empty the selector if it was the selected computer.

**The global enforce gate.** Enforce does not become the image default (`CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY
= "enforce"` in `nix/autoputer-vm.nix`, a later deploy) until all three
hold:
(a) the observe ledgers show zero `would_refuse` for real toolchains: this
probe, and ordinary engineering traffic on the test computers;
(b) the selected computer has run enforce with its toolchains passing;
(c) a real claude-mode (Claude delegate) capsule session shows
`tunnel_identity` `pass` for `api.anthropic.com` on its own ledger. The
delegate's client stack (Bun/Node) is not exercised by the probe, and one
GREASE ECH from it would end every delegate session under enforce. Do not
select the Claude delegate's allowlisted computer for enforce before (c)
holds.

Rollback at any stage. For a named computer, the deterministic rollback is
emptying the selector (or removing its id) and redeploying. Its launch config
hash changes, so it cold boots without the boot argument and runs the image's
mode. Setting the image line to `off` is not enough there: the boot script's
environment file overrides the unit's `Environment=`, so a computer that is
still named stays in enforce. For every other computer, set `off` (or
`observe`) in the image line and deploy. Also: `git revert e97b1b0db` removes the selector and this
round's edits; `git revert e0efea302` returns to enforce everywhere;
reverting `e20c92b6c` as well removes the check. SEC-06's plain CONNECT
variant counts as repaired only where enforce runs: on the selected computer
at landing, everywhere at the global flip. Until then, observe closes
nothing.

## Deployed proof (2026-10-22, deploy S, ddd20de0e)

Run by FP0 on test computers only: observe on computer-71afbf14 and enforce
on computer-9648031f, the computer `VM_CAPSULE_TUNNEL_ENFORCE_ONLY` names.

- Step 1 passes. 71afbf14 booted on S with `capsule executor: egress tunnel
  identity observe` and 9648031f with `... enforce`. Neither computer has a
  package-mirror grant.
- Steps 2 and 3 pass on 71afbf14. Every toolchain tunnel was `pass`
  (pypi.org, files.pythonhosted.org, registry.npmjs.org, huggingface.co, and
  git's tunnels to github.com). Exactly one `would_refuse` on github.com,
  outcome `ok`, reason `ClientHello SNI "pypi.org" is not the CONNECT host
  "github.com"`, and one `tunnel identity (observe) would refuse` line.
- Step 4 passes on 9648031f. The toolchain tunnels were `pass`; the mismatch
  was `refused` with the same reason, `bytes_up` 0 and `bytes_down` 0.

**Runbook correction.** A capsule's platform source is the computer's own
pinned source, which can lag the deployed commit: on 71afbf14 it was
840e1a701, without this script, so the first run failed with "No such file
or directory". Run the probe from a copy fetched inside the capsule at the
deployed commit (`git init`, then `git fetch --depth 1 --filter=blob:none
https://github.com/choir-hip/go-choir <deployed sha>` and `git show
FETCH_HEAD:scripts/capsule-tunnel-identity-probe.sh`), and check its sha256.
Two other routes failed: the script pasted into the prompt (the prompt bar's
Texture validator rejects its bracketed indices as legacy citations and
answers HTTP 500), and the script as base64 (the model did not copy the long
string exactly).

The global enforce gate above still needs the observe ledgers to show zero
`would_refuse` for real toolchains over time, and a Claude-mode capsule's
`api.anthropic.com` tunnel to read `pass`.

