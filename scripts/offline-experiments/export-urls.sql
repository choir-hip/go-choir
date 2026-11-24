-- capture_id, url, canonical_url
COPY (
  SELECT p.capture_id, p.url, p.canonical_url
  FROM proj_capture_index p JOIN og_objects o ON o.canonical_id = p.capture_id
  WHERE p.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND p.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND p.fetched_at >= '@START@+00' AND p.fetched_at < '@END@+00'
    AND p.source_id NOT LIKE 'web:read%'
    AND NOT o.tombstone AND COALESCE(o.superseded_by,'')=''
  ORDER BY p.fetched_at, p.capture_id
) TO STDOUT WITH (FORMAT csv);
