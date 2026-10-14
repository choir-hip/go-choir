-- Proposed retrieval indexes, one statement per line so stage.sh can time
-- each. See docs/reports/object-graph-retrieval-research-2026-10-14.md §3.
ALTER TABLE og_objects ADD COLUMN og_meta_canonical_url VARCHAR(2048) GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.canonical_url'))) STORED
CREATE INDEX idx_og_objects_meta_canonical_url ON og_objects(og_meta_canonical_url(255))
CREATE INDEX idx_og_objects_kind_owner_computer_updated ON og_objects(object_kind, owner_id, computer_id, updated_at)
CREATE INDEX idx_og_objects_fx_doc_hash ON og_objects ((JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.doc_hash'))))
CREATE INDEX idx_og_edges_to_kind ON og_edges(to_id, kind)
-- (kind, created_at) alone made Dolt 2.1.9 pick it for "to_id = ? AND kind = ?" (12 s instead of 1 ms);
-- with to_id appended the planner keeps (to_id, kind) for that query. Check EXPLAIN PLAN for every hot query.
CREATE INDEX idx_og_edges_kind_created_to ON og_edges(kind, created_at, to_id)
CREATE INDEX idx_capture_index_url ON capture_index(url_hash)
CREATE INDEX idx_capture_index_computer_fetched ON capture_index(computer_id, fetched_at)
CREATE INDEX idx_capture_index_source_fetched ON capture_index(source_id, fetched_at)
CREATE FULLTEXT INDEX ft_capture_index_title ON capture_index(title_ci)
CREATE FULLTEXT INDEX ft_capture_blocks_text ON capture_blocks(text)
CREATE INDEX idx_claims_speaker_fetched ON claims(speaker_entity_id, fetched_at)
CREATE INDEX idx_claims_proposition ON claims(proposition_id)
CREATE INDEX idx_claims_event ON claims(event_id, fetched_at)
CREATE INDEX idx_claims_capture ON claims(capture_id)
CREATE INDEX idx_claims_due ON claims(settled, modality, deadline)
CREATE INDEX idx_entity_aliases_entity ON entity_aliases(entity_id)
CREATE INDEX idx_event_participants_event ON event_participants(event_id)
CREATE INDEX idx_world_events_start ON world_events(start_at)
