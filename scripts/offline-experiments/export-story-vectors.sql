-- passage vectors of the story versions export-versions.sql lists: story_id, version_id, ordinal, embedding
COPY (
  SELECT p.story_id, p.version_id, p.ordinal, p.embedding::text
  FROM vec_story_passages p JOIN og_objects o ON o.canonical_id = p.version_id
  WHERE p.embedding IS NOT NULL AND NOT o.tombstone AND o.created_at >= '@VSTART@+00'
  ORDER BY o.created_at, p.version_id, p.ordinal
) TO STDOUT WITH (FORMAT csv);
