-- entity rows: canonical_id, created_at, updated_at, tombstone, metadata (name, type, aliases)
COPY (
  SELECT canonical_id, to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
         to_char(updated_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), tombstone, metadata
  FROM og_objects WHERE object_kind='choir.entity' AND owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1'
  ORDER BY created_at, canonical_id
) TO STDOUT WITH (FORMAT csv);
