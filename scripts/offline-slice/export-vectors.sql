COPY (
  SELECT v.capture_id, v.embedding::text
  FROM vec_captures v JOIN proj_capture_index p ON p.capture_id = v.capture_id
  JOIN og_objects o ON o.canonical_id = p.capture_id
  WHERE p.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND p.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND p.fetched_at >= '2026-11-15 00:00+00' AND p.fetched_at < '2026-11-22 00:00+00'
    AND NOT o.tombstone AND COALESCE(o.superseded_by,'')=''
    AND v.embedding IS NOT NULL
  ORDER BY p.fetched_at, p.capture_id
) TO STDOUT WITH (FORMAT csv);
