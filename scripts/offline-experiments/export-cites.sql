-- A story version cites an arrival of the window: version_id, capture_id, edge created.
COPY (
  SELECT e.from_id, e.to_id, to_char(e.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
  FROM og_edges e
  JOIN proj_capture_index c ON c.capture_id = e.to_id
  WHERE e.kind = 'cites_capture' AND NOT e.tombstone AND e.created_at >= '@START@+00'
    AND c.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND c.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND c.fetched_at >= '@START@+00' AND c.fetched_at < '@END@+00'
  ORDER BY e.created_at
) TO STDOUT WITH (FORMAT csv);
