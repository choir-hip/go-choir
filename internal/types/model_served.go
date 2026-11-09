package types

import (
	"sort"
	"strings"
)

// EventModelServed is the durable record of which models served a run
// (docs/problems/model-provenance-unwired-2026-11-05.md): one line per tool
// loop activation, written when the loop returns, and one per decision-model
// (System 1, citation judge) call. It names the model that answered each
// call, after any quota switch or pool failover, not the policy's
// prediction. The per-call trace line (loop.progress phase tool_loop) carries
// the same facts but is retired after 14 days; this line is small and kept.
const EventModelServed EventKind = "model.served"

// ModelServedSchema versions the payload of an EventModelServed line.
const ModelServedSchema = "choir.model_served.v1"

// ModelServedPurposeToolLoop is the purpose of a tool loop activation's
// line; a decision-model line names its judge (system1, support_judgement,
// inert_judge, subcall_judge).
const ModelServedPurposeToolLoop = "tool_loop"

// ModelServedRoleSystem1 is the role of a decision-model (System 1, Jev)
// call: the citation, inertness, subcall and System 1 judges.
const ModelServedRoleSystem1 = "system1"

// ModelServedRecord is the payload of one EventModelServed line.
type ModelServedRecord struct {
	Schema  string `json:"schema"`
	Purpose string `json:"purpose"`
	// Role is the desk role the calls served, when the writer knows it
	// better than the run record does (the decision model's purpose).
	Role string `json:"role,omitempty"`
	// SelectedProvider/SelectedModel/SelectedSource are the selection the
	// activation started on (policy, pool assignment or owner override).
	SelectedProvider string             `json:"selected_provider,omitempty"`
	SelectedModel    string             `json:"selected_model,omitempty"`
	SelectedSource   string             `json:"selected_source,omitempty"`
	Models           []ModelServedUsage `json:"models"`
	Switches         []ModelSwitch      `json:"switches,omitempty"`
}

// ModelServedUsage is one model's share of the calls a record covers.
type ModelServedUsage struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// ReportedModel is the model id the provider reported, when it differs
	// from the catalogued name the call was routed to.
	ReportedModel     string `json:"reported_model,omitempty"`
	Calls             int    `json:"calls"`
	InputTokens       int    `json:"input_tokens,omitempty"`
	OutputTokens      int    `json:"output_tokens,omitempty"`
	CachedInputTokens int    `json:"cached_input_tokens,omitempty"`
}

// ModelSwitch is one mid-run move from one model to another and why
// (model_failover, provider_quota, provider_quota_retry, ...).
type ModelSwitch struct {
	FromProvider string `json:"from_provider"`
	FromModel    string `json:"from_model"`
	ToProvider   string `json:"to_provider"`
	ToModel      string `json:"to_model"`
	Reason       string `json:"reason"`
}

// ModelServedTally accumulates one activation's served calls and switches.
// It is not safe for concurrent use; the tool loop is single-threaded.
type ModelServedTally struct {
	order    []string
	byKey    map[string]*ModelServedUsage
	switches []ModelSwitch
}

// Note counts one answered call on provider/model.
func (t *ModelServedTally) Note(provider, model, reportedModel string, input, output, cached int) {
	provider, model = strings.TrimSpace(provider), strings.TrimSpace(model)
	if model == "" {
		return
	}
	if t.byKey == nil {
		t.byKey = map[string]*ModelServedUsage{}
	}
	key := provider + "\x00" + model
	u, ok := t.byKey[key]
	if !ok {
		u = &ModelServedUsage{Provider: provider, Model: model}
		t.byKey[key] = u
		t.order = append(t.order, key)
	}
	if r := strings.TrimSpace(reportedModel); r != "" && r != model && u.ReportedModel == "" {
		u.ReportedModel = r
	}
	u.Calls++
	u.InputTokens += input
	u.OutputTokens += output
	u.CachedInputTokens += cached
}

// Switch records a move from one model to another.
func (t *ModelServedTally) Switch(s ModelSwitch) {
	t.switches = append(t.switches, s)
}

