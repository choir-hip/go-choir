#!/usr/bin/env bash
# Keep-set snapshot for the data system cutover (C0; build sheet work order 3).
#
#   keepset-snapshot.sh new OUTDIR   # the PostgreSQL graph database
#   keepset-snapshot.sh diff OLDDIR NEWDIR
#
# The "old" mode read Store B on the corpus Dolt server; the cutover deleted
# that store (C7, 2026-10-30), so only snapshots of the new store remain to
# compare.
#
# Runs on Node B as root. Read-only: metadata, counts and id digests only;
# no document, capture or mail content is read or printed. Each snapshot
# writes OUTDIR/keepset.tsv (key<TAB>value, sorted). diff exits 0 only when
# every keep-set line matches.
#
# What it covers (docs/evidence/keep-set-accounting-2026-10-29.md):
#   - the Autopaper object graph: og_objects and og_edges counts by kind and
#     tombstone, and a SHA-256 over the sorted (kind, id, content_hash) list;
#   - published Textures: publication, version and distinct source_doc_id
#     counts (inside the og kinds above, counted again by name);
#   - captures: the distinct blob ids the web_capture metadata references,
#     and how many are missing from capture-bodies/;
#   - og blobs: distinct body_ref values and how many are missing on disk;
#   - the owner's email: per-mailbox message, attachment and draft counts
#     (maild's SQLite files, which the cutover does not touch);
#   - file trees the cutover must not touch: file count and bytes.
# Guest keep-set content (Textures in each computer, precommitment records)
# is checked by the guest migrator's per-table differential; the agent
# record lives on the tape, which the cutover does not rewrite.
set -euo pipefail

ROOT="${CHOIR_ROOT:-/var/lib/go-choir}"
ART="$ROOT/platform-artifacts"
MAIL="$ROOT/mail"
# TCP to 127.0.0.1, where the choir role authenticates by password (scram).
# The Unix socket is peer-authenticated, and this script runs as root, so a
# socket DSN fails (review of the cutover, 2026-10-30, F10).
PG_DSN="${CHOIR_PG_GRAPH_DSN:-host=127.0.0.1 port=5432 dbname=graph user=choir sslmode=disable}"
PG_PW_FILE="${KEEPSET_PG_PW_FILE:-$ROOT/secrets/pg-app-password}"
LIMIT=(nice -n 19 ionice -c3)

usage() { sed -n 4,5p "$0" >&2; exit 2; }

# q SQL -> rows on stdout, comma-separated, no header.
q_new() {
  PGPASSWORD="$( [ -r "$PG_PW_FILE" ] && cat "$PG_PW_FILE" || true)" \
    "${LIMIT[@]}" psql "$PG_DSN" -v ON_ERROR_STOP=1 -X -A -t -F, -c "$1"
}

# JSON field of the capture metadata (metadata is TEXT).
json_new() { printf "(metadata::jsonb->>'%s')" "$1"; }

# Boolean-ish column printed by either engine -> 0/1.
norm_bool() { awk -F, -v OFS=, '{ for (i=1;i<=NF;i++) if ($i=="t"||$i=="true"||$i=="TRUE") $i=1; else if ($i=="f"||$i=="false"||$i=="FALSE") $i=0; print }'; }

