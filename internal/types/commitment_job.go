package types

import "strings"

// CommitmentJob is the body of a capsule job's end record
// (docs/design/capsule-long-exec-2026-10-15.md): a system record from
// job:<id> to the engineering inbox that started the job. It carries what
// a desk needs to act on the job (its state, exit code, receipt and a
// bounded tail), never the whole log; LogRef names the log by content
// address.
type CommitmentJob struct {
	JobID        string   `json:"job_id"`
	State        string   `json:"state"`
	ExitCode     *int     `json:"exit_code,omitempty"`
	Argv         []string `json:"argv,omitempty"`
	Cwd          string   `json:"cwd,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	ReceiptRef   string   `json:"receipt_ref,omitempty"`
	OutputSHA256 string   `json:"output_sha256,omitempty"`
	OutputBytes  int64    `json:"output_bytes,omitempty"`
	LogRef       string   `json:"log_ref,omitempty"`
	Tail         string   `json:"tail,omitempty"`
	StartedAt    string   `json:"started_at,omitempty"`
	FinishedAt   string   `json:"finished_at,omitempty"`
}

// JobSystemAgentID is a job end record's provenance.
const JobSystemAgentID = "system:jobs"

// JobSenderInbox is the sender inbox of job id's end record.
func JobSenderInbox(jobID string) string { return "job:" + strings.TrimSpace(jobID) }

// JobRecordID is job id's end record id: one per job.
func JobRecordID(jobID string) string { return "job:" + strings.TrimSpace(jobID) + ":ended" }