// Empty reports whether nothing was served and nothing switched.
func (t *ModelServedTally) Empty() bool {
	return t == nil || (len(t.order) == 0 && len(t.switches) == 0)
}

// Record is the tally as a payload, models in first-served order.
func (t *ModelServedTally) Record(purpose string) ModelServedRecord {
	rec := ModelServedRecord{Schema: ModelServedSchema, Purpose: purpose, Models: []ModelServedUsage{}}
	if t == nil {
		return rec
	}
	for _, key := range t.order {
		rec.Models = append(rec.Models, *t.byKey[key])
	}
	rec.Switches = append(rec.Switches, t.switches...)
	return rec
}

// ModelProvenance is the models that served a Texture document's runs, by
// desk role: derived from EventModelServed lines and the Claude delegate's
// use on engineering reports, never from the model policy.
type ModelProvenance struct {
	Schema string `json:"schema"`
	DocID  string `json:"doc_id,omitempty"`
	// Runs is how many runs of the document's lineage were read; RunsServed
	// how many of them carry a served-model record.
	Runs       int                     `json:"runs"`
	RunsServed int                     `json:"runs_served"`
	Roles      []ModelProvenanceRole   `json:"roles"`
	Switches   []ModelProvenanceSwitch `json:"switches,omitempty"`
	// Search is the search providers that served the same runs
	// (EventSearchServed), by desk role; absent when none were recorded.
	Search *SearchProvenance `json:"search,omitempty"`
}

// ModelProvenanceSchema versions ModelProvenance.
const ModelProvenanceSchema = "choir.model_provenance.v1"

// ModelProvenanceRole is one desk role's served models.
type ModelProvenanceRole struct {
	Role   string                 `json:"role"`
	Models []ModelProvenanceModel `json:"models"`
}

// ModelProvenanceModel is one model's calls for a role.
type ModelProvenanceModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// ReportedModel is the model id the provider reported answering, when
	// it differs from the catalogued name the calls were routed to.
	ReportedModel string `json:"reported_model,omitempty"`
	Calls         int    `json:"calls"`
	Runs          int    `json:"runs"`
	InputTokens   int    `json:"input_tokens,omitempty"`
	OutputTokens  int    `json:"output_tokens,omitempty"`
	// Via names how the model came to serve: "selected" (the run's
	// selection: policy, pool or owner override, Source naming which),
	// or the switch reason that moved a run onto it.
	Via    []string `json:"via,omitempty"`
	Source string   `json:"source,omitempty"`
	// Delegate marks the Claude delegate (engineering), whose calls are
	// delegate runs started, not provider calls.
	Delegate bool `json:"delegate,omitempty"`
}

// ModelProvenanceSwitch is one mid-run switch, with the run and role.
type ModelProvenanceSwitch struct {
	RunID string `json:"run_id"`
	Role  string `json:"role"`
	ModelSwitch
}

// ModelProvenanceBuilder folds served records into a ModelProvenance.
type ModelProvenanceBuilder struct {
	roles    map[string]map[string]*ModelProvenanceModel
	runsSeen map[string]map[string]bool // role/model key -> run ids
	switches []ModelProvenanceSwitch
	runs     map[string]bool
	served   map[string]bool
}

func provenanceModelKey(provider, model string, delegate bool) string {
	if delegate {
		return "delegate\x00" + model
	}
	return provider + "\x00" + model
}

// AddRun counts a run of the lineage.
func (b *ModelProvenanceBuilder) AddRun(runID string) {
	if b.runs == nil {
		b.runs = map[string]bool{}
	}
	if runID = strings.TrimSpace(runID); runID != "" {
		b.runs[runID] = true
	}
}

