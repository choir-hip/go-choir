package main

// CLI/MCP parity by construction (docs/reports/choir-cli-mcp-parity-2026-10-14.md).
// Ways the one-registry design could fail, written before these tests:
//   - a command is reachable on the CLI but silently missing from MCP (or the
//     reverse), with no stated reason;
//   - a CLI flag has no MCP argument (or an MCP argument has no CLI flag), so
//     one surface cannot express what the other can;
//   - the two surfaces build different HTTP requests from equivalent inputs
//     (a default applied on one side only, a value trimmed on one side, a
//     body field named differently), or print different output;
//   - a tool's schema is not valid JSON Schema, lists a required property it
//     does not define, or its defaults violate its own constraints;
//   - annotations lie: a read-only tool marked destructive, a writing tool
//     marked read-only, a hint left implicit (ChatGPT requires explicit
//     booleans; the spec's implicit destructiveHint default is true);
//   - a scope named in the registry is not a scope the auth service issues;
//   - a model-written id escapes its path segment ("../" into another route);
//   - a model-written argument reaches the host or the key;
//   - a CLI-only command is callable as a tool;
//   - an MCP argument error is a protocol error instead of an isError result,
//     or a refused call still reaches the server.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// The commands that are CLI-only. Adding one is a deliberate act: it needs a
// reason in the registry and an entry here.
var wantCLIOnly = []string{
	"api-key create", "computer bootstrap-chain", "computer rematerialize-from-tape",
	"computer restore", "help", "mcp",
	// Owner act (Library S2): publishing a package release.
	"package publish",
	"precommit import-ledger",
	// Owner acts (P7/P8): publishing and adopting a release.
	"release adopt", "release publish", "self-dev approve",
	"self-dev mode set", "self-dev wait", "texture watch", "trace export", "version",
}

