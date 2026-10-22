#!/usr/bin/env bash
# Daily Dolt history audit (the unit go-choir-platform-dolt-history-audit).
# Recurrence control for the 2026-08-26 incident: per-mutation DOLT_COMMIT
# grew reachable commit history that no dolt gc (shallow or --full) can
# collect (docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md).
# A nonzero exit fails the unit: a signal that the squash runbook is due.
#
# With Store A's snapshot commits off (PLATFORM_DOLT_SNAPSHOTS=off) oldgen
# stops growing and growth moves to newgen (noms minus oldgen: an upper bound on the working set's divergence from HEAD, as it also counts the journal and uncollected chunks)
# and the journal, so both have lines too; a journal past its line means
# auto GC is not running (docs/problems/platform-dolt-oldgen-rate-and-squash-
# window-2026-10-22.md, review round 1 R4).
#
#   dolt-history-audit.sh name:dbdir:home ...   (missing repos are skipped)
#
# Lines (KiB unless named): GO_CHOIR_DOLT_MAX_COMMITS 1000000,
# GO_CHOIR_DOLT_MAX_COMMIT_DELTA 10000, GO_CHOIR_DOLT_MAX_OLDGEN_KIB 50 GiB,
# GO_CHOIR_DOLT_MAX_NEWGEN_KIB 8 GiB, GO_CHOIR_DOLT_MAX_JOURNAL_KIB 4 GiB
# (auto GC runs when the journal passes 128 MiB).
set -euo pipefail
journal_name=vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv
for spec in "$@"; do
  name="${spec%%:*}"
  rest="${spec#*:}"
  dbdir="${rest%%:*}"
  home="${rest#*:}"
  [ -d "$dbdir/.dolt" ] || { echo "$name-dolt history audit: skipped (no repo at $dbdir)"; continue; }
  commits=$(cd "$dbdir" && HOME="$home" dolt sql -q "SELECT COUNT(*) FROM dolt_log" 2>/dev/null | grep -oE '[0-9]+' | head -1 || echo 0)
  oldgen_kib=$(du -sk "$dbdir/.dolt/noms/oldgen" 2>/dev/null | cut -f1 || echo 0)
  noms_kib=$(du -sk "$dbdir/.dolt/noms" 2>/dev/null | cut -f1 || echo 0)
  newgen_kib=$(( ${noms_kib:-0} - ${oldgen_kib:-0} ))
  journal_kib=$(du -sk "$dbdir/.dolt/noms/$journal_name" 2>/dev/null | cut -f1 || echo 0)
  echo "$name-dolt history audit: dolt_log commits=${commits} oldgen=${oldgen_kib}KiB noms=${noms_kib}KiB newgen=${newgen_kib}KiB journal=${journal_kib:-0}KiB"
  max_commits="${GO_CHOIR_DOLT_MAX_COMMITS:-1000000}"
  max_oldgen_kib="${GO_CHOIR_DOLT_MAX_OLDGEN_KIB:-52428800}"
  max_newgen_kib="${GO_CHOIR_DOLT_MAX_NEWGEN_KIB:-8388608}"
  max_journal_kib="${GO_CHOIR_DOLT_MAX_JOURNAL_KIB:-4194304}"
  # Commit-rate SLO (2026-09-19, Move 1): per-mutation DOLT_COMMIT was
  # replaced by the debounced snapshot committer, so dolt_log should grow
  # by at most a few thousand commits between daily audit runs. A larger
  # delta means per-mutation commits regressed.
  max_commit_delta="${GO_CHOIR_DOLT_MAX_COMMIT_DELTA:-10000}"
  state_file="$home/history-audit-last-count"
  prev_commits=0
  if [ -f "$state_file" ]; then
    prev_commits=$(cat "$state_file" 2>/dev/null || echo 0)
  fi
  echo "${commits:-0}" > "$state_file.tmp" && mv "$state_file.tmp" "$state_file"
  commit_delta=$(( ${commits:-0} - ${prev_commits:-0} ))
  if [ "${prev_commits:-0}" -gt 0 ] && [ "$commit_delta" -gt "$max_commit_delta" ]; then
    echo "$name-dolt history audit FAILED: dolt_log grew $commit_delta commits since last audit (> $max_commit_delta); per-mutation DOLT_COMMIT regressed — check the debounced snapshot committer (internal/doltbatch)" >&2
    exit 1
  fi
  if [ "${commits:-0}" -gt "$max_commits" ]; then
    echo "$name-dolt history audit FAILED: $commits commits > $max_commits; run the history-squash runbook (docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md)" >&2
    exit 1
  fi
  if [ "${oldgen_kib:-0}" -gt "$max_oldgen_kib" ]; then
    echo "$name-dolt history audit FAILED: oldgen ${oldgen_kib}KiB > ${max_oldgen_kib}KiB; run the history-squash runbook" >&2
    exit 1
  fi
  if [ "${journal_kib:-0}" -gt "$max_journal_kib" ]; then
    echo "$name-dolt history audit FAILED: journal ${journal_kib}KiB > ${max_journal_kib}KiB; auto GC is not keeping up (read the Dolt unit's auto GC lines)" >&2
    exit 1
  fi
  if [ "$newgen_kib" -gt "$max_newgen_kib" ]; then
    echo "$name-dolt history audit FAILED: newgen ${newgen_kib}KiB > ${max_newgen_kib}KiB; the working set has run far ahead of HEAD: run the Store A squash (docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md)" >&2
    exit 1
  fi
done