func (b *ModelProvenanceBuilder) model(role, provider, model string, delegate bool) *ModelProvenanceModel {
	if b.roles == nil {
		b.roles = map[string]map[string]*ModelProvenanceModel{}
		b.runsSeen = map[string]map[string]bool{}
	}
	if b.roles[role] == nil {
		b.roles[role] = map[string]*ModelProvenanceModel{}
	}
	key := provenanceModelKey(provider, model, delegate)
	m := b.roles[role][key]
	if m == nil {
		m = &ModelProvenanceModel{Provider: provider, Model: model, Delegate: delegate}
		b.roles[role][key] = m
	}
	return m
}

func (b *ModelProvenanceBuilder) countRun(role string, m *ModelProvenanceModel, runID string) {
	seenKey := role + "\x00" + provenanceModelKey(m.Provider, m.Model, m.Delegate)
	if b.runsSeen[seenKey] == nil {
		b.runsSeen[seenKey] = map[string]bool{}
	}
	if runID != "" && !b.runsSeen[seenKey][runID] {
		b.runsSeen[seenKey][runID] = true
		m.Runs++
	}
}

func addVia(m *ModelProvenanceModel, via string) {
	if via == "" {
		return
	}
	for _, v := range m.Via {
		if v == via {
			return
		}
	}
	m.Via = append(m.Via, via)
}

// AddServed folds one served record of runID under role.
func (b *ModelProvenanceBuilder) AddServed(runID, role string, rec ModelServedRecord) {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "unknown"
	}
	b.AddRun(runID)
	if b.served == nil {
		b.served = map[string]bool{}
	}
	if runID != "" && len(rec.Models) > 0 {
		b.served[runID] = true
	}
	// How each model came to serve: the selection, or the switch onto it.
	reached := map[string]string{}
	if rec.SelectedModel != "" {
		reached[provenanceModelKey(rec.SelectedProvider, rec.SelectedModel, false)] = "selected"
	}
	for _, s := range rec.Switches {
		key := provenanceModelKey(s.ToProvider, s.ToModel, false)
		if _, ok := reached[key]; !ok {
			reached[key] = s.Reason
		}
		b.switches = append(b.switches, ModelProvenanceSwitch{RunID: runID, Role: role, ModelSwitch: s})
	}
	for _, u := range rec.Models {
		m := b.model(role, u.Provider, u.Model, false)
		m.Calls += u.Calls
		m.InputTokens += u.InputTokens
		m.OutputTokens += u.OutputTokens
		if m.ReportedModel == "" && u.ReportedModel != "" && u.ReportedModel != u.Model {
			m.ReportedModel = u.ReportedModel
		}
		via, ok := reached[provenanceModelKey(u.Provider, u.Model, false)]
		if !ok {
			via = "selected"
		}
		addVia(m, via)
		if via == "selected" && m.Source == "" {
			m.Source = rec.SelectedSource
		}
		b.countRun(role, m, runID)
	}
}

// ModelServedViaUnrecorded marks a run with no model.served line (it ended
// before the line existed): its selection is shown, with no call count.
const ModelServedViaUnrecorded = "selected_unrecorded"

// AddSelectionOnly folds a run that has no served record: its selected
// model, marked unrecorded, with no calls.
func (b *ModelProvenanceBuilder) AddSelectionOnly(runID, role, provider, model, source string) {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "unknown"
	}
	b.AddRun(runID)
	m := b.model(role, provider, model, false)
	addVia(m, ModelServedViaUnrecorded)
	if m.Source == "" {
		m.Source = source
	}
	b.countRun(role, m, runID)
}

// ModelProviderClaudeDelegate is the provider name the provenance gives the
// Claude delegate (headless Claude Code in an engineering capsule).
const ModelProviderClaudeDelegate = "claude-delegate"

// AddDelegate folds one engineering attempt's Claude delegate use under the
// role of the run that started it.
func (b *ModelProvenanceBuilder) AddDelegate(runID, role, model string, calls int) {
	if calls <= 0 {
		return
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = "unknown"
	}
	b.AddRun(runID)
	m := b.model(role, ModelProviderClaudeDelegate, model, true)
	m.Calls += calls
	addVia(m, ModelProviderClaudeDelegate)
	b.countRun(role, m, runID)
}

