# Guest release policy (nix/guest-release-policy.json)

JSON has no comments, so the rules for the three numbers live here. Design:
docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md 3.1 and 6.6.
`flake.nix` writes them into every guest image's `release.json`; vmctl
compares a hibernated computer's release with the current one at every wake.

- **security_epoch.** A monotone integer. Raise it in the commit that carries
  a security fix to guest code outside the floor (the runtime, a desk, a
  credential or authority path; the prefixes are in
  nix/guest-security-surfaces.txt). It never falls: a revert of a commit that
  raised it keeps the raised value. Raising it makes every older release-bound
  snapshot cold boot onto the fixed release. A fix to floor code needs no
  raise: it moves `floor_digest` by itself. `scripts/check-guest-security-epoch`
  refuses a change to a named surface that neither raises it nor carries the
  reviewed `Security-Effect` trailers.
- **guest_abi.** The host-guest protocol version this build speaks.
- **guest_abi_min.** The oldest guest protocol this build's host services
  still serve. Raise it in the commit that makes a host service stop serving
  a request shape an older runtime sends.
