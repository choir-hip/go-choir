#!/usr/bin/env bash
# Rollback below F-c deploy 1 (run telemetry in its own table):
# docs/design/choir-event-table-f-c-2026-10-22.md, status section.
#
# A bare git revert of deploy 1 boots the older binary over stores that
# deploy 1 opened; its replay manifest calls the empty choir_events unknown,
# so checkpoint publication stays refused until a forward deploy. The
# revert commit must therefore also carry one boot step that drops
# choir_events while it is empty. This script adds it.
#
#   scripts/rollback/fc-deploy1-rollback.sh apply
#       At the repo root, after `git revert` of deploy 1's commits (the
#       working tree must no longer define choirEventsDDL): copies
#       internal/store/testdata/fc-deploy1-rollback/fc_deploy1_rollback.go
#       to internal/store/fc_deploy1_rollback.go, inserts the boot substep
#       into bootstrap after the retiredRuntimeTables loop, and builds
#       internal/store. Commit the two files with the revert.
#   scripts/rollback/fc-deploy1-rollback.sh patch-bootstrap IN OUT
#       Writes IN (a store.go) with the boot substep inserted to OUT. The
#       store tests use it to build the package with the step in place.
#
# Below the rollback floor (any op ever stamped) the step keeps the table and
# logs; the repair there is forward (design section 4.5).
set -euo pipefail

ARTIFACT=internal/store/testdata/fc-deploy1-rollback/fc_deploy1_rollback.go

patch_bootstrap() { # IN OUT
	awk '
		BEGIN { anchors = 0; inserted = 0 }
		{ print }
		/^\tfor _, table := range retiredRuntimeTables \{$/ { anchors++; inloop = 1; next }
		inloop && /^\t}$/ {
			inloop = 0
			print "\t// Rollback below F-c deploy 1: drop choir_events while it is empty"
			print "\t// (fc_deploy1_rollback.go; design choir-event-table-f-c-2026-10-22.md)."
			print "\tif err := bootSubstep(\"runtime-schema\", \"drop-empty-choir-events\", func() error { return rollbackRecipeDropEmptyChoirEvents(s.db) }); err != nil {"
			print "\t\treturn err"
			print "\t}"
			inserted++
		}
		END {
			if (anchors != 1 || inserted != 1) {
				printf "fc-deploy1-rollback: expected one retiredRuntimeTables loop in bootstrap, found %d (inserted %d)\n", anchors, inserted > "/dev/stderr"
				exit 3
			}
		}' "$1" >"$2"
}

case "${1:-}" in
apply)
	[ -f go.mod ] && [ -f "$ARTIFACT" ] || { echo "run at the repo root of a tree that has $ARTIFACT" >&2; exit 2; }
	if grep -q 'choirEventsDDL' internal/store/*.go; then
		echo "internal/store still defines choirEventsDDL: revert deploy 1's commits first" >&2
		exit 2
	fi
	if grep -q 'rollbackRecipeDropEmptyChoirEvents' internal/store/store.go; then
		echo "bootstrap already carries the step" >&2
		exit 2
	fi
	tmp=$(mktemp)
	patch_bootstrap internal/store/store.go "$tmp"
	mv "$tmp" internal/store/store.go
	cp "$ARTIFACT" internal/store/fc_deploy1_rollback.go
	gofmt -l internal/store/store.go internal/store/fc_deploy1_rollback.go | sed 's/^/not gofmt-clean: /'
	go build ./internal/store
	echo "applied: commit internal/store/store.go and internal/store/fc_deploy1_rollback.go with the revert"
	;;
patch-bootstrap)
	[ $# -eq 3 ] || { echo "usage: $0 patch-bootstrap IN OUT" >&2; exit 2; }
	patch_bootstrap "$2" "$3"
	;;
*)
	sed -n '2,24p' "$0" >&2
	exit 2
	;;
esac