// Build returns the provenance, roles by name and models by calls (a
// reader orders the roles as its surface needs).
func (b *ModelProvenanceBuilder) Build(docID string) ModelProvenance {
	out := ModelProvenance{Schema: ModelProvenanceSchema, DocID: docID, Runs: len(b.runs), RunsServed: len(b.served), Roles: []ModelProvenanceRole{}}
	for role, models := range b.roles {
		r := ModelProvenanceRole{Role: role}
		for _, m := range models {
			r.Models = append(r.Models, *m)
		}
		sort.Slice(r.Models, func(i, j int) bool {
			if r.Models[i].Calls != r.Models[j].Calls {
				return r.Models[i].Calls > r.Models[j].Calls
			}
			return r.Models[i].Provider+"/"+r.Models[i].Model < r.Models[j].Provider+"/"+r.Models[j].Model
		})
		out.Roles = append(out.Roles, r)
	}
	sort.Slice(out.Roles, func(i, j int) bool { return out.Roles[i].Role < out.Roles[j].Role })
	out.Switches = append(out.Switches, b.switches...)
	return out
}

// PublicModelProvenanceSchema versions PublicModelProvenance.
const PublicModelProvenanceSchema = "choir.model_provenance.public.v1"

// PublicModelProvenance is the projection of a ModelProvenance that a
// published version pins for anyone to read: desk roles, providers, models,
// call and token counts, how each model was reached, switches as
// role/from/to/reason, and the Claude delegate's use. It carries no run,
// document or overlay id and no policy source (a policy source can be a
// file path on the computer), so nothing internal leaves the computer.
type PublicModelProvenance struct {
	Schema   string                      `json:"schema"`
	Roles    []PublicModelProvenanceRole `json:"roles"`
	Switches []PublicModelSwitch         `json:"switches,omitempty"`
}

// PublicModelProvenanceRole is one desk role's served models, public.
type PublicModelProvenanceRole struct {
	Role   string              `json:"role"`
	Models []PublicModelServed `json:"models"`
}

// PublicModelServed is one model's share of a role, public.
type PublicModelServed struct {
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	ReportedModel string   `json:"reported_model,omitempty"`
	Calls         int      `json:"calls"`
	InputTokens   int      `json:"input_tokens,omitempty"`
	OutputTokens  int      `json:"output_tokens,omitempty"`
	Via           []string `json:"via,omitempty"`
	Delegate      bool     `json:"delegate,omitempty"`
}

// PublicModelSwitch is one mid-run switch, public: no run id.
type PublicModelSwitch struct {
	Role         string `json:"role"`
	FromProvider string `json:"from_provider"`
	FromModel    string `json:"from_model"`
	ToProvider   string `json:"to_provider"`
	ToModel      string `json:"to_model"`
	Reason       string `json:"reason"`
}

// Public is the projection a published version pins. It is built field by
// field, so a field added to ModelProvenance later stays private until it
// is named here.
func (p ModelProvenance) Public() PublicModelProvenance {
	out := PublicModelProvenance{Schema: PublicModelProvenanceSchema, Roles: []PublicModelProvenanceRole{}}
	for _, r := range p.Roles {
		pr := PublicModelProvenanceRole{Role: r.Role, Models: []PublicModelServed{}}
		for _, m := range r.Models {
			pr.Models = append(pr.Models, PublicModelServed{
				Provider: m.Provider, Model: m.Model, ReportedModel: m.ReportedModel, Calls: m.Calls,
				InputTokens: m.InputTokens, OutputTokens: m.OutputTokens,
				Via: append([]string(nil), m.Via...), Delegate: m.Delegate,
			})
		}
		out.Roles = append(out.Roles, pr)
	}
	for _, s := range p.Switches {
		out.Switches = append(out.Switches, PublicModelSwitch{Role: s.Role, FromProvider: s.FromProvider, FromModel: s.FromModel,
			ToProvider: s.ToProvider, ToModel: s.ToModel, Reason: s.Reason})
	}
	return out
}
