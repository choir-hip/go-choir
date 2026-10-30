package platform

// Operational projection tables live in Store A, never in the semantic tape.
//
// The data system cutover (2026-10-30) deleted the projection bases and the
// host replay workers. This DDL is unchanged until a later change drops the
// tables nothing reads any more:
//   - computer_projection_base_refusals: no writer, no reader;
//   - computer_projection_base_pins: no writer; read only by the artifact GC
//     live set (projectionBasePins);
//   - computer_projection_jobs: the enqueue route is gone; only the
//     watermark advertisement's supersede and the artifact GC live set
//     (projectionBasePins) still touch the rows an earlier build wrote;
//   - projection_worker_status: written by checkpointd's heartbeat; no reader.
//
// computer_projection_bases and computer_projection_epoch_roots are still
// written by the watermark advertisement (file_cas.go);
// computer_checkpoint_verdicts is live.
const projectionJobSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_projection_jobs (
 computer_id VARCHAR(128) PRIMARY KEY,
 generation BIGINT NOT NULL,
 status VARCHAR(32) NOT NULL,
 reason VARCHAR(255) NOT NULL,
 target_sequence BIGINT NOT NULL,
 target_head CHAR(64) NOT NULL,
 watermark_sequence BIGINT NOT NULL,
 seed_base_ref VARCHAR(255) NOT NULL,
 key_digest VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 error_text TEXT NOT NULL,
 genesis_repair BOOLEAN NOT NULL,
 failures INT NOT NULL,
 lease_token VARCHAR(64) NOT NULL,
 lease_until TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 build_commit VARCHAR(128) NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS computer_projection_base_refusals (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 refusals INT NOT NULL,
 last_error TEXT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 build_commit VARCHAR(128) NOT NULL DEFAULT '',
 PRIMARY KEY(computer_id,base_ref)
);
CREATE TABLE IF NOT EXISTS computer_projection_bases (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 sequence BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(computer_id,base_ref)
);
CREATE TABLE IF NOT EXISTS computer_projection_epoch_roots (
 computer_id VARCHAR(128) PRIMARY KEY,
 base_ref VARCHAR(255) NOT NULL,
 sequence BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS computer_projection_base_pins (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 reference_id VARCHAR(255) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(computer_id,base_ref,reference_id)
);
CREATE TABLE IF NOT EXISTS computer_checkpoint_verdicts (
 computer_id VARCHAR(128) NOT NULL,
 checkpoint_digest CHAR(64) NOT NULL,
 accepted_sequence BIGINT NOT NULL,
 accepted_head CHAR(64) NOT NULL,
 status VARCHAR(32) NOT NULL,
 reason VARCHAR(64) NOT NULL,
 detail TEXT NOT NULL,
 differing_tables TEXT NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(computer_id,checkpoint_digest)
);
CREATE TABLE IF NOT EXISTS projection_worker_status (
 worker_id VARCHAR(64) PRIMARY KEY,
 heartbeat TIMESTAMPTZ NOT NULL,
 build_commit VARCHAR(128) NOT NULL,
 error_text TEXT NOT NULL
);
`
