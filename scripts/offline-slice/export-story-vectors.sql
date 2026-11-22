-- passage vectors of the versions export-story-versions.sql lists: version_id, ordinal, embedding
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
  SELECT p.version_id, p.ordinal, p.embedding::text
  FROM vec_story_passages p JOIN scope s ON s.version_id = p.version_id
  WHERE p.embedding IS NOT NULL
  ORDER BY p.version_id, p.ordinal
) TO STDOUT WITH (FORMAT csv);
