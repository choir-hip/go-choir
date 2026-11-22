-- Rung 0 of the offline slice (docs/design/first-design-offline-slice-plan-2026-11-22.md).
-- Read-only queries against corpusd's `graph` database on Node B. Each \copy-style
-- COPY ... TO STDOUT is run by scripts/offline-slice/export one at a time.
-- Scope: the paper's computer, live captures (not tombstoned, not superseded),
-- fetched in the week 2026-11-15 00:00Z (inclusive) to 2026-11-22 00:00Z (exclusive).

-- captures: id, fetched_at, published_at, host, url_key, canonical_key, language, source_id, title, text_hash
COPY (
  SELECT p.capture_id, to_char(p.fetched_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
         COALESCE(to_char(p.published_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
         p.host, p.url_key, p.canonical_key, p.language, p.source_id, p.title,
         COALESCE(o.metadata::json->>'extracted_text_blob_id','')
  FROM proj_capture_index p JOIN og_objects o ON o.canonical_id = p.capture_id
  WHERE p.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND p.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND p.fetched_at >= '2026-11-15 00:00+00' AND p.fetched_at < '2026-11-22 00:00+00'
    AND NOT o.tombstone AND COALESCE(o.superseded_by,'')=''
  ORDER BY p.fetched_at, p.capture_id
) TO STDOUT WITH (FORMAT csv);
