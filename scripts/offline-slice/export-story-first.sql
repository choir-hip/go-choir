-- each story's first version time (all stories with a vector): story_id, first_version_created
COPY (
  SELECT p.story_id, to_char(min(o.created_at) AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
  FROM (SELECT DISTINCT story_id, version_id FROM vec_story_passages WHERE embedding IS NOT NULL) p
  JOIN og_objects o ON o.canonical_id = p.version_id
  GROUP BY p.story_id
) TO STDOUT WITH (FORMAT csv);
