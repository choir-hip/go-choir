-- Offline experiments E1 to E3 (docs/evidence/offline-experiments-2026-11-24/README.md).
-- Read-only queries against corpusd's `graph` database on Node B, run one at a time
-- by scripts/offline-experiments/export under nice, a statement timeout and
-- default_transaction_read_only. The paper computer's public corpus only.
-- Placeholders @START@ and @END@ (UTC, the arrivals window) and @VSTART@ (the
-- first story version time that is read) are replaced by the export script.
--
-- ==== captures: source arrivals only (research's own web reads are not arrivals)
-- id, fetched_at, published_at, host, url_key, canonical_key, language, source_id, title, text_hash
COPY (
  SELECT p.capture_id, to_char(p.fetched_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
         COALESCE(to_char(p.published_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
         p.host, p.url_key, p.canonical_key, p.language, p.source_id, p.title,
         COALESCE(o.metadata::json->>'extracted_text_blob_id','')
  FROM proj_capture_index p JOIN og_objects o ON o.canonical_id = p.capture_id
  WHERE p.owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND p.computer_id='computer-1355b83bced1613eb048ee28d66688a8'
    AND p.fetched_at >= '@START@+00' AND p.fetched_at < '@END@+00'
    AND p.source_id NOT LIKE 'web:read%'
    AND NOT o.tombstone AND COALESCE(o.superseded_by,'')=''
  ORDER BY p.fetched_at, p.capture_id
) TO STDOUT WITH (FORMAT csv);