snapshot() {
  local mode="$1" out="$2" q json
  q="q_$mode"; json="json_$mode"
  install -d -m 0700 "$out"
  tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
  {
    $q "SELECT object_kind, tombstone, COUNT(*) FROM og_objects GROUP BY object_kind, tombstone ORDER BY object_kind, tombstone" |
      norm_bool | awk -F, '{ printf "og_objects.count\t%s/tombstone=%s\t%s\n", $1, $2, $3 }'
    $q "SELECT kind, tombstone, COUNT(*) FROM og_edges GROUP BY kind, tombstone ORDER BY kind, tombstone" |
      norm_bool | awk -F, '{ printf "og_edges.count\t%s/tombstone=%s\t%s\n", $1, $2, $3 }'

    $q "SELECT object_kind, canonical_id, content_hash FROM og_objects" | LC_ALL=C sort > "$tmp/objects"
    printf 'og_objects.ids_sha256\t-\t%s\n' "$(sha256sum < "$tmp/objects" | cut -d' ' -f1)"
    $q "SELECT kind, edge_id, from_id, to_id FROM og_edges" | LC_ALL=C sort > "$tmp/edges"
    printf 'og_edges.ids_sha256\t-\t%s\n' "$(sha256sum < "$tmp/edges" | cut -d' ' -f1)"

    printf 'textures.publications\t-\t%s\n' "$($q "SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.publication'")"
    printf 'textures.versions\t-\t%s\n' "$($q "SELECT COUNT(*) FROM og_objects WHERE object_kind = 'choir.publication_version'")"
    printf 'textures.distinct_source_docs\t-\t%s\n' "$($q "SELECT COUNT(DISTINCT $($json source_doc_id)) FROM og_objects WHERE object_kind = 'choir.publication_version'")"

    for f in content_blob_id blocks_blob_id extracted_text_blob_id; do
      $q "SELECT DISTINCT $($json "$f") FROM og_objects WHERE object_kind = 'choir.web_capture'"
    done | grep -E '^sha256:[0-9a-f]{64}$' | LC_ALL=C sort -u > "$tmp/capture_blobs"
    printf 'captures.distinct_blobs\t-\t%s\n' "$(wc -l < "$tmp/capture_blobs" | tr -d ' ')"
    printf 'captures.missing_blobs\t-\t%s\n' "$(sed 's/^sha256://' "$tmp/capture_blobs" |
      while read -r h; do [ -e "$ART/capture-bodies/${h:0:2}/$h" ] || echo "$h"; done | wc -l | tr -d ' ')"

    $q "SELECT DISTINCT body_ref FROM og_objects WHERE body_ref <> ''" | LC_ALL=C sort -u > "$tmp/body_refs"
    printf 'og_blobs.distinct_body_refs\t-\t%s\n' "$(wc -l < "$tmp/body_refs" | tr -d ' ')"
    printf 'og_blobs.missing\t-\t%s\n' "$(while read -r r; do [ -e "$ART/$r" ] || echo "$r"; done < "$tmp/body_refs" | wc -l | tr -d ' ')"

    if [ -d "$MAIL/users" ]; then
      for db in "$MAIL"/users/*/mail.db; do
        [ -e "$db" ] || continue
        owner="$(basename "$(dirname "$db")")"
        for t in email_messages email_attachments email_drafts; do
          n="$(sqlite3 -readonly "$db" "SELECT COUNT(*) FROM $t" 2>/dev/null || echo absent)"
          printf 'mail.%s\t%s\t%s\n' "$t" "${owner:0:8}" "$n"
        done
      done
    fi

    for d in capture-bodies sha256/og; do
      [ -d "$ART/$d" ] || continue
      printf 'files.%s\tcount_bytes\t%s\n' "$d" "$("${LIMIT[@]}" find "$ART/$d" -type f -printf '%s\n' | awk '{ n++; b+=$1 } END { printf "%d/%d", n, b }')"
    done
  } | LC_ALL=C sort > "$out/keepset.tsv"
  printf 'keepset: %s lines -> %s/keepset.tsv\n' "$(wc -l < "$out/keepset.tsv" | tr -d ' ')" "$out"
}

# files.* may grow between snapshots if anything still writes; the cutover
# stops every writer first, so equality is required there too.
compare() {
  local a="$1/keepset.tsv" b="$2/keepset.tsv"
  if diff -u "$a" "$b"; then
    echo "keepset: MATCH ($(wc -l < "$a" | tr -d ' ') lines)"
  else
    echo "keepset: MISMATCH" >&2
    return 1
  fi
}

case "${1:-}" in
  new) [ $# -eq 2 ] || usage; snapshot "$1" "$2" ;;
  diff) [ $# -eq 3 ] || usage; compare "$2" "$3" ;;
  *) usage ;;
esac
