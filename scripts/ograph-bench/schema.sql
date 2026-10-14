-- Benchmark schema. og_objects/og_edges copy Store B's DDL
-- (internal/platform/store.go corpusSchemaDDL) WITHOUT its secondary
-- indexes; load.sh builds those after import so their build time is measured.
-- The other tables are the proposed typed retrieval tables
-- (docs/reports/object-graph-retrieval-research-2026-10-14.md §3).

CREATE TABLE og_objects (
	canonical_id  VARCHAR(255) NOT NULL PRIMARY KEY,
	object_kind   VARCHAR(128) NOT NULL,
	owner_id      VARCHAR(255) NOT NULL,
	computer_id   VARCHAR(255) NOT NULL DEFAULT '',
	version_id    VARCHAR(255) NOT NULL DEFAULT '',
	content_hash  VARCHAR(128) NOT NULL,
	body          LONGBLOB,
	body_ref      VARCHAR(512) NOT NULL DEFAULT '',
	body_size     BIGINT NOT NULL DEFAULT 0,
	metadata      LONGTEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     BOOLEAN NOT NULL DEFAULT FALSE,
	superseded_by VARCHAR(255) NULL -- nullable only because CSV import maps '' to NULL
);

CREATE TABLE og_edges (
	edge_id    VARCHAR(255) NOT NULL PRIMARY KEY,
	from_id    VARCHAR(255) NOT NULL,
	to_id      VARCHAR(255) NOT NULL,
	kind       VARCHAR(128) NOT NULL,
	metadata   LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL,
	tombstone  BOOLEAN NOT NULL DEFAULT FALSE
);

-- Typed projection of choir.web_capture metadata (one row per capture).
CREATE TABLE capture_index (
	canonical_id  VARCHAR(255) NOT NULL PRIMARY KEY,
	owner_id      VARCHAR(255) NOT NULL,
	computer_id   VARCHAR(255) NOT NULL,
	url_hash      CHAR(32) NOT NULL,
	canonical_url VARCHAR(2048) NOT NULL,
	host          VARCHAR(255) NOT NULL,
	source_id     VARCHAR(255) NOT NULL,
	lang          VARCHAR(16) NOT NULL,
	fetched_at    DATETIME NOT NULL,
	published_at  DATETIME NULL,
	title         VARCHAR(1024) NOT NULL,
	title_ci      VARCHAR(1024) COLLATE utf8mb4_0900_ai_ci NOT NULL
);

-- Block text of captures (today it lives only in the CAS blob).
CREATE TABLE capture_blocks (
	capture_id VARCHAR(255) NOT NULL,
	block_ord  INT NOT NULL,
	lang       VARCHAR(16) NOT NULL,
	text       TEXT COLLATE utf8mb4_0900_ai_ci NOT NULL,
	PRIMARY KEY (capture_id, block_ord)
);

CREATE TABLE claims (
	claim_id          VARCHAR(128) NOT NULL PRIMARY KEY,
	capture_id        VARCHAR(255) NOT NULL,
	speaker_entity_id VARCHAR(64) NOT NULL,
	proposition_id    VARCHAR(64) NOT NULL,
	event_id          VARCHAR(64) NOT NULL,
	modality          VARCHAR(16) NOT NULL,
	stance            VARCHAR(16) NOT NULL,
	selection         VARCHAR(16) NOT NULL,
	uttered_at        DATETIME NOT NULL,
	fetched_at        DATETIME NOT NULL,
	deadline          DATETIME NULL,
	settled           TINYINT NOT NULL DEFAULT 0,
	lang              VARCHAR(16) NOT NULL,
	gloss             VARCHAR(600) COLLATE utf8mb4_0900_ai_ci NOT NULL
);

CREATE TABLE entities (
	entity_id   VARCHAR(64) NOT NULL PRIMARY KEY,
	qid         VARCHAR(32) NULL,
	entity_type VARCHAR(32) NOT NULL,
	label       VARCHAR(512) NOT NULL
);

CREATE TABLE entity_aliases (
	alias_norm VARCHAR(255) NOT NULL,
	entity_id  VARCHAR(64) NOT NULL,
	lang       VARCHAR(16) NOT NULL,
	alias      VARCHAR(255) NOT NULL,
	PRIMARY KEY (alias_norm, entity_id, lang)
);

CREATE TABLE alias_trigrams (
	gram       VARCHAR(8) NOT NULL,
	alias_norm VARCHAR(255) NOT NULL,
	entity_id  VARCHAR(64) NOT NULL,
	PRIMARY KEY (gram, alias_norm, entity_id)
);

CREATE TABLE world_events (
	event_id        VARCHAR(64) NOT NULL PRIMARY KEY,
	start_at        DATETIME NOT NULL,
	end_at          DATETIME NOT NULL,
	label           VARCHAR(512) NOT NULL,
	scale           VARCHAR(16) NOT NULL,
	parent_event_id VARCHAR(64) NULL
);

CREATE TABLE event_participants (
	entity_id VARCHAR(64) NOT NULL,
	start_at  DATETIME NOT NULL,
	event_id  VARCHAR(64) NOT NULL,
	role      VARCHAR(16) NOT NULL,
	PRIMARY KEY (entity_id, start_at, event_id)
);
