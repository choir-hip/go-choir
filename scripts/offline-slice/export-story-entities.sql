-- the stories' entity links: story_id, entity canonical id, role, edge created
COPY (
  SELECT e.from_id, e.to_id, COALESCE(e.metadata::json->>'role',''), to_char(e.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
  FROM og_edges e
  WHERE e.kind = 'mentions_entity' AND NOT e.tombstone AND e.from_id LIKE 'obj:choir.publication:%'
    AND e.created_at < '2026-11-22 00:00+00'
  ORDER BY e.created_at
) TO STDOUT WITH (FORMAT csv);
