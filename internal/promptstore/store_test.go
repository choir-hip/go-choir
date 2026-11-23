package promptstore

import (
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"strings"
	"testing"
)

func TestPromptStoreSeedsDefaults(t *testing.T) {
	store := New(t.TempDir())

	core, err := store.LoadCore()
	if err != nil {
		t.Fatalf("load core prompt: %v", err)
	}
	if !strings.Contains(core, "multiagent writing, research, and execution system") {
		t.Fatalf("unexpected core prompt: %q", core)
	}

	prompts, err := store.List("user-alice")
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}

	if len(prompts) != len(promptRoles()) {
		t.Fatalf("prompt count = %d, want %d", len(prompts), len(promptRoles()))
	}

	for _, prompt := range prompts {
		if prompt.Role == "" {
			t.Fatal("prompt role should not be empty")
		}
		if prompt.Content == "" {
			t.Fatalf("prompt %s content should not be empty", prompt.Role)
		}
		if prompt.Source != "default" {
			t.Fatalf("prompt %s source = %q, want default", prompt.Role, prompt.Source)
		}
	}
}

func TestPromptStoreSupportsUserOverridesAndReset(t *testing.T) {
	store := New(t.TempDir())

	saved, err := store.Save("user-alice", agentprofile.Texture, "Custom texture prompt")
	if err != nil {
		t.Fatalf("save prompt override: %v", err)
	}
	if saved.Source != "user" {
		t.Fatalf("saved source = %q, want user", saved.Source)
	}

	loaded, err := store.Load("user-alice", agentprofile.Texture)
	if err != nil {
		t.Fatalf("load prompt override: %v", err)
	}
	if loaded.Content != "Custom texture prompt" {
		t.Fatalf("loaded content = %q, want custom override", loaded.Content)
	}
	if loaded.Source != "user" {
		t.Fatalf("loaded source = %q, want user", loaded.Source)
	}

	reset, err := store.Reset("user-alice", agentprofile.Texture)
	if err != nil {
		t.Fatalf("reset prompt override: %v", err)
	}
	if reset.Source != "default" {
		t.Fatalf("reset source = %q, want default", reset.Source)
	}
	if reset.Content == "" || reset.Content == "Custom texture prompt" {
		t.Fatalf("reset content should return the default prompt, got %q", reset.Content)
	}
	if !strings.Contains(reset.Content, "You are Texture") {
		t.Fatalf("reset content should load yaml-backed texture prompt, got %q", reset.Content)
	}
}

// Support judgement left management
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md): the default
// management prompt must not teach Judge or support_pairs records.
func TestManagementDefaultPromptCarriesNoSupportJudgement(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Management)
	if err != nil {
		t.Fatalf("load management prompt: %v", err)
	}
	for _, absent := range []string{"choir.Judge", "support_pairs", "Judging citations"} {
		if strings.Contains(prompt.Content, absent) {
			t.Errorf("management prompt still says %q", absent)
		}
	}
}

// Research searches the computer's own corpus before the web (symptom 3 of
// docs/archive/problems/prompt-bar-research-slow-unsynthesized-mangled-2026-10-14.md).
func TestResearchPromptSearchesTheCorpusBeforeTheWeb(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Research)
	if err != nil {
		t.Fatalf("load research prompt: %v", err)
	}
	corpus, web := strings.Index(prompt.Content, "choir.SearchWireCorpus("), strings.Index(prompt.Content, "choir.SearchAll(")
	if corpus < 0 || web < 0 || corpus > web {
		t.Fatalf("research prompt does not search the corpus before the web: %q", prompt.Content)
	}
}

