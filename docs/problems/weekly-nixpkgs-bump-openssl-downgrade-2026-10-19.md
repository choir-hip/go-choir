# Weekly nixpkgs bump: OpenSSL goes 3.6.2 -> 3.5.8, a downgrade (2026-10-19)

Mutation class: orange (host closure and guest image composition; no runtime
behavior change intended beyond the OpenSSL library line). Protected surfaces:
none directly; the change decides which OpenSSL enters the Node B host closure
and the guest image when the weekly bump (PR #74) is merged. Merging is a
deploy and is the owner's act; this work touches only branch
`patch/weekly-nixpkgs`.

Owner rule: Choir takes upgrades, never downgrades.

Heresy delta: `discovered` 3 (below), `introduced` 0, `repaired` 0 at the time
of this commit (the repair is recorded in the decision section when it lands).

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

Partly. The row reads `nixosConfigurations.go-choir-b.pkgs.openssl.version`:
the package-set default of the host configuration. That is exactly what moved
(the guest uses the same default), so the 3.6.2 -> 3.5.8 move is real for every
package that takes `openssl` from the set. But it is not an inventory of what
ships:

- the closure contains more than one OpenSSL. Instantiating the bump's host
  toplevel and guest image shows `openssl-3.5.8` AND `openssl-3.6.2` (two
  derivations each). The 3.6.2 one comes from the pinned `nixpkgs-qdrant` input
  (rev `ec84054`, the qdrant lane from the previous problem doc): qdrant and
  its dependencies are taken whole from the deployed nixpkgs, so the older
  3.6.2 stays in the host closure;
- packages can pin a specific line independently of the default.

The workflow therefore also gets a step that lists every OpenSSL library store
path in the built host closure and guest image and the closure members that
reference it. That output is the evidence for "which version does the closure
carry", not the single `pkgs.openssl` row.

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

Cost numbers are measured with `nix build --dry-run` for
`go-choir-b ... toplevel` plus `guest-image` (x86_64-linux), cache.nixos.org as
the substituter.

| option | result | cost |
|---|---|---|
| A. Accept 3.5.8 | downgrade by version number; fewer open advisories than 3.6.2 but 13 open, 1 High | zero; violates the owner rule |
| B. Global overlay `openssl = prev.openssl_3_6` (3.6.5) | closure at 3.6.5 everywhere | **2462 derivations to build (baseline 238), 4140 paths / 17.9 GiB fetched (baseline 1261 / 3.7 GiB).** A global override rebuilds everything that links `openssl`, which includes rustc and cargo, systemd, curl (and with it git, cmake, libgit2 ...), python3 (24 environments), nodejs, gradle, ruby gems. Hours on a hosted runner against a 240 minute job timeout; rejected |
| C. Targeted overrides of the OpenSSL consumers that run in the Choir closure (`openssh`, `systemd`, `python3`/searxng, `nodejs`, `socat`, `rsync` ... decided from the closure inventory) with `openssl_3_6` | the network-facing daemons use 3.6.5; default stays 3.5.8 for the rest | per package rebuild only; exact set and cost recorded in the decision below once measured |
| D. Wait for 3.5.9 in nixos-unstable | clean, cached, but 3.6.2 -> 3.5.9 is still a downgrade by number | zero build cost; violates the owner rule |
| E. Move to `openssl_4_0` (4.0.3) | newest, supported to 2027-05 | major version, API removals; mass rebuild like B; no |

## Decision

Pending the closure inventory (the new workflow step), then a targeted
override (option C) chosen from facts, with the build cost stated. Option B is
rejected on cost. The 2026-11-01 end of support of 3.6 is recorded as a
residual: the override is a `max(default, openssl_3_6)` expression, so it
disappears by itself once the default catches up, and a later weekly bump must
revisit it before or when 3.6 loses support (a 3.5.9-or-newer default is then
the correct, supported floor, and the "never downgrade" rule must be reconciled
with it by the owner).

## Residuals

- 3.6 EOL 2026-11-01 (above).
- The qdrant lane keeps an older nixpkgs closure (OpenSSL 3.6.2, 41 open
  advisories) in the host closure for as long as it applies. If qdrant links
  that OpenSSL at runtime, the lane leaves a vulnerable library on the host; the
  inventory step shows whether it does. Retiring the lane when the base rustc
  uses LLVM 22 resolves it; until then, overriding qdrant's OpenSSL is a
  separate decision.
