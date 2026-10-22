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
# The revert removes this script and its artifact from the tree (deploy 1
# added both), so the rollback reads them from the deploy-1 tip by git show
# (round 1d, DeepSeek). The sequence, at the repo root, TIP being the last
# deploy-1 commit on main:
#
#   T=$(mktemp -d)
#   git show "$TIP:scripts/rollback/fc-deploy1-rollback.sh" >"$T/fc-deploy1-rollback.sh"
#   git revert --no-edit <deploy 1's commits>
#   bash "$T/fc-deploy1-rollback.sh" apply --from "$TIP"
#   git add internal/store/store.go internal/store/fc_deploy1_rollback.go
#   git commit -m "rollback below F-c deploy 1: drop an empty choir_events at boot"
#
# Modes:
#   apply [--from REV]
#       After the revert (refuses while choirEventsDDL is still defined):
#       writes the artifact (from REV with git show, else from the working
#       tree) to internal/store/fc_deploy1_rollback.go, inserts the boot
#       substep into bootstrap after the retiredRuntimeTables loop, and builds
#       internal/store.
#   patch-bootstrap IN OUT
#       Writes IN (a store.go) with the boot substep inserted to OUT. The
#       store tests use it to build the package with the step in place.
#   rehearse TIP BASE
#       Runs the sequence above in a throwaway clone (git clone --shared, in
#       a temp directory; nothing in this tree changes): checks out TIP,
#       extracts this script from TIP, reverts BASE..TIP without
#       committing, applies --from TIP, then checks that the tree has no
#       choirEventsDDL, no artifact under testdata, the step in bootstrap,
#       and that go build and go vet of internal/store pass.
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
	from=""
	if [ "${2:-}" = --from ]; then
		[ -n "${3:-}" ] || { echo "apply --from needs a revision" >&2; exit 2; }
		from=$3
	fi
	[ -f go.mod ] && [ -f internal/store/store.go ] || { echo "run at the repo root" >&2; exit 2; }
	if grep -q 'choirEventsDDL' internal/store/*.go; then
		echo "internal/store still defines choirEventsDDL: revert deploy 1's commits first" >&2
		exit 2
	fi
	if grep -q 'rollbackRecipeDropEmptyChoirEvents' internal/store/store.go; then
		echo "bootstrap already carries the step" >&2
		exit 2
	fi
	tmp=$(mktemp)
	if [ -n "$from" ]; then
		git show "$from:$ARTIFACT" >"$tmp" || { echo "no $ARTIFACT at $from" >&2; rm -f "$tmp"; exit 2; }
	elif [ -f "$ARTIFACT" ]; then
		cp "$ARTIFACT" "$tmp"
	else
		echo "$ARTIFACT is not in the tree (the revert removed it): use apply --from <deploy-1 tip>" >&2
		rm -f "$tmp"
		exit 2
	fi
	patched=$(mktemp)
	patch_bootstrap internal/store/store.go "$patched"
	mv "$patched" internal/store/store.go
	mv "$tmp" internal/store/fc_deploy1_rollback.go
	gofmt -l internal/store/store.go internal/store/fc_deploy1_rollback.go | sed 's/^/not gofmt-clean: /'
	go build ./internal/store
	echo "applied: commit internal/store/store.go and internal/store/fc_deploy1_rollback.go with the revert"
	;;
patch-bootstrap)
	[ $# -eq 3 ] || { echo "usage: $0 patch-bootstrap IN OUT" >&2; exit 2; }
	patch_bootstrap "$2" "$3"
	;;
rehearse)
	[ $# -eq 3 ] || { echo "usage: $0 rehearse TIP BASE" >&2; exit 2; }
	tip=$(git rev-parse --verify "$2^{commit}")
	base=$(git rev-parse --verify "$3^{commit}")
	work=$(mktemp -d)
	trap 'rm -rf "$work"' EXIT
	echo "rehearse: clone into $work/repo, tip $tip, base $base"
	git clone --quiet --shared --no-checkout "$(git rev-parse --show-toplevel)" "$work/repo"
	cd "$work/repo"
	git checkout --quiet --detach "$tip"
	git show "$tip:scripts/rollback/fc-deploy1-rollback.sh" >"$work/fc-deploy1-rollback.sh"
	git revert --no-commit "$base..$tip"
	echo "rehearse: reverted $(git rev-list --count "$base..$tip") commits"
	[ ! -e "$ARTIFACT" ] || { echo "FAIL: the revert left $ARTIFACT" >&2; exit 1; }
	[ ! -e scripts/rollback/fc-deploy1-rollback.sh ] || { echo "FAIL: the revert left the script" >&2; exit 1; }
	echo "rehearse: the revert removed the script and the artifact, as expected"
	bash "$work/fc-deploy1-rollback.sh" apply --from "$tip"
	! grep -q choirEventsDDL internal/store/*.go || { echo "FAIL: choirEventsDDL still defined" >&2; exit 1; }
	[ "$(grep -c 'rollbackRecipeDropEmptyChoirEvents(s.db)' internal/store/store.go)" = 1 ] || { echo "FAIL: the step is not in bootstrap once" >&2; exit 1; }
	cmp -s internal/store/fc_deploy1_rollback.go <(git show "$tip:$ARTIFACT") || { echo "FAIL: the installed file differs from the artifact at $tip" >&2; exit 1; }
	go vet ./internal/store
	echo "rehearse: PASS (revert, apply --from $tip, go build and go vet of internal/store)"
	git diff --stat HEAD -- internal/store/store.go internal/store/fc_deploy1_rollback.go | tail -1
	;;
*)
	sed -n '2,51p' "$0" >&2
	exit 2
	;;
esac