func TestRegistryParityCLIAndMCP(t *testing.T) {
	seenPath, seenTool := map[string]bool{}, map[string]bool{}
	toolName := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	var cliOnly []string
	for _, cmd := range commands() {
		name := cmd.name()
		if seenPath[name] {
			t.Fatalf("duplicate command path %q", name)
		}
		seenPath[name] = true
		if got, rest, _ := resolve(cmd.path); got != cmd || len(rest) != 0 {
			t.Fatalf("%s does not resolve to itself", name)
		}
		if cmd.summary == "" {
			t.Fatalf("%s has no summary", name)
		}
		if (cmd.handler == nil) == (cmd.legacy == nil) {
			t.Fatalf("%s must have exactly one of handler or legacy runner", name)
		}
		if cmd.legacy != nil && cmd.cliOnly == "" {
			t.Fatalf("%s has a hand-written CLI runner but is not declared CLI-only with a reason", name)
		}
		if cmd.cliOnly != "" {
			cliOnly = append(cliOnly, name)
			continue
		}
		tool := cmd.toolName()
		if !toolName.MatchString(tool) || seenTool[tool] {
			t.Fatalf("%s: tool name %q is invalid or duplicate", name, tool)
		}
		seenTool[tool] = true

		// Every CLI flag is an MCP argument, a client setting that is MCP
		// server configuration, or a parameter declared CLI-only with a
		// reason; every MCP argument is a CLI flag or positional argument, or
		// declared MCP-only with a reason.
		fs, _, _ := cmd.cliFlagSet(io.Discard)
		mcpProps := cmd.inputSchema()["properties"].(map[string]any)
		byFlag := map[string]param{}
		for _, p := range cmd.params {
			if p.cliOnly != "" && p.mcpOnly != "" {
				t.Fatalf("%s: parameter %s is on neither surface", name, p.name)
			}
			if f := p.cliFlag(); f != "" && p.onCLI() {
				byFlag[f] = p
			}
		}
		fs.VisitAll(func(f *flag.Flag) {
			if reason, ok := clientConfigFlags[f.Name]; ok && reason != "" {
				return
			}
			if f.Name == "computer" && !cmd.hasParam("computer") {
				if _, ok := mcpProps["computer"]; !ok {
					t.Errorf("%s: the shared --computer flag has no MCP computer argument", name)
				}
				return
			}
			p, ok := byFlag[f.Name]
			if !ok {
				t.Errorf("%s: CLI flag --%s is not declared in the registry", name, f.Name)
				return
			}
			if _, ok := mcpProps[p.name]; !ok && p.cliOnly == "" {
				t.Errorf("%s: CLI flag --%s has no MCP argument and no CLI-only reason", name, f.Name)
			}
		})
		for prop := range mcpProps {
			if prop == "computer" && !cmd.hasParam("computer") {
				continue
			}
			var p *param
			for i := range cmd.params {
				if cmd.params[i].name == prop {
					p = &cmd.params[i]
				}
			}
			if p == nil {
				t.Errorf("%s: MCP argument %s is not a registry parameter", name, prop)
				continue
			}
			onCLI := (p.cliFlag() != "" && fs.Lookup(p.cliFlag()) != nil) || p.pos != posNone
			if !onCLI && p.mcpOnly == "" {
				t.Errorf("%s: MCP argument %s has no CLI flag or positional and no MCP-only reason", name, prop)
			}
		}
		for _, p := range cmd.params {
			if p.cliOnly != "" {
				if _, ok := mcpProps[p.name]; ok {
					t.Errorf("%s: CLI-only parameter %s leaked into the MCP schema", name, p.name)
				}
			}
		}
		for _, banned := range []string{"host", "api_key", "api_key_file", "timeout"} {
			if _, ok := mcpProps[banned]; ok {
				t.Errorf("%s: MCP schema exposes client configuration %q to the model", name, banned)
			}
		}
	}
	sort.Strings(cliOnly)
	if strings.Join(cliOnly, "|") != strings.Join(wantCLIOnly, "|") {
		t.Fatalf("CLI-only commands = %q, want %q (a new exception needs a reason and an entry here)", cliOnly, wantCLIOnly)
	}

	// The served tools/list is exactly the registry's MCP commands.
	responses := mcpSession(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var listed struct {
		Tools []mcpTool `json:"tools"`
	}
	if err := json.Unmarshal(responses[0].Result, &listed); err != nil {
		t.Fatal(err)
	}
	var served []string
	for _, tool := range listed.Tools {
		served = append(served, tool.Name)
	}
	var want []string
	for _, cmd := range mcpCommands() {
		want = append(want, cmd.toolName())
	}
	if strings.Join(served, " ") != strings.Join(want, " ") {
		t.Fatalf("tools/list = %v\nwant %v", served, want)
	}
	if len(want)+len(wantCLIOnly) != len(commands()) {
		t.Fatalf("%d tools + %d CLI-only != %d commands", len(want), len(wantCLIOnly), len(commands()))
	}
	t.Logf("%d commands: %d in both, %d CLI-only, 0 MCP-only", len(commands()), len(want), len(wantCLIOnly))

	// The top-level usage names every command.
	var usage bytes.Buffer
	printUsage(&usage)
	for _, cmd := range commands() {
		if !strings.Contains(usage.String(), "  "+cmd.name()) {
			t.Fatalf("usage omits %s", cmd.name())
		}
	}
}

// issuedScopes reads the scope set the auth service issues
// (internal/auth/handlers.go validScopes) from its source.
func issuedScopes(t *testing.T) map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "internal", "auth", "handlers.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	scopes := map[string]bool{"public": true, "authenticated": true}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "validScopes" {
			return true
		}
		for _, elt := range spec.Values[0].(*ast.CompositeLit).Elts {
			key := elt.(*ast.KeyValueExpr).Key.(*ast.BasicLit).Value
			unquoted, _ := strconv.Unquote(key)
			scopes[unquoted] = true
		}
		return false
	})
	if len(scopes) < 10 {
		t.Fatalf("read only %d scopes from %s", len(scopes), path)
	}
	return scopes
}

