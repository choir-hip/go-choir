# Deploy G failed: a new vendored package left vendorHash stale, and the VM mirror cannot see it (2026-10-22)

Found by FP0, 06:47 UTC. Status: documented; fix in the next commit.

## Evidence

CI 39734752229 for a077df91e (deploy G) failed in "Prebuild Deploy Artifacts
(Node B)" and "Build Differential SBOM Candidate", so "Deploy to Staging"
failed with nothing deployed (staging kept 8fabec36a). The builder's error:

    internal/pkgmirror/server.go:31:2: cannot find module providing package
    golang.org/x/crypto/blake2b: import lookup disabled by -mod=vendor

## Cause

NET-02 (lane/fp10-net02-mirrors, landed in deploy G) imports
`golang.org/x/crypto/blake2b`, a package no other code imported. The module
was already required (go.mod: golang.org/x/crypto v0.57.0), so go.mod and
go.sum did not change, but `go mod vendor` vendors only imported packages, so
the vendored tree gains `golang.org/x/crypto/blake2b` and `vendor/modules.txt`
changes. flake.nix `commonGoArgs.vendorHash` is the NAR hash of that tree; it
was not updated, and the Nix build runs with `-mod=vendor`.

FP0's VM mirror (`~/ci-mirror.sh`) builds and tests from the module cache,
not from a vendored tree, and has no Nix, so it passed the lane (142a97716).
Reviewers read code, not the vendor set. Nothing before CI's Nix build
checks that the vendored package set is unchanged.

## Reproduction of the hash without Nix builds

`go mod vendor -o DIR` then `nix hash path DIR` reproduces the flake's hash:
on 96a82ae33 (before NET-02) it gives the current
`sha256-JyLmVWCFKfjqtNkY2Ly6MrfVzQntoR//Goi5H8dOlq0=`; on 40cd51ee0 it gives
`sha256-VcX33AW1t9TO6vPVPKqmZqYmpPgy0bEGc/YI+VAhVrk=`, and the only
difference between the two trees is `golang.org/x/crypto/blake2b` plus
`modules.txt`. The vendored tree does not depend on the host platform, so the
Mac's Go 1.26.9 gives the same hash as the builder's.

## Fix

1. flake.nix `commonGoArgs.vendorHash` to the new hash (next commit).
2. The VM mirror compares the vendored package list (`vendor/modules.txt`
   from `go mod vendor`) of the lane against origin/main and fails with
   "vendorHash must be recomputed" when it differs, so this is caught before
   landing.

## Heresy delta

discovered: the pre-landing check cannot see vendored-package changes;
introduced: none; repaired: none in this commit.
