-- All published versions of Autopaper stories since @DSTART@ (the depth of the paper's story record):
-- story_id, version_id, revision, published_at, story title (as it is now), first publication time of the story.
COPY (
  SELECT e.from_id, e.to_id, e.metadata::json->>'revision', COALESCE(v.metadata::json->>'published_at',''),
         COALESCE(pub.metadata::json->>'title',''), to_char(pub.created_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
  FROM og_objects v
  JOIN og_edges e ON e.to_id = v.canonical_id AND e.kind = 'has_version' AND NOT e.tombstone
  JOIN og_objects pub ON pub.canonical_id = e.from_id
  WHERE v.object_kind = 'choir.publication_version' AND NOT v.tombstone AND v.created_at >= '@DSTART@+00' AND v.created_at < '@END@+00'
    AND pub.object_kind = 'choir.publication' AND NOT pub.tombstone
    AND pub.metadata::json->>'state' = 'published' AND pub.metadata::json->>'publish_mode' = 'auto'
  ORDER BY v.created_at, v.canonical_id
) TO STDOUT WITH (FORMAT csv);