func TestRegistrySchemasAnnotationsAndScopes(t *testing.T) {
	issued := issuedScopes(t)
	types := map[string]bool{"string": true, "integer": true, "number": true, "boolean": true, "array": true, "object": true}
	for _, cmd := range commands() {
		name := cmd.name()
		if len(cmd.scopes) == 0 {
			t.Fatalf("%s declares no scopes", name)
		}
		for _, s := range cmd.scopes {
			if !issued[s] {
				t.Fatalf("%s: scope %q is not issued by internal/auth", name, s)
			}
		}
		if cmd.readOnly && cmd.destructive {
			t.Fatalf("%s is both read-only and destructive", name)
		}
		for _, s := range cmd.scopes {
			if cmd.readOnly && (strings.HasPrefix(s, "write:") || strings.HasSuffix(s, ":write") || strings.HasSuffix(s, ":mode") ||
				strings.HasSuffix(s, ":propose") || strings.HasSuffix(s, ":approve") || s == "manage:keys") {
				t.Fatalf("%s is read-only but needs write scope %s", name, s)
			}
		}
		for _, p := range cmd.params {
			if p.env != "" && p.kind != kString {
				t.Fatalf("%s: %s has an environment default but is not a string", name, p.name)
			}
			if p.cliOnly == "" && p.mcpOnly == "" && p.kind == kJSON {
				t.Fatalf("%s: %s is a JSON parameter on both surfaces", name, p.name)
			}
			if p.pos == posNone && p.cliFlag() == "" && p.onCLI() {
				t.Fatalf("%s: %s has neither a flag nor a position", name, p.name)
			}
			// A default must satisfy the parameter's own constraints.
			if p.def != nil {
				if reason := p.validate(p.defaultValue(), true); reason != "" && reason != "required" {
					t.Fatalf("%s: default of %s %s", name, p.name, reason)
				}
			}
		}
		if cmd.cliOnly != "" {
			continue
		}
		tool := cmd.mcpTool()
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		// Every hint is an explicit boolean, and title is set (Claude's
		// directory and ChatGPT both require them).
		var decoded struct {
			Title       string                     `json:"title"`
			Annotations map[string]json.RawMessage `json:"annotations"`
			InputSchema struct {
				Type                 string                    `json:"type"`
				Properties           map[string]map[string]any `json:"properties"`
				Required             []string                  `json:"required"`
				AdditionalProperties *bool                     `json:"additionalProperties"`
			} `json:"inputSchema"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Title == "" {
			t.Fatalf("%s has no title", tool.Name)
		}
		for _, hint := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
			v, ok := decoded.Annotations[hint]
			if !ok || (string(v) != "true" && string(v) != "false") {
				t.Fatalf("%s: %s is not an explicit boolean: %s", tool.Name, hint, raw)
			}
		}
		if cmd.readOnly != (string(decoded.Annotations["readOnlyHint"]) == "true") ||
			(!cmd.readOnly && cmd.destructive) != (string(decoded.Annotations["destructiveHint"]) == "true") {
			t.Fatalf("%s: annotations disagree with the registry: %s", tool.Name, raw)
		}
		s := decoded.InputSchema
		if s.Type != "object" || s.AdditionalProperties == nil || *s.AdditionalProperties {
			t.Fatalf("%s: input schema must be a closed object: %s", tool.Name, raw)
		}
		for _, req := range s.Required {
			if _, ok := s.Properties[req]; !ok {
				t.Fatalf("%s: required %q is not a property", tool.Name, req)
			}
		}
		for prop, ps := range s.Properties {
			typ, _ := ps["type"].(string)
			if !types[typ] {
				t.Fatalf("%s.%s: type %v", tool.Name, prop, ps["type"])
			}
			if d, _ := ps["description"].(string); d == "" {
				t.Fatalf("%s.%s: no description", tool.Name, prop)
			}
			if typ == "array" {
				items, _ := ps["items"].(map[string]any)
				if items["type"] != "string" {
					t.Fatalf("%s.%s: array items %v", tool.Name, prop, ps["items"])
				}
			}
			if enum, ok := ps["enum"]; ok {
				values, _ := enum.([]any)
				if typ != "string" || len(values) == 0 {
					t.Fatalf("%s.%s: enum %v", tool.Name, prop, enum)
				}
				if d, ok := ps["default"]; ok {
					found := false
					for _, v := range values {
						found = found || v == d
					}
					if !found {
						t.Fatalf("%s.%s: default %v not in enum", tool.Name, prop, d)
					}
				}
			}
			if pat, ok := ps["pattern"].(string); ok {
				if _, err := regexp.Compile(pat); err != nil {
					t.Fatalf("%s.%s: pattern %q: %v", tool.Name, prop, pat, err)
				}
			}
			if d, ok := ps["default"]; ok {
				switch typ {
				case "string":
					_, ok = d.(string)
				case "integer", "number":
					_, ok = d.(float64)
				case "boolean":
					_, ok = d.(bool)
				case "array":
					_, ok = d.([]any)
				}
				if !ok {
					t.Fatalf("%s.%s: default %v is not a %s", tool.Name, prop, d, typ)
				}
			}
		}
	}
}

// ---- equivalence: the same request and output from both surfaces ----

type capReq struct {
	Method, Path, Query, Computer string
	Body                          []byte
}

func parityServer(t *testing.T, reply string) *[]capReq {
	t.Helper()
	var got []capReq
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, capReq{Method: r.Method, Path: r.URL.EscapedPath(), Query: r.URL.RawQuery, Computer: r.Header.Get("X-Choir-Computer"), Body: raw})
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_parity")
	t.Setenv("CHOIR_COMPUTER", "")
	t.Setenv("CHOIR_API_KEY_FILE", "")
	t.Setenv(timeoutEnvVar, "")
	return &got
}

type rpcReply struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// mcpSession runs `choir mcp` over stdio with the given request lines and
// returns one reply per request (notifications get none).
func mcpSession(t *testing.T, lines ...string) []rpcReply {
	t.Helper()
	if os.Getenv(apiKeyEnvVar) == "" {
		t.Setenv(apiKeyEnvVar, "choir_sk_parity")
	}
	cliStdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	defer func() { cliStdin = os.Stdin }()
	var out, stderr bytes.Buffer
	if code := run([]string{"mcp"}, &out, &stderr); code != 0 {
		t.Fatalf("mcp exit %d: %s", code, stderr.String())
	}
	var replies []rpcReply
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var r rpcReply
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("non-JSON on MCP stdout: %q", line)
		}
		replies = append(replies, r)
	}
	return replies
}

func toolCall(t *testing.T, tool, args string) mcpToolResult {
	t.Helper()
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": json.RawMessage(args)}})
	replies := mcpSession(t, string(line))
	if len(replies) != 1 || replies[0].Error != nil {
		t.Fatalf("%s: replies %+v", tool, replies)
	}
	var result mcpToolResult
	if err := json.Unmarshal(replies[0].Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func canonicalJSON(raw []byte) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return string(raw)
	}
	out, _ := json.Marshal(v)
	return string(out)
}

type parityCase struct {
	cli       []string
	mcp       string
	reply     string
	stdin     string
	fails     bool          // both surfaces refuse the reply
	normalize func(*capReq) // drop fields that legitimately differ per call
}

func TestEveryToolMatchesItsCLICommand(t *testing.T) {
	durable := `{"schema":"choir.durable_work.v1","events":[]}`
	approval := `{"state":"awaiting_approval","bundle_digest":"sha256:b","verifier_refs":["verifier:1"],"desired_event_head":"d","effective_event_head":"e","generation":3}`
	checkpoint := `{"published_checkpoint":{"checkpoint":{"checkpoint_digest":"sha256:c","seq":7}}}`
	checkpointFile := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := os.WriteFile(checkpointFile, []byte(checkpoint), 0o600); err != nil {
		t.Fatal(err)
	}
	dropQuery := func(r *capReq) { r.Query = "" }
	cases := map[string]parityCase{
		"trajectories_list":  {cli: []string{"trajectories"}, mcp: `{}`, reply: `{"trajectories":[{"trajectory_id":"t1"}]}`},
		"trajectory_get":     {cli: []string{"trajectory", "traj/1"}, mcp: `{"trajectory_id":"traj/1"}`},
		"trajectory_cancel":  {cli: []string{"trajectory", "cancel", "--idempotency-key", "k", "--expected-lifecycle-version", "4", "--expected-head-revision-id", "r4", "t1"}, mcp: `{"trajectory_id":"t1","idempotency_key":"k","expected_lifecycle_version":4,"expected_head_revision_id":"r4"}`},
		"lifecycle_snapshot": {cli: []string{"lifecycle", "snapshot", "t1"}, mcp: `{"trajectory_id":"t1"}`, reply: durable},
		"lifecycle_events":   {cli: []string{"lifecycle", "events", "--after", "5", "t1"}, mcp: `{"trajectory_id":"t1","after":5}`, reply: durable},
		"lifecycle_capsule_evidence": {cli: []string{"lifecycle", "capsule-evidence", "--attempt", "2", "t1", "a1"}, mcp: `{"trajectory_id":"t1","assignment_id":"a1","attempt":2}`,
			reply: `{"schema":"` + vocabmigrate.SchemaCapsuleEvidenceV1 + `"}`},
		"identity_verify":   {cli: []string{"identity"}, mcp: `{}`, fails: true, normalize: dropQuery},
		"texture_create":    {cli: []string{"texture", "create", "--title", "T", "--content", "C", "--request-id", "r1", "--desk", "engineering"}, mcp: `{"title":"T","content":"C","request_id":"r1","desk":"engineering"}`, reply: `{"schema":"choir.texture_create.v1"}`},
		"texture_revise":    {cli: []string{"texture", "revise", "--prompt", "tighten", "--expected-head", "rev-1", "doc-1"}, mcp: `{"doc_id":"doc-1","prompt":"tighten","expected_head":"rev-1"}`},
		"texture_read":      {cli: []string{"texture", "read", "doc-1"}, mcp: `{"doc_id":"doc-1"}`},
		"texture_history":   {cli: []string{"texture", "history", "doc-1"}, mcp: `{"doc_id":"doc-1"}`},
		"texture_revisions": {cli: []string{"texture", "revisions", "doc-1"}, mcp: `{"doc_id":"doc-1"}`},
		"texture_show":      {cli: []string{"texture", "show", "doc-1"}, mcp: `{"doc_id":"doc-1"}`, reply: `{"current_revision_id":"rev-1","doc_id":"doc-1"}`},
		"texture_open_source": {cli: []string{"texture", "open-source", "--revision", "r", "--source-ref", "s", "--source-ref-version", "v", "doc-1"}, mcp: `{"doc_id":"doc-1","revision":"r","source_ref":"s","source_ref_version":"v"}`,
			reply: `{"schema":"choir.texture_source_open.v1"}`},
		"search": {cli: []string{"search", "jailer", "launch"}, mcp: `{"query":"jailer launch"}`},
		"run_start": {cli: []string{"run", "start", "--idempotency-key", "k1", "fix", "the", "deploy"}, mcp: `{"text":"fix the deploy","idempotency_key":"k1"}`,
			reply: `{"schema":"choir.durable_work.v1","command_id":"k1","start_request_digest":"d","trajectory_id":"t","doc_id":"doc","revision_id":"r","subject_id":"s","obligation_ids":["o"],"reducer_seq":1,"snapshot_cursor":1}`},
		"run_status":                       {cli: []string{"run", "status", "run-1"}, mcp: `{"run_id":"run-1"}`},
		"run_list":                         {cli: []string{"run", "list", "--limit", "7"}, mcp: `{"limit":7}`},
		"run_cancel":                       {cli: []string{"run", "cancel", "run-1"}, mcp: `{"run_id":"run-1"}`},
		"files_mkdir":                      {cli: []string{"files", "mkdir", "System/overlays"}, mcp: `{"path":"System/overlays"}`},
		"files_put":                        {cli: []string{"files", "put", "a b/x.toml"}, stdin: "[roles]\n", mcp: `{"path":"a b/x.toml","content":"[roles]\n"}`},
		"files_get":                        {cli: []string{"files", "get", "a/b.txt"}, mcp: `{"path":"a/b.txt"}`, reply: "plain bytes"},
		"computer_status":                  {cli: []string{"computer", "status", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"computer_replay_completeness":     {cli: []string{"computer", "replay-completeness", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"computer_replace_workspace":       {cli: []string{"computer", "replace-workspace", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"computer_checkpoint":              {cli: []string{"computer", "checkpoint", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"computer_import_residue_snapshot": {cli: []string{"computer", "import-residue-snapshot", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"computer_producer_reports":        {cli: []string{"computer", "producer-reports", "--computer", "c1", "--agent-id", "a", "--stale-delivered"}, mcp: `{"computer":"c1","agent_id":"a","stale_delivered":true}`},
		"computer_settle_producer_reports": {cli: []string{"computer", "settle-producer-reports", "--computer", "c1", "--command-id", "cmd", "--report-ids", "r1,r2", "r3"}, mcp: `{"computer":"c1","command_id":"cmd","report_ids":["r1","r2","r3"]}`},
		"computer_stop":                    {cli: []string{"computer", "stop", "--computer", "c1", "--idempotency-key", "k"}, mcp: `{"computer":"c1","idempotency_key":"k"}`},
		"computer_start":                   {cli: []string{"computer", "start", "--computer", "c1", "--idempotency-key", "k"}, mcp: `{"computer":"c1","idempotency_key":"k"}`},
		"computer_restart":                 {cli: []string{"computer", "restart", "--computer", "c1", "--idempotency-key", "k"}, mcp: `{"computer":"c1","idempotency_key":"k"}`},
		"computer_refresh":                 {cli: []string{"computer", "refresh", "--computer", "c1", "--idempotency-key", "k"}, mcp: `{"computer":"c1","idempotency_key":"k"}`},
		"computer_recover":                 {cli: []string{"computer", "recover", "--computer", "c1", "--idempotency-key", "k"}, mcp: `{"computer":"c1","idempotency_key":"k"}`},
		"api_key_list":                     {cli: []string{"api-key", "list"}, mcp: `{}`},
		"api_key_revoke":                   {cli: []string{"api-key", "revoke", "key-1"}, mcp: `{"key_id":"key-1"}`},
		"self_dev_list":                    {cli: []string{"self-dev", "list", "--computer", "c1", "--state", "applied,failed"}, mcp: `{"computer":"c1","state":["applied","failed"]}`},
		"self_dev_show":                    {cli: []string{"self-dev", "show", "--computer", "c1", "--operation", "op-1"}, mcp: `{"computer":"c1","operation":"op-1"}`},
		"self_dev_head":                    {cli: []string{"self-dev", "head", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"self_dev_patch":                   {cli: []string{"self-dev", "patch", "--computer", "c1", "--operation", "op-1"}, mcp: `{"computer":"c1","operation":"op-1"}`, reply: sourcePatchReply("op-1", "c1", []byte(testSourcePatch), "")},
		"self_dev_visual":                  {cli: []string{"self-dev", "visual", "--computer", "c1", "--operation", "op-1"}, mcp: `{"computer":"c1","operation":"op-1"}`, reply: visualListing("op-1", goodRow("/", testPNG))},
		"self_dev_reject":                  {cli: []string{"self-dev", "reject", "--computer", "c1", "--operation", "op-1", "--reason", "no"}, mcp: `{"computer":"c1","operation":"op-1","reason":"no"}`, reply: approval},
		"self_dev_mode_get":                {cli: []string{"self-dev", "mode", "get", "--computer", "c1"}, mcp: `{"computer":"c1"}`},
		"release_list":                     {cli: []string{"release", "list", "--limit", "5", "--publisher", "c9"}, mcp: `{"limit":5,"publisher":"c9"}`},
		"release_show":                     {cli: []string{"release", "show", "--release", testReleaseID(t)}, mcp: `{"release":"` + testReleaseID(t) + `"}`, reply: testReleaseReply(t)},
		"release_adoptions":                {cli: []string{"release", "adoptions", "--computer", "c1", "--adoption", "adoption-1"}, mcp: `{"computer":"c1","adoption":"adoption-1"}`},
		"package_search":                   {cli: []string{"package", "search", "--query", "read"}, mcp: `{"query":"read"}`},
		"package_history":                  {cli: []string{"package", "history", "--package", "reader"}, mcp: `{"package":"reader"}`},
		"package_show":                     {cli: []string{"package", "show", "--release", testPackageID(t)}, mcp: `{"release":"` + testPackageID(t) + `"}`, reply: testPackageReply(t)},
		"precommit_add":                    {cli: []string{"precommit", "add", "--claim", "tests pass", "--p", "0.8", "--tag", "a,b", "--key", "k"}, mcp: `{"claim":"tests pass","p":0.8,"tags":["a","b"],"idempotency_key":"k"}`},
		"precommit_settle": {cli: []string{"precommit", "settle", "calib:1", "--outcome", "true", "--evidence", "CI 1", "--evidence", "a, b"},
			mcp: `{"claim_id":"calib:1","outcome":"true","evidence":["CI 1","a, b"]}`},
		"precommit_list":      {cli: []string{"precommit", "list", "--status", "open"}, mcp: `{"status":"open"}`},
		"calibration_summary": {cli: []string{"precommit", "calibration", "--scorer", "independent"}, mcp: `{"scorer":"independent"}`, reply: `{"context":"Calibration track record.\n","overall":{}}`},
	}
	setIdentity(t, "claude-code/lead", "claude-opus-5-5")
	for _, cmd := range mcpCommands() {
		tool := cmd.toolName()
		tc, ok := cases[tool]
		if !ok {
			t.Errorf("tool %s has no CLI/MCP equivalence case", tool)
			continue
		}
		delete(cases, tool)
		t.Run(tool, func(t *testing.T) {
			reply := tc.reply
			if reply == "" {
				reply = `{"ok":true,"id":"x"}`
			}
			got := parityServer(t, reply)
			cliStdin = strings.NewReader(tc.stdin)
			var stdout, stderr bytes.Buffer
			code := run(tc.cli, &stdout, &stderr)
			cliStdin = os.Stdin
			cliReqs := append([]capReq(nil), *got...)
			*got = nil
			result := toolCall(t, tool, tc.mcp)
			mcpReqs := *got
			if tc.fails {
				if code != 1 || !result.IsError {
					t.Fatalf("want both to fail: cli exit %d, mcp isError %v", code, result.IsError)
				}
			} else if code != 0 || result.IsError {
				t.Fatalf("cli exit %d (%s), mcp %+v", code, stderr.String(), result)
			}
			if len(cliReqs) == 0 || len(cliReqs) != len(mcpReqs) {
				t.Fatalf("cli made %d requests, mcp %d", len(cliReqs), len(mcpReqs))
			}
			for i := range cliReqs {
				a, b := cliReqs[i], mcpReqs[i]
				if tc.normalize != nil {
					tc.normalize(&a)
					tc.normalize(&b)
				}
				if a.Method != b.Method || a.Path != b.Path || a.Query != b.Query || a.Computer != b.Computer || canonicalJSON(a.Body) != canonicalJSON(b.Body) {
					t.Fatalf("request %d differs:\ncli %s %s?%s [%s] %s\nmcp %s %s?%s [%s] %s", i,
						a.Method, a.Path, a.Query, a.Computer, a.Body, b.Method, b.Path, b.Query, b.Computer, b.Body)
				}
			}
			if tc.fails {
				return
			}
			// The same output: the CLI's JSON is the tool's structured content
			// (or, for raw bytes, its text).
			last := result.Content[len(result.Content)-1]
			if cmd.render == nil && !bytes.HasPrefix(bytes.TrimSpace(stdout.Bytes()), []byte("{")) {
				if stdout.String() != last.Text {
					t.Fatalf("raw output differs: cli %q mcp %q", stdout.String(), last.Text)
				}
				return
			}
			if canonicalJSON(stdout.Bytes()) != canonicalJSON([]byte(last.Text)) {
				t.Fatalf("output differs:\ncli %s\nmcp %s", stdout.String(), last.Text)
			}
			if len(result.StructuredContent) > 0 && canonicalJSON(result.StructuredContent) != canonicalJSON([]byte(last.Text)) {
				t.Fatalf("structuredContent differs from the text block")
			}
		})
	}
	for tool := range cases {
		t.Errorf("equivalence case %s names no MCP tool", tool)
	}
}

// ---- protocol and safety over stdio ----

func TestMCPProtocolOverStdio(t *testing.T) {
	got := parityServer(t, `{"ok":true}`)
	replies := mcpSession(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2099-01-01"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"api_key_create","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"run_list","arguments":{"limit":9000}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"run_status","arguments":{"run_id":"r","host":"https://evil.example"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"texture_read","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"run_list","arguments":{"limit":"7"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"files_get","arguments":{"path":"a/../../auth/api-keys"}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"precommit_list","arguments":{"status":"closed"}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"trajectory_get","arguments":{"trajectory_id":"../runs"}}}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"run_list","arguments":{"computer":"computer-b"}}}`,
		`{"jsonrpc":"2.0","id":12,"method":"resources/list"}`,
	)
	if len(replies) != 12 {
		t.Fatalf("%d replies, want 12 (the notification gets none)", len(replies))
	}
	if !strings.Contains(string(replies[0].Result), `"protocolVersion":"2025-11-25"`) ||
		!strings.Contains(string(replies[1].Result), `"protocolVersion":"`+mcpProtocolVersions[0]+`"`) {
		t.Fatalf("version negotiation: %s / %s", replies[0].Result, replies[1].Result)
	}
	if replies[2].Error == nil || replies[2].Error.Code != -32602 {
		t.Fatalf("a CLI-only command must not be callable: %+v", replies[2])
	}
	refused := map[int]string{3: "limit must be a number between 1 and 500", 4: "unknown argument host", 5: "doc_id is required",
		6: "limit: must be an integer", 7: "path segments . and .. are not allowed", 8: "status must be one of"}
	for i, want := range refused {
		var r mcpToolResult
		_ = json.Unmarshal(replies[i].Result, &r)
		if replies[i].Error != nil || !r.IsError || !strings.Contains(r.Content[0].Text, want) {
			t.Fatalf("reply %d = %s %+v, want isError %q", i, replies[i].Result, replies[i].Error, want)
		}
	}
	// Only the two allowed calls reached the server: the escaped id stayed
	// in its segment, and the computer argument became the target header.
	if len(*got) != 2 {
		t.Fatalf("server saw %d requests, want 2: %+v", len(*got), *got)
	}
	if (*got)[0].Path != "/api/trajectories/..%2Fruns" {
		t.Fatalf("trajectory id escaped its segment: %s", (*got)[0].Path)
	}
	if (*got)[1].Path != "/api/runs" || (*got)[1].Computer != "computer-b" || (*got)[1].Query != "limit=50" {
		t.Fatalf("computer targeting: %+v", (*got)[1])
	}
	if replies[11].Error == nil || replies[11].Error.Code != -32601 {
		t.Fatalf("resources/list: %+v", replies[11])
	}
}

func TestMCPFilesGetReturnsBinaryAsABlob(t *testing.T) {
	parityServer(t, "\x00\xff\x10binary")
	result := toolCall(t, "files_get", `{"path":"img/x.bin"}`)
	if result.IsError || len(result.Content) != 1 || result.Content[0].Type != "resource" || result.Content[0].Resource == nil {
		t.Fatalf("result = %+v", result)
	}
	raw, err := base64.StdEncoding.DecodeString(result.Content[0].Resource.Blob)
	if err != nil || string(raw) != "\x00\xff\x10binary" || result.Content[0].Resource.URI != "choir://files/img/x.bin" {
		t.Fatalf("blob = %q (%v), uri %s", raw, err, result.Content[0].Resource.URI)
	}
}

func TestCLIGroupsAndHelpComeFromTheRegistry(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "choir_sk_help")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"self-dev", "mode"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "subcommand required (get|set)") {
		t.Fatalf("group without subcommand: %d %q", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"precommit", "add", "--help"}, &stdout, &stderr); code != 2 ||
		!strings.Contains(stderr.String(), "Scopes: precommit:write") || !strings.Contains(stderr.String(), "MCP tool: precommit_add") {
		t.Fatalf("command help: %d %q", code, stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"computer", "--help"}, &stdout, io.Discard); code != 0 || !strings.Contains(stdout.String(), "computer restore") {
		t.Fatalf("group help: %d %q", code, stdout.String())
	}
	stderr.Reset()
	// Flags after a positional id are honoured (interleaved parsing).
	got := parityServer(t, `{}`)
	if code := run([]string{"run", "status", "run-1", "--computer", "c9"}, io.Discard, &stderr); code != 0 || len(*got) != 1 || (*got)[0].Computer != "c9" {
		t.Fatalf("interleaved flags: %d %q %+v", code, stderr.String(), *got)
	}
}
