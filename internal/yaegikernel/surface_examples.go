package yaegikernel

import (
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Worked examples, result fields and common mistakes for the choir verbs,
// kept beside choirFunctionDocs so each verb's Help carries one literal call
// a desk can copy (progressive disclosure of the verb surface,
// docs/evidence/token-efficiency-2026-11-09/followup/progressive-disclosure.md).
// A FunctionDoc's own Example wins over an entry here; surface_cards_test.go
// pins that every callable verb ends up with one and that each parses as Go.

// The desks an example addresses come from the registry, so an example never
// names a desk the registry does not hold.
var (
	exResearcher = strconv.Quote(deskmanifest.ResearcherName())
	exAdmitter   = strconv.Quote(deskmanifest.AdmitterName())
	exCustodian  = strconv.Quote(deskmanifest.DocumentCustodianName())
)

const exReq = `map[string]any{"p_success": 0.8, "weight": 0.3, "settlement": "recipient"}`

var choirExamples = map[string]string{
	"Ask":              `a, err := choir.Ask(` + exResearcher + `, "Which outlet first reported the strike?", ` + exReq + `)`,
	"Calibration":      `c := choir.Calibration()` + "\n" + `fmt.Println(c.N, c.Brier, c.Text)`,
	"CancelAct":        `id, err := choir.CancelAct(actRef)`,
	"CancelAssignment": `id, err := choir.CancelAssignment(assignmentID, "superseded by the owner's new request")`,
	"CancelTask":       `id, err := choir.CancelTask(taskID, "the captures were already fed")`,
	"Cast":             `id, err := choir.Cast(` + exResearcher + `, "Find the court filing behind the merger report", nil, ` + exReq + `)`,
	"Complete":         `err := choir.Complete("completed", "none", "build and tests pass; diff frozen", nil, []string{receiptRef})`,
	"Context":          `c := choir.Context()` + "\n" + `fmt.Println(c)`,
	"Disagreement":     `id, err := choir.Disagreement(map[string]any{"commitment_id": id, "scorer_verdict": "no", "resolver_verdict": "yes", "evidence_refs": []string{ref}})`,
	"Done":             `choir.Done("the requested story is published and every capture is placed")`,
	"Escalate":         `id, err := choir.Escalate(` + exAdmitter + `, "the build tool is missing from the capsule", ` + exReq + `)`,
	"Freeze":           `err := choir.Freeze(buildRef, []string{testRef}, []string{toolchainRef})`,
	"InspectBundle":    `b, err := choir.InspectBundle()` + "\n" + `fmt.Println(b)`,
	"InstallPacket":    `p, err := choir.InstallPacket(releaseID)` + "\n" + `fmt.Println(p.Risk, p.Added, p.Truncated)`,
	"ListDir":          `names, err := choir.ListDir("/workspace/platform")`,
	"ListEvidence":     `l, err := choir.ListEvidence("", 10)` + "\n" + `fmt.Println(string(l))`,
	"Pack":             `p := choir.Pack()` + "\n" + `for _, it := range p.Items { fmt.Println(it) }`,
	"PollSource":       `r, err := choir.PollSource("arxiv:cs_ai", 20)` + "\n" + `fmt.Println(string(r))`,
	"ProductAPI":       `r, err := choir.ProductAPI("GET", "/api/texture/documents", nil)` + "\n" + `fmt.Println(string(r))`,
	"ReadCapture":      `c, err := choir.ReadCapture(captureID, 0, 8000)` + "\n" + `fmt.Println(string(c))`,
	"ReadEvidence":     `e, err := choir.ReadEvidence(evidenceID)` + "\n" + `fmt.Println(string(e))`,
	"ReadFile":         `src, err := choir.ReadFile("/workspace/platform/go.mod")`,
	"Records":          `for _, r := range choir.Records() { fmt.Println(r.Seq, r.Kind, r.From, r.Objective, r.Body) }`,
	"Reject":           `choir.Reject("r2", "the record asks for a rewrite the owner already declined")`,
	"Reply":            `id, err := choir.Reply(` + exAdmitter + `, recordID, "Start with the frontend; the API change follows.")`,
	"ReportPacket":     "packet := `{\"kind\":\"evidence_update\",\"claims\":[],\"sources\":[]}`\n" + `id, err := choir.ReportPacket(` + exCustodian + `, packet, "")`,
	"Research":         `res, err := choir.Research([]string{"Who filed the suit?", "What does the filing allege?"})` + "\n" + `for _, r := range res { fmt.Println(r.Objective, r.Findings, r.Error) }`,
	"Resolve":          `id, err := choir.Resolve(targetRef, map[string]any{"verdict": "confirmed", "evidence_refs": []string{ref}})`,
	"Restate":          `id, err := choir.Restate("tests_pass", map[string]any{"p": 0.8})`,
	"RunMemoryEntry":   `m, err := choir.RunMemoryEntry(entryID)`,
	"SaveEvidence":     `e, err := choir.SaveEvidence("note", "https://example.com/a", "Title", "the text to keep", nil)`,
	"Screens":          `s, err := choir.Screens(operationID)` + "\n" + `fmt.Println(s.State, len(s.Screens))`,
	"ShotStart":        `j, err := choir.ShotStart(operationID)` + "\n" + `fmt.Println(j.JobID, j.State)`,
	"ShotVerdict":      `err := choir.ShotVerdict(operationID, "pass", []map[string]any{{"Pair": "home", "BeforeSHA256": before, "AfterSHA256": after, "Answers": map[string]any{"q1": "yes"}, "Note": "matches"}}, "")`,
	"ShotWait":         `w, err := choir.ShotWait(jobID, 60)` + "\n" + `fmt.Println(w.State, len(w.Screens))`,
	"Stop":             `err := choir.Stop(id)`,
	"Verify":           `err := choir.Verify(assignmentID, "pass", []string{verifierRef}, bundleDigest, "the build and tests reproduce")`,
	"VerifyFindings":   `err := choir.VerifyFindings([]map[string]any{{"Pair": "home", "BeforeSHA256": before, "AfterSHA256": after, "Answers": map[string]any{"q1": "yes"}, "Note": "matches"}})`,
	"Wait":             `choir.Wait("waiting on the research desk's search for the filing")`,
	"WriteFile":        `n, err := choir.WriteFile("/workspace/platform/notes.txt", "text")`,
}

// choirFields adds result fields the Go type cannot show: a verb that
// returns JSON bytes.
var choirFields = map[string]string{
	"Read":             "capture_id, url, title, block_count, blocks [{id, kind, text, page}], next_block, headings",
	"ReadCapture":      "capture_id, url, title, blocks [{id, kind, text, page}], next_block",
	"SearchWireCorpus": "events [{event_id, headline, outlets, origins, corroborated, first_seen, last_seen, capture_ids}], captures [{capture_id, title, url, outlet, language, topic, fetched_at}], published [{route_path, title, snippet, kind, author, read_url}]",
}

// choirPitfalls are the mistakes the traces show for a verb, each with its
// fix. Every verb's Help also derives the result-count mistake from its
// signature (callShape).
var choirPitfalls = map[string]string{
	"Records":      "A field you guessed (r.Type, r.Task, r.To, r.Title) is a compile error: read the fields above; a capture's id and title are under r.Captures.Items, not on the record.",
	"Stories":      "The list is p.Stories, not p.Items; the next page is p.Next.",
	"Cast":         "Leaving out the last argument: req is required, and a Cast with no req is refused. Pass nil for spec when there is none.",
	"Feed":         "Leaving out req: it is required. captureIDs is a []string of ids from a captures record.",
	"Decline":      "Declining an id that is not in a captures record of this turn is refused.",
	"Read":         "The result is JSON bytes: r.Title does not compile. fmt.Println(string(r)), or json.Unmarshal(r, &c) into a struct.",
	"ReadCapture":  "The result is JSON bytes: c.Blocks does not compile. Decode it with json.Unmarshal(c, &v); pass the next_block it returns as fromBlock to read on.",
	"CellOut":      "Slicing past the end panics: out[20000:] on a shorter string. Search or slice with s[:min(len(s), n)] and print only what you need.",
	"Write":        "edits is one string of directives, each followed by its text, as in the example; it is not JSON and not a list.",
	"ReportPacket": "A packet written as JSON inside a Go map literal does not compile: pass a JSON string in a raw backquoted string, as in the example.",
	"Exec":         "args is a []string, not variadic: choir.Exec(\"go\", []string{\"test\", \"./...\"}).",
	"Searches":     "r.Results holds the hits; check r.Done before reading them as final.",
}
