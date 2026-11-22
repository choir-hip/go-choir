-- the oracle: a story version cites a capture of the week. version_id, story_id, capture_id, edge created
COPY (
  SELECT e.from_id, p.story_id, e.to_id, to_char(e.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
  FROM og_edges e
  JOIN proj_capture_index c ON c.capture_id = e.to_id
  JOIN (SELECT DISTINCT story_id, version_id FROM vec_story_passages) p ON p.version_id = e.from_id
  WHERE e.kind = 'cites_capture' AND NOT e.tombstone
    AND c.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND c.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND c.fetched_at >= '2026-11-15 00:00+00' AND c.fetched_at < '2026-11-22 00:00+00'
  ORDER BY e.created_at
) TO STDOUT WITH (FORMAT csv);
