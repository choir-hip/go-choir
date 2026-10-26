# Patch Watch fails silently, and three security findings sit open (2026-10-26)

Status: documented; fixes in flight. Found by FP0 reading a failed Patch Watch
run (64948b9a2, 13:00 UTC). Security comes first (owner, 2026-10-25/26).

## 1. The govulncheck step dies before it reports

`.github/workflows/patch-watch.yml:143-151` runs
`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./... > govuln.txt` under
GitHub's default `bash -e -o pipefail`; `set -uo pipefail` does not clear `-e`.
govulncheck exits 3 when it finds anything, so the step fails before it writes
the report line or sets `count`, and the "Patch SLA: govulncheck reachable
findings" issue is never upserted. Patch Watch has failed every day since at
least 2026-10-22 (runs on 4bc3aa3ec, ff5cb4cbb, 11049e60a, 03b6cba09,
64948b9a2), and nobody saw the finding. Fix: capture the exit status without
`-e` (`status=0; govulncheck ... || status=$?`).

## 2. A reachable standard-library vulnerability

Local run on main (b20406ec2), Go 1.26.9: `GO-2026-6809`, found in
`crypto/x509@go1.26.9`, fixed in `crypto/x509@go1.26.10`; reachable from Choir
code. Fix: the Go toolchain lane (flake.nix `choirGoRelease`, go.mod
`toolchain`, ci.yml go-version pins), 1.26.9 to 1.26.10.

## 3. Two known-exploited kernel CVEs await a human check (issue #73, sev-S0)

Open since 2026-10-23. Host and guest declare 6.18.57; the longterm head is
6.18.58. CVE-2025-39964 and CVE-2026-53266 (both KEV 2026-09-18) have no kernel
CNA record, so the workflow cannot say whether 6.18.57 carries the fix. Five
pre-2024 KEV entries are assumed fixed before 6.18 and also say "human check".
Next: determine from the kernel's stable changelogs whether 6.18.57 is fixed;
if not, bump to 6.18.58 or newer.

## 4. Stale Firecracker issues

Issues #72 and #79 say Firecracker is behind (1.15.1 and 1.16.1 to 1.17.0); the
host has run 1.17.0 since 1fc348938. The two Firecracker advisories in #73
(CVE-2026-5747, CVE-2026-1386) need a check that 1.17.0 carries their fixes.

## Update 13:2x: the kernel and Firecracker items are clear

A read-only check against NVD, the CVE.org record API, the Debian tracker,
cdn.kernel.org changelogs and the Firecracker advisories found 6.18.57 fixed for
all seven: CVE-2025-39964 (AF_ALG, fixed in mainline 6.17, so in the 6.18 base),
CVE-2026-53266 (ebt_snat, fixed in 6.18.36), and the five pre-2024 entries (each
fix in v6.18 by tag containment). Both kernel CNA records exist; the watch's
"no CNA record" lookup missed them, so issue #73's S0 premise is wrong. Firecracker
1.17.0 is past both advisories (CVE-2026-5747 fixed in 1.14.4/1.15.1 and 1.16.0;
CVE-2026-1386 fixed in 1.13.2/1.14.1 and 1.15.0); Choir does use virtio-pci
(`--enable-pci` on every launch) and the jailer, so both applied and both are
fixed. Remaining: the watch's CVE lookup should read the CVE.org record API; issues
#72, #73 and #79 can be closed with this evidence (owner's repository; left for
the owner); 6.18.58 is optional hygiene for a maintenance window
(`nix/kernel-security-floor`, then `nix flake update nixpkgs-kernel`).
