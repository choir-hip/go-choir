package main

// The assembled run of `choir trace run <run_id>`: one JSON document whose
// field names are the contract (`--json` prints it; the text view is a
// rendering of it). Every field is read from the run's own stored record;
// nothing here is inferred by a model, and a thing the record does not hold
// is said to be absent rather than left out.

const traceRunSchema = "choir.trace_run.v1"

// traceRunDoc is one assembled run.
type traceRunDoc struct {
	Schema string         `json:"schema"`
	Run    traceRunHeader `json:"run"`
	Totals traceRunTotals `json:"totals"`
	// Steps are the run's steps in time order. A model call holds the cells
	// it issued; a cell holds the module-surface calls made inside it.
	Steps []traceRunStep `json:"steps"`
	// Retention names, per event kind, what the working store has already
	// retired for a run this old, so a quiet run is not read as a run that
	// did nothing.
	Retention []traceRunRetention `json:"retention"`
	// Reads says what was read from which route and how it answered.
	Reads []traceRunRead `json:"reads"`
	// Partial is true when any read was refused, cut or failed; PartialWhy
	// says what is missing.
	Partial    bool     `json:"partial"`
	PartialWhy []string `json:"partial_why,omitempty"`
	// Gaps is what this run's record does not contain, from what was
	// actually absent in this run's data.
	Gaps []traceRunGap `json:"gaps"`
}

// traceRunText is a stored text with its size; Cut says the head shown is
// less than the stored text (Chars is the stored size in characters).
type traceRunText struct {
	Text  string `json:"text"`
	Chars int    `json:"chars"`
	Cut   bool   `json:"cut,omitempty"`
	// StoredCut says the record itself holds a cut text (the writer cut it
	// before storing it), as opposed to this view clipping it.
	StoredCut bool `json:"stored_cut,omitempty"`
}

type traceRunModelUse struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model"`
	Calls    int    `json:"calls"`
}

type traceRunWoke struct {
	// Recorded is true when the run row names the record or act that woke
	// it; false says "not recorded".
	Recorded bool `json:"recorded"`
	// RequestSource is the run row's label (inbox_record, channel_message,
	// update_coagent, lifecycle_texture_control, ...); a label, not a record.
	RequestSource string            `json:"request_source,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
	// Candidate is an inference, labelled as one: the newest record received
	// in the run's inbox before it started.
	Candidate string `json:"candidate,omitempty"`
	Text      string `json:"text"`
}

type traceRunHeader struct {
	RunID        string `json:"run_id"`
	AgentID      string `json:"agent_id,omitempty"`
	AgentProfile string `json:"agent_profile,omitempty"`
	AgentRole    string `json:"agent_role,omitempty"`
	Desk         string `json:"desk,omitempty"`
	ChannelID    string `json:"channel_id,omitempty"`
	TrajectoryID string `json:"trajectory_id,omitempty"`
	ComputerID   string `json:"computer_id,omitempty"`
	State        string `json:"state"`
	CreatedAt    string `json:"started_at"`
	FinishedAt   string `json:"ended_at,omitempty"`
	DurationMs   *int64 `json:"duration_ms,omitempty"`
	// Live is true when the run has not reached a terminal state: the
	// newest events lag the telemetry window (about 1 s, up to 2 s), and no
	// end is invented.
	Live        bool   `json:"live,omitempty"`
	LiveAsOfSeq int64  `json:"live_as_of_seq,omitempty"`
	ParentRunID string `json:"parent_run_id,omitempty"`
	// ChildRuns are runs whose requested_by_run_id names this run, found in
	// the newest runs of the same channel; ChildRunsNote says how they were found.
	ChildRuns     []string           `json:"child_runs,omitempty"`
	ChildRunsNote string             `json:"child_runs_note,omitempty"`
	Models        []traceRunModelUse `json:"models,omitempty"`
	// Served is the model.served summary rows (kept, not retired): per
	// purpose and role, the calls and tokens each model served. A research
	// child's calls are recorded here on the parent, under the child's role.
	Served  []traceRunServed `json:"served,omitempty"`
	WokenBy traceRunWoke     `json:"woken_by"`
	Prompt  traceRunText     `json:"prompt"`
	Result  traceRunText     `json:"result"`
	Error   string           `json:"error,omitempty"`
}

// traceRunServed is one model's share of a model.served row.
type traceRunServed struct {
	Purpose      string `json:"purpose,omitempty"`
	Role         string `json:"role,omitempty"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	Calls        int    `json:"calls"`
	InputTokens  int64  `json:"input_tokens,omitempty"`
	OutputTokens int64  `json:"output_tokens,omitempty"`
}

