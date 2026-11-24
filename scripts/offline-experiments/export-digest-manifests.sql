-- version manifests of versions created from @VSTART@: version_id, manifest body_ref (for the title and the first sentence)
COPY (
  SELECT v.canonical_id, COALESCE(m.body_ref,'')
  FROM og_objects v
  LEFT JOIN og_edges e ON e.from_id = v.canonical_id AND e.kind = 'has_manifest' AND NOT e.tombstone
  LEFT JOIN og_objects m ON m.canonical_id = e.to_id
  WHERE v.object_kind = 'choir.publication_version' AND NOT v.tombstone AND v.created_at >= '@VSTART@+00' AND v.created_at < '@END@+00'
  ORDER BY v.created_at
) TO STDOUT WITH (FORMAT csv, DELIMITER E'\t', QUOTE E'\001');
