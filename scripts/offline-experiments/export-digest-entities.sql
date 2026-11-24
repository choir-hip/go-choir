-- entity objects: canonical_id, created_at, metadata (name)
COPY (
  SELECT canonical_id, to_char(created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'), metadata
  FROM og_objects WHERE object_kind='choir.entity' AND owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND NOT tombstone
) TO STDOUT WITH (FORMAT csv);