type traceRunTotals struct {
	ModelCalls int `json:"model_calls"`
	// ModelCallsStatus says where ModelCalls came from: rows (the tool_loop
	// rows), served_only (the model.served summary; per-call rows absent),
	// aged_out (the rows are past retention) or not_recorded (no row of
	// either kind). For the last two ModelCalls and the tokens are unknown,
	// not zero.
	ModelCallsStatus string `json:"model_calls_status"`
	// ChildModelCalls counts calls model.served records under a child's role
	// (research children); they are in no tool_loop row of this run.
	ChildModelCalls int   `json:"child_model_calls,omitempty"`
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	CachedTokens    int64 `json:"cached_tokens"`
	Cells           int   `json:"cells"`
	// ModuleCalls counts module-surface calls by verb, from the rows the
	// record holds; ModuleCallsOmitted counts rows the writer dropped past
	// its bound, by verb (the record says how many, not what they were).
	ModuleCalls        map[string]int `json:"module_calls"`
	ModuleCallsOmitted map[string]int `json:"module_calls_omitted,omitempty"`
	Searches           int            `json:"searches"`
	SearchesEnded      int            `json:"searches_ended"`
	PagesReturned      int            `json:"pages_returned"`
	// PagesRead is the largest "pages read" any search's decision counted
	// when it was asked: a floor, as of the last search with counts.
	PagesRead int `json:"pages_read_at_last_search"`
	Errors    int `json:"errors"`
}

// traceRunReason is the reason the record actually holds for a step, in
// this order: the turn commit's Reason, the model's words immediately
// before the step, or "not recorded".
type traceRunReason struct {
	Source string `json:"source"` // turn_commit | model_words | cell | not_recorded
	Text   string `json:"text,omitempty"`
	Chars  int    `json:"chars,omitempty"`
	Cut    bool   `json:"cut,omitempty"`
	// StoredCut says the stored model text was cut by the writer (2000 chars).
	StoredCut bool   `json:"stored_cut,omitempty"`
	Note      string `json:"note,omitempty"`
}

type traceRunModel struct {
	Provider     string       `json:"provider,omitempty"`
	Model        string       `json:"model,omitempty"`
	Iteration    int          `json:"iteration,omitempty"`
	InputTokens  int64        `json:"input_tokens"`
	OutputTokens int64        `json:"output_tokens"`
	CachedTokens int64        `json:"cached_tokens"`
	LatencyMs    int64        `json:"latency_ms"`
	StopReason   string       `json:"stop_reason,omitempty"`
	ToolCalls    int          `json:"tool_calls"`
	ToolNames    []string     `json:"tool_names,omitempty"`
	Words        traceRunText `json:"words"`
}

type traceRunCell struct {
	Tool string `json:"tool"`
	// Source is the cell's source as stored on its tool.result; Output the
	// stored output. Either is empty with its *Missing set when that record
	// is gone (retired) or never paired.
	Source        traceRunText `json:"source"`
	SourceMissing string       `json:"source_missing,omitempty"`
	Output        traceRunText `json:"output"`
	OutputMissing string       `json:"output_missing,omitempty"`
	OutputBytes   int64        `json:"output_bytes"`
	IsError       bool         `json:"is_error"`
	Error         string       `json:"error,omitempty"`
	Intents       int          `json:"intents,omitempty"`
	KilledBy      string       `json:"killed_by,omitempty"`
	WorkerExit    string       `json:"worker_exit,omitempty"`
	CallID        string       `json:"call_id,omitempty"`
	CellRecorded  bool         `json:"cell_record"`
}