// Search for what the run lacks, not for what is new (docs/problems/
// search-volume-outruns-keenable-credits-2026-11-18.md, "Why a run searches
// again"). Failure list: the prompt does not send the desk to the corpus first
// and to write each thread as held, stale or missing; still tells it to run
// searches at once or to cover every open thread in each round with several
// searches; or does not say that SearchAll sends the first wording and names
// the rest (v19, 2026-11-21: this replaced the 15-unread line and `fresh`); v20
// (2026-11-22) removed the read-first refusals and SearchFresh: it says the
// objective is to build on and use the corpus, graph first then the web, and that
// the Report names what it adds and what the corpus lacked.
func TestResearchPromptSearchesForWhatTheRunLacks(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Research)
	if err != nil {
		t.Fatalf("load research prompt: %v", err)
	}
	flat := strings.Join(strings.Fields(prompt.Content), " ")
	for _, want := range []string{
		"The first message normally ends with a corpus brief, or a sentence saying why there is none",
		"neither is there, search the corpus yourself first",
		"Search the corpus yourself with choir.SearchWireCorpus",
		"for what the brief lacks or says it left out",
		"A published story carries no date there",
		"A field marked [cut] was shortened",
		"or says the host's search did not answer",
		"is not proof that nothing is held",
		"The host tries to search this computer's corpus",
		"one corpus search in your own words before the web",
		"each thread of the objective as stored (with its capture ids), stale or missing",
		"Search the web only for the stale and missing threads, one question at a time",
		"read what it returns before you search again",
		"choir.SearchAll(question, wordings)",
		"the held mark means an earlier search of this run returned the page",
		"build on and use what the paper already holds",
		"your Report is an addition to the corpus",
		"A research run on a story opens with a digest of the graph",
		"Graph first, then the web",
		"which claims rest on which capture ids from the graph",
		"\"adds: ...\" (what this adds to the corpus) and \"missing: ...\" (what the corpus lacked; \"missing: none\" when it lacked nothing)",
		"A search of the same terms as an earlier one returns that earlier search; one search runs at a time",
		"A detail you cannot confirm is a finding to report, not a task to keep searching for",
		"a reworded query that returns the pages you already hold tells you the web lacks it",
		"say what you looked for and stop",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("research prompt lacks %q", want)
		}
	}
	for _, absent := range []string{"several searches per thread", "To run searches at once", "Cover every open thread in each round", "all the wordings of one question", "stops when wordings stop finding pages", "fewer than 15 pages", "choir.Search(query, true)", "next round of searches", "start a search with choir.SearchStart(query) and read it with", "what the host's search covered", "The host searched this computer's corpus", "ran in part"} {
		if strings.Contains(flat, absent) {
			t.Errorf("research prompt still says %q", absent)
		}
	}
	// The call stops on what the code does; the prompt does not promise that a
	// question is answered.
	if strings.Contains(flat, "is answered") {
		t.Error("research prompt overstates the stop rule")
	}
}

// The research prompt reads what the question needs and states one count: no "8
// independent sources" target and no Searches.Summary pick-list (2026-11-22: ten runs
// whose objective said "corroborate" sent 74 of 157 searches, and the prompts carried
// three source counts, 8, 2 and 1; docs/problems/research-runs-send-reworded-web-searches-2026-11-22.md).
// Failure list: the 8-source target or its pick-list is still there; the prompt does
// not say the event record's outlets list is a source list; does not say what "enough"
// is (the platform holds a story citing fewer than two outlets, and the second may come
// from the event record); or loses the pack-everything rule and the early interim packet.
func TestResearchPromptReadsWhatTheQuestionNeedsAndStatesOneCount(t *testing.T) {
	prompt, err := New(t.TempDir()).Load("user-alice", agentprofile.Research)
	if err != nil {
		t.Fatalf("load research prompt: %v", err)
	}
	flat := strings.Join(strings.Fields(prompt.Content), " ")
	for _, want := range []string{
		"Read what the question needs",
		"the graph's event record lists the outlets that reported an occurrence with the capture_ids that cite them: those capture_ids are sources too",
		"Search the web whenever the corpus does not hold what the question needs; what to stop is rewording a query against pages you already hold",
		"the held mark means an earlier search of this run returned the page, not that the paper holds it",
		"The platform holds a story that cites fewer than two outlets; the second outlet may come from the graph's event record (its outlets and capture ids), not only the web",
		"Every capture that supports a claim goes in a packet",
		"Send an interim packet when you have a claim you can cite",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("research prompt lacks %q", want)
		}
	}
	for _, absent := range []string{"at least 8 independent sources", "across distinct hosts", "Searches.Summary", "say in the Report why fewer", "as soon as the first source is captured", "mark each page new or held"} {
		if strings.Contains(flat, absent) {
			t.Errorf("research prompt still says %q", absent)
		}
	}
}

// Registering an app desk leaves the prompt store alone: only rows with a
// stored role prompt are prompt roles, so Load, List and Save for every
// built-in desk work, and the app desk has no prompt role.
func TestAppDeskRowLeavesPromptStoreWorking(t *testing.T) {
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:promptstore", Verbs: []string{"Cast"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62900, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	defer deskmanifest.RetireApp("app:promptstore")
	for _, r := range promptRoles() {
		if r == "app:promptstore" {
			t.Fatal("an app desk is a prompt role")
		}
	}
	store := New(t.TempDir())
	prompts, err := store.List("user-alice")
	if err != nil || len(prompts) != len(promptRoles()) {
		t.Fatalf("List = %d prompts, %v", len(prompts), err)
	}
	for _, r := range promptRoles() {
		if _, err := store.Load("user-alice", r); err != nil {
			t.Errorf("Load(%s): %v", r, err)
		}
		if _, err := store.Save("user-alice", r, "custom "+r); err != nil {
			t.Errorf("Save(%s): %v", r, err)
		}
	}
	if _, err := store.Load("user-alice", "app:promptstore"); err == nil {
		t.Error("Load accepted the app desk")
	}
}
