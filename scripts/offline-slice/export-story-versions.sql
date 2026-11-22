-- Story versions in scope for rung 0b: every version created in the week, plus the
-- latest version each story had before the week, with a vector. Columns:
-- story_id, version_id, version_created, manifest_body_ref, publication title.
COPY (
  WITH v AS (
    SELECT DISTINCT p.story_id, p.version_id, o.created_at
    FROM vec_story_passages p JOIN og_objects o ON o.canonical_id = p.version_id
    WHERE p.embedding IS NOT NULL AND NOT o.tombstone AND o.created_at < '2026-11-22 00:00+00'
  ), pre AS (
    SELECT DISTINCT ON (story_id) * FROM v WHERE created_at < '2026-11-15 00:00+00' ORDER BY story_id, created_at DESC
  ), scope AS (
    SELECT * FROM v WHERE created_at >= '2026-11-15 00:00+00' UNION SELECT * FROM pre
  )
  SELECT s.story_id, s.version_id, to_char(s.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
         COALESCE(m.body_ref,''), COALESCE(pub.metadata::json->>'title','')
  FROM scope s
  LEFT JOIN og_edges e ON e.from_id = s.version_id AND e.kind = 'has_manifest' AND NOT e.tombstone
  LEFT JOIN og_objects m ON m.canonical_id = e.to_id
  LEFT JOIN og_objects pub ON pub.canonical_id = s.story_id
  ORDER BY s.created_at, s.version_id
) TO STDOUT WITH (FORMAT csv);