type traceRunCall struct {
	Verb    string `json:"verb"`
	IsError bool   `json:"is_error"`
	Error   string `json:"error,omitempty"`
	// Query is always "not recorded": the egress row never holds it.
	Query string `json:"query,omitempty"`
	// Decision is what the run held and had read when it asked, for a
	// search start; nil says the row carries no counts.
	Decision *traceRunDecision `json:"decision,omitempty"`
	// End is the search's end as recorded (search_done), paired to the start
	// by order and time because the start row carries no search id.
	End *traceRunSearchEnd `json:"end,omitempty"`
	// EndNote says why End is absent ("no end recorded").
	EndNote string `json:"end_note,omitempty"`
}

type traceRunDecision struct {
	Verb            string `json:"search_verb,omitempty"`
	EarlierSearches int    `json:"earlier_searches"`
	EarlierPagesNew int    `json:"earlier_pages_new"`
	PagesTried      int    `json:"pages_tried"`
	PagesRead       int    `json:"pages_read"`
	OutletsRead     int    `json:"outlets_read"`
}

type traceRunSearchEnd struct {
	SearchID      string `json:"search_id,omitempty"`
	OffsetMs      int64  `json:"offset_ms"`
	DurationMs    int64  `json:"duration_ms"`
	Answered      int    `json:"answered"`
	Arrived       int    `json:"arrived"`
	Total         int    `json:"total"`
	PagesReturned int    `json:"pages_returned"`
	Error         string `json:"error,omitempty"`
	PairedBy      string `json:"paired_by"`
}

type traceRunRecord struct {
	Direction string `json:"direction"` // sent | received
	RecordID  string `json:"record_id"`
	Kind      string `json:"kind"`
	Subtype   string `json:"subtype,omitempty"`
	Inbox     string `json:"inbox,omitempty"`
	From      string `json:"from,omitempty"`
	Round     int    `json:"round,omitempty"`
	Final     bool   `json:"final,omitempty"`
	// Forecast is what the record stores as a forecast: the Cast's
	// hypotheses with their probabilities, and p_success.
	Forecast []string `json:"forecast,omitempty"`
	PSuccess *float64 `json:"p_success,omitempty"`
	// Report is the child's Report summary when the record is one.
	Report  string `json:"report,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// traceRunStep is one step. Kind is one of: lifecycle, model_call, cell,
// module_call, tool_call, record, turn_commit, control, work_item,
// judgment, compaction, retry, served, event.
type traceRunStep struct {
	Kind       string `json:"kind"`
	At         string `json:"at"`
	OffsetMs   int64  `json:"offset_ms"`
	DurationMs *int64 `json:"duration_ms,omitempty"`
	Seq        int64  `json:"seq,omitempty"`
	EventID    string `json:"event_id,omitempty"`
	Title      string `json:"title"`
	// Note carries a plain-language fact the title cannot (an interrupted
	// run, a missing pairing).
	Note   string            `json:"note,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
	Reason *traceRunReason   `json:"reason,omitempty"`
	Model  *traceRunModel    `json:"model,omitempty"`
	Cell   *traceRunCell     `json:"cell,omitempty"`
	Call   *traceRunCall     `json:"call,omitempty"`
	Record *traceRunRecord   `json:"record,omitempty"`
	Steps  []traceRunStep    `json:"steps,omitempty"`
}

type traceRunRetention struct {
	Kind   string `json:"kind"`
	Days   int    `json:"days"`
	Status string `json:"status"` // gone | partial
	Text   string `json:"text"`
	Rows   int    `json:"rows_present"`
}

type traceRunRead struct {
	Route  string `json:"route"`
	Status string `json:"status"` // ok | partial | failed | skipped
	Detail string `json:"detail,omitempty"`
	Items  int    `json:"items,omitempty"`
}

type traceRunGap struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
