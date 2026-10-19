# Weekly nixpkgs bump: OpenSSL goes 3.6.2 -> 3.5.8, a downgrade (2026-10-19)

Mutation class: orange (host closure and guest image composition; no runtime
behavior change intended beyond the OpenSSL library line). Protected surfaces:
none directly; the change decides which OpenSSL enters the Node B host closure
and the guest image when the weekly bump (PR #74) is merged. Merging is a
deploy and is the owner's act; this work touches only branch
`patch/weekly-nixpkgs`.

Owner rule: Choir takes upgrades, never downgrades.

Heresy delta: see the Decision section.

## Observation

The version table that `.github/workflows/flake-bump.yml` posts on PR #74
(run 39310887940, head `d27538d1a`) reports:

| package | main | this PR |
|---|---|---|
| openssl | 3.6.2 | 3.5.8 **(moved)** |

Every other row is equal or newer (see "Other rows" below).

## Cause (verified by evaluating both nixpkgs revisions)

`pkgs.openssl` is an alias in `pkgs/top-level/all-packages.nix`. Evaluated
with `nix eval` against both revisions:

| attribute | deployed lock `ec84054` | bump `fe13242` (26.11.20261015) |
|---|---|---|
| `openssl` | 3.6.2 (= `openssl_3_6`) | **3.5.8 (= `openssl_3_5`)** |
| `openssl_3_5` | 3.5.7 | 3.5.8 |
| `openssl_3_6` | 3.6.2 | 3.6.5 |
| `openssl_4_0` | 4.0.1 | 4.0.3 |
| `openssl_3` (3.0) | 3.0.21 | removed ("end of life") |

nixpkgs deliberately moved the default back to the 3.5 LTS line:
**NixOS/nixpkgs PR #564262, "openssl: downgrade default to 3.5 LTS"**
(https://github.com/NixOS/nixpkgs/pull/564262, author adamcstephens, merged
2026-09-17, merge commit `95437efd1d158ad64dbe157cbe3f888204296302`). The
whole diff is one line, `-  openssl = openssl_3_6;` / `+  openssl = openssl_3_5;`.
Rationale in the PR body: "3.6.x will go out of support in November before
branch off. I suggest this is a simpler change than trying to bring all of
the tree up to 4.x." So it is not an accident and not a stale snapshot: the
nixos-26.11 branch-off is meant to ship an LTS default, and unstable already
carries it. OpenSSL's own policy page: 3.6 is supported until 2026-11-01
(13 months, non-LTS), 3.5 until 2030-04-08 (LTS), 4.0 until 2027-05-14.

Our own code does not override OpenSSL anywhere: `grep -rn openssl flake.nix
nix/` finds only `${pkgs.openssl}/bin/openssl` uses in `nix/node-b.nix` (token
and key generation) and no overlay, `override`, or `overrideAttrs` on it. The
host (`go-choir-b`) and guest (`go-choir-autoputer-vm`) both use the unmodified
default.

## Is the table mis-measuring?

No, but it is a proxy. The row reads
`nixosConfigurations.go-choir-b.pkgs.openssl.version`: the package-set default
of the host configuration, which the guest (`go-choir-autoputer-vm`) shares.
The workflow now also prints what the built closures contain (step "OpenSSL
actually in the closure", added in this branch; first output posted on PR #74
from run 39320308935). Result for the unchanged bump:

- host closure: exactly one OpenSSL library, `openssl-3.5.8` (plus `-bin`,
  `-dev`);
- guest image: exactly one, `openssl-3.5.8`;
- about 55 closure members link it, including openssh, systemd, curl, git,
  python3 3.14.7 and python3.14-cryptography, nodejs-slim, rsync, socat,
  libreoffice, pipewire and the gst plugins (the guest installs nodejs and
  libreoffice on purpose, `nix/autoputer-vm.nix`).

So the move is real and total: every OpenSSL consumer that ran 3.6.2 on main
runs 3.5.8 after the merge. (An older `openssl-3.6.2` derivation is in the
build-time derivation graph because the qdrant lane takes qdrant whole from the
pinned `nixpkgs-qdrant` input; it is not in the runtime closure, so qdrant does
not leave a vulnerable OpenSSL on the host.) Go services and caddy use Go's own
crypto and are unaffected; qdrant does not link OpenSSL.

## Security comparison (openssl-library.org/news/vulnerabilities, fetched 2026-10-19)

Versions evaluated against each advisory's "affected from X before Y" ranges
for the CVEs published in 2026:

| version | 2026 advisories still affecting it |
|---|---|
| 3.6.2 (deployed on main) | 41 (2 High: CVE-2026-45447 PKCS7_verify use-after-free, CVE-2026-84782 DTLS stale-buffer retransmit; 8 Moderate) |
| 3.5.7 (deployed lock's 3.5) | 23 |
| **3.5.8 (bump default)** | **13**, all from the 2026-09-29 batch (1 High: CVE-2026-84782; rest Low) |
| 3.5.9 (nixpkgs master, not yet in nixos-unstable) | 0 |
| **3.6.5 (`openssl_3_6` on the bump)** | **0** |

Findings:

1. 3.5.8 is not equivalent to 3.6.2 in "newness", but it is not worse in
   security: the 3.5 line received the June and August 2026 fixes (3.5.7/3.5.8)
   that 3.6.2 lacks. 3.5.8 is missing the 2026-09-29 batch (fixed in 3.5.9 and
   3.6.5), including CVE-2026-84782 (High, DTLS).
2. The deployed 3.6.2 is itself well behind: 3.6.3, 3.6.4 and 3.6.5 exist and
   fix 41 advisories.
3. The security-optimal choice available in the bump's nixpkgs is
   `openssl_3_6` = **3.6.5**: zero open 2026 advisories. 3.5.9 would also be
   clean but is not in nixos-unstable `fe13242`.
4. 3.6 reaches end of support 2026-11-01, 12 days from today. After that no
   3.6.x fixes are published. This is the reason nixpkgs went back to LTS and
   is a standing cost of choosing 3.6 (see decision).

## Other rows in the PR #74 table

No other downgrades. Moved up: go (base) 1.26.4 -> 1.26.8, firecracker
1.15.1 -> 1.16.1, systemd 260.2 -> 261.3, openssh 10.3p1 -> 10.5p1, glibc 2.42 ->
2.44, caddy 2.11.4 -> 2.11.7, searxng 0-unstable-2026-06-20 ->
0-unstable-2026-10-13, python3 3.13.13 -> 3.14.7 (a minor-line jump, an
upgrade but a behavior-risk one: it is outside this problem), nodejs 24.16.0 ->
24.21.0. Unchanged: host kernel 6.18.55 (separate lane), go (choirGo) 1.26.9,
dolt 2.4.2, qdrant 1.18.2 (pinned to the deployed build by the qdrant lane).
Note that python3 in the closure is 3.14.7 while `python3-3.13.15` also appears
as a build input of some dependents.

## Options

Costs are measured with `nix build --dry-run` for the `go-choir-b` toplevel
plus `guest-image` (x86_64-linux) against cache.nixos.org. Baseline (the bump as
it is): 238 derivations to build (Choir's own services and NixOS glue), 1261
paths / 3.7 GiB fetched; the run took about 17 minutes. There is no Choir
binary cache (a search for cachix, attic and substituters in the workflows and
nix files finds none), so any non-nixpkgs derivation is built from source in CI
and again on Node B.

| option | result | cost |
|---|---|---|
| A. Accept 3.5.8 as nixpkgs ships it | version number goes down; open 2026 advisories go **down** 41 -> 13 | zero; breaks the owner's "never downgrade" rule by number, not by security |
| B. Global overlay `openssl = prev.openssl_3_6` (3.6.5; measured by an uncommitted experiment) | whole closure on 3.6.5, 0 open advisories | **2462 derivations to build, 4140 paths / 17.9 GiB fetched.** Rebuilds llvm 21 and 22, clang, rustc and cargo, openjdk 21, gradle, libreoffice, nodejs, python3 (24 environments), torch, systemd, curl (hence the version-control tools, cmake, libgit2), postgresql, duckdb, gdal, pipewire, gst. Many hours from source with no cache against a 240 minute job timeout, and the same again on Node B |
| C. Override only a few consumers (`openssh` measured) | those consumers on 3.6.5; the rest stays 3.5.8 | an `openssh.override { openssl = ... }` overlay takes 278 builds / 2145 paths / 7.1 GiB, because other closure members (libreoffice among them) depend on openssh through the package set; a service-local `services.openssh.package` avoids the cascade but leaves two OpenSSL lines in the runtime closure and the "closure uses >= 3.6.2" invariant is still false. Rejected: more moving parts, no invariant gained |
| D. Wait for 3.5.9 (nixpkgs master has it; nixos-unstable `fe13242` does not) | 0 open advisories, cached | zero; still a number downgrade vs 3.6.2 |
| E. `openssl_4_0` (4.0.3) | newest, supported to 2027-05 | major version with API removals; mass rebuild like B |
| F. Pin base nixpkgs to a revision before PR 564262 (3.6 default) | cached 3.6.x | gives up "current nixos-unstable head" for every other package; forks the bump process |

## Decision (made without a blocking ask, per CLAUDE.md "No Blocking Asks")

Take the conservative option on the branch and state the exception plainly:

1. **Do not override OpenSSL.** B is infeasible (hours of from-source rebuild
   of rustc, llvm, libreoffice, nodejs and torch, twice, for a library line
   that loses support 2026-11-01); C buys no invariant. Options A and D are the
   engineering answer: 3.5 LTS is supported to 2030-04, and the next weekly
   bump brings 3.5.9 (cached, 0 open advisories).
2. **The bump is a version-number downgrade and a security upgrade.** Open 2026
   advisories 41 -> 13; the High PKCS7_verify use-after-free (CVE-2026-45447)
   is fixed. What remains is the 2026-09-29 batch (one High: DTLS stale-buffer
   retransmit, CVE-2026-84782; DTLS is not used by sshd, curl, git, python TLS
   or the Go services). Merging PR #74 therefore needs an explicit owner
   exception to "never downgrade by version number". That is the one open
   decision, recorded as a named residual below. Until the owner ratifies it,
   PR #74 should not be merged.
3. **Make the rule enforceable.** The workflow now marks any version-table row
   whose version sorts below main as `**(DOWNGRADE)**` and repeats the list at
   the top of the PR comment, so a downgrade cannot hide in a long table. The
   closure-level OpenSSL step covers the library itself.

Heresy delta: `discovered` 2 (a silent library downgrade in the weekly bump;
the table measures a proxy), `introduced` 0, `repaired` 1 (downgrades are now
flagged), with the OpenSSL downgrade itself left open as a named owner
decision.

## Residuals

- Owner decision: ratify (or reject) the 3.6.2 -> 3.5.8 version-number
  downgrade on the grounds above. If rejected, the only mechanisms are B or F,
  both costly; neither should be taken without that explicit decision.
- 3.6 reaches end of support 2026-11-01. Any "stay on 3.6" decision expires
  then; this is why the nixpkgs default moved to LTS.
- Re-check at the next weekly bump that `openssl` is >= 3.5.9 (0 open
  advisories) and close this document.
- python3 3.13.13 -> 3.14.7 is a minor-line jump outside this problem; it is an
  upgrade by number and should be reviewed as a behavior change.
