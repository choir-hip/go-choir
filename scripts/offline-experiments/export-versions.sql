-- Published versions of Autopaper stories (the front list's rows) from @VSTART@ on:
-- story_id, version_id, revision, published_at (the front orders by this), story title.
-- The front lists publications with state published and publish_mode auto,
-- ordered by their latest version's published_at (internal/platform/publication_front.go).
COPY (
  SELECT e.from_id, e.to_id, e.metadata::json->>'revision', COALESCE(v.metadata::json->>'published_at',''),
         COALESCE(pub.metadata::json->>'title','')
  FROM og_objects v
  JOIN og_edges e ON e.to_id = v.canonical_id AND e.kind = 'has_version' AND NOT e.tombstone
  JOIN og_objects pub ON pub.canonical_id = e.from_id
  WHERE v.object_kind = 'choir.publication_version' AND NOT v.tombstone AND v.created_at >= '@VSTART@+00'
    AND pub.object_kind = 'choir.publication' AND NOT pub.tombstone
    AND pub.metadata::json->>'state' = 'published' AND pub.metadata::json->>'publish_mode' = 'auto'
  ORDER BY v.created_at, v.canonical_id
) TO STDOUT WITH (FORMAT csv);
