package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/calibration"
)

// `choir mcp` is a stdio MCP server (newline-delimited JSON-RPC 2.0 on
// stdin/stdout, diagnostics on stderr) exposing the precommit tools:
// precommit_add, precommit_settle, precommit_list, calibration_summary.
// Design: docs/reports/choir-mcp-precommitments-2026-10-14.md §5. It reuses
// the CLI's host, key and computer configuration; the key never comes from
// a tool argument, since tool arguments are written by the model.

const mcpServerVersion = "0.1.0"

var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const mcpInstructions = `Choir precommitments: record calibrated claims before the evidence that settles them exists, settle them with evidence, and read your calibration.
- precommit_add before you assert, forecast or commit to something checkable ("tests pass", "the deploy is live", "this fix removes the error"): state p, the probability it is true, and how it settles.
- precommit_settle once the evidence exists; one settlement per scorer, never edited.
- calibration_summary before stating new probabilities: paste its first text block into your working context.`

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func strEnum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func objectSchema(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func filterProps(extra map[string]any) map[string]any {
	props := map[string]any{
		"agent":   str("Claimant agent; defaults to the server's CHOIR_AGENT"),
		"all":     map[string]any{"type": "boolean", "description": "All agents instead of the default agent"},
		"model":   str("Claimant model"),
		"type":    strEnum("Claim type", "assertion", "forecast", "commitment"),
		"project": str("Project"),
	}
	for k, v := range extra {
		props[k] = v
	}
	return props
}

var mcpTools = []mcpTool{
	{
		Name:        "precommit_add",
		Description: "Record a calibrated claim (precommitment) on your Choir computer before the evidence that settles it exists. Returns the claim with its id.",
		InputSchema: objectSchema([]string{"claim", "p"}, map[string]any{
			"claim":           str("The claim, stated so it can be settled true or false"),
			"p":               map[string]any{"type": "number", "minimum": 0, "maximum": 1, "description": "Probability that the claim is true"},
			"type":            strEnum("assertion (default), forecast or commitment", "assertion", "forecast", "commitment"),
			"criterion":       str("How it settles: the evidence that will decide it"),
			"deadline":        str("RFC 3339 time by which it should be settled"),
			"project":         str("Project"),
			"tags":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"agent":           str("Agent name; defaults to CHOIR_AGENT"),
			"model":           str("Model id; defaults to CHOIR_MODEL"),
			"session_id":      str("Session id; defaults to CHOIR_SESSION"),
			"idempotency_key": str("A retry with the same key and claim writes nothing new"),
		}),
	},
	{
		Name:        "precommit_settle",
		Description: "Append one scorer's settlement of a claim, citing evidence. A scorer settles a claim once; settlements are never edited. The scorer defaults to CHOIR_AGENT: self when that is the claimant, independent otherwise.",
		InputSchema: objectSchema([]string{"claim_id", "outcome", "evidence"}, map[string]any{
			"claim_id":     str("The claim id"),
			"outcome":      strEnum("Outcome", "true", "false", "partial", "unresolvable"),
			"evidence":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1, "description": "Evidence refs: commit, CI run, staging check, trace, problem doc"},
			"method":       strEnum("Scorer method", "self", "independent", "rule", "owner"),
			"scorer":       str("Scorer name; for rule, the rule's name"),
			"scorer_model": str("Scorer model"),
			"note":         str("Short note on the outcome"),
		}),
	},
	{
		Name:        "precommit_list",
		Description: "List claims with their settlements, governing outcome and Brier score.",
		InputSchema: objectSchema(nil, filterProps(map[string]any{
			"status": strEnum("open includes overdue", "open", "overdue", "settled", "all"),
			"limit":  map[string]any{"type": "integer", "minimum": 1},
		})),
	},
	{
		Name:        "calibration_summary",
		Description: "Your calibration: Brier score, overconfidence, calibration bins, self-score gap, largest misses. The first text block is a short track record to paste into your context before stating new probabilities.",
		InputSchema: objectSchema(nil, filterProps(map[string]any{
			"scorer":                strEnum("Whose settlements count; governing = rule > owner > independent > self", "governing", "self", "independent", "rule", "owner"),
			"group_by":              str("Comma-separated: agent, model, type, project"),
			"include_reconstructed": map[string]any{"type": "boolean", "description": "Count claims recorded after the fact"},
		})),
	},
}

// precommitTools is what the MCP server calls; precommitAPI implements it.
type precommitTools interface {
	add(calibration.ClaimRequest) (json.RawMessage, error)
	settle(string, calibration.SettlementRequest) (json.RawMessage, error)
	list(url.Values) (json.RawMessage, error)
	calibration(url.Values) (json.RawMessage, error)
}

type mcpServer struct {
	api    precommitTools
	stderr io.Writer
	out    io.Writer
}

func runMCP(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if keyFile := strings.TrimSpace(os.Getenv("CHOIR_API_KEY_FILE")); keyFile != "" && !hasFlagArg(args, "api-key-file") {
		args = append([]string{"--api-key-file", keyFile}, args...)
	}
	for i, a := range args {
		if strings.TrimLeft(a, "-") == "api-key-file" && i+1 < len(args) && strings.TrimSpace(args[i+1]) == "-" || strings.TrimLeft(a, "-") == "api-key-file=-" {
			fmt.Fprintln(stderr, "choir mcp: stdin carries the protocol; pass the key in $CHOIR_API_KEY or a mode-0600 --api-key-file")
			return 2
		}
	}
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir mcp: %v\n", err)
		return 2
	}
	srv := &mcpServer{api: precommitAPI{c}, stderr: stderr, out: stdout}
	if err := srv.serve(cliStdin); err != nil {
		fmt.Fprintf(stderr, "choir mcp: %v\n", err)
		return 1
	}
	return 0
}

func (s *mcpServer) write(resp rpcResponse) {
	resp.JSONRPC = "2.0"
	if len(resp.ID) == 0 {
		resp.ID = json.RawMessage("null")
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(s.stderr, "choir mcp: encode response: %v\n", err)
		return
	}
	_, _ = s.out.Write(append(raw, '\n'))
}

func (s *mcpServer) fail(id json.RawMessage, code int, msg string) {
	s.write(rpcResponse{ID: id, Error: &rpcError{Code: code, Message: msg}})
}

// serve reads one JSON-RPC message per line until EOF.
func (s *mcpServer) serve(in io.Reader) error {
	reader := bufio.NewReaderSize(in, 1<<16)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			s.handle(bytes.TrimSpace(line))
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (s *mcpServer) handle(line []byte) {
	if line[0] == '[' {
		s.fail(nil, -32600, "batches are not supported")
		return
	}
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		s.fail(nil, -32700, "parse error")
		return
	}
	isNotification := len(msg.ID) == 0 || string(msg.ID) == "null"
	if msg.Method == "" {
		return // a response from the client; nothing to do
	}
	if isNotification {
		return // notifications/initialized, notifications/cancelled, ...
	}
	switch msg.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		version := mcpProtocolVersions[0]
		for _, v := range mcpProtocolVersions {
			if v == params.ProtocolVersion {
				version = v
			}
		}
		s.write(rpcResponse{ID: msg.ID, Result: map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "choir", "version": mcpServerVersion},
			"instructions":    mcpInstructions,
		}})
	case "ping":
		s.write(rpcResponse{ID: msg.ID, Result: map[string]any{}})
	case "tools/list":
		s.write(rpcResponse{ID: msg.ID, Result: map[string]any{"tools": mcpTools}})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			s.fail(msg.ID, -32602, "invalid params")
			return
		}
		result, known := s.callTool(params.Name, params.Arguments)
		if !known {
			s.fail(msg.ID, -32602, "unknown tool "+params.Name)
			return
		}
		s.write(rpcResponse{ID: msg.ID, Result: result})
	default:
		s.fail(msg.ID, -32601, "method not found: "+msg.Method)
	}
}

func toolError(err error) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: err.Error()}}, IsError: true}
}

func toolJSON(raw json.RawMessage) mcpToolResult {
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") != nil {
		pretty.Reset()
		pretty.Write(raw)
	}
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: pretty.String()}}}
}

type filterArgs struct {
	Agent   string `json:"agent"`
	All     bool   `json:"all"`
	Model   string `json:"model"`
	Type    string `json:"type"`
	Project string `json:"project"`
}

func (f filterArgs) query() url.Values {
	q := url.Values{}
	if !f.All {
		agent := strings.TrimSpace(f.Agent)
		if agent == "" {
			agent = envOr(agentEnvVar, "")
		}
		q.Set("agent", agent)
	}
	q.Set("model", strings.TrimSpace(f.Model))
	q.Set("type", strings.TrimSpace(f.Type))
	q.Set("project", strings.TrimSpace(f.Project))
	return q
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func orEnv(v, env string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return envOr(env, "")
}

// callTool runs one tool; known is false for an unknown tool name. Tool
// failures (bad arguments, an HTTP error from the computer) are isError
// results, not protocol errors, so the model can read and correct them.
func (s *mcpServer) callTool(name string, raw json.RawMessage) (mcpToolResult, bool) {
	switch name {
	case "precommit_add":
		var a struct {
			Claim          string   `json:"claim"`
			P              *float64 `json:"p"`
			Type           string   `json:"type"`
			Criterion      string   `json:"criterion"`
			Deadline       string   `json:"deadline"`
			Project        string   `json:"project"`
			Tags           []string `json:"tags"`
			Agent          string   `json:"agent"`
			Model          string   `json:"model"`
			SessionID      string   `json:"session_id"`
			IdempotencyKey string   `json:"idempotency_key"`
		}
		if err := decodeArgs(raw, &a); err != nil {
			return toolError(err), true
		}
		out, err := s.api.add(calibration.ClaimRequest{
			Agent:     calibration.AgentIdentity{Name: orEnv(a.Agent, agentEnvVar), Model: orEnv(a.Model, modelEnvVar), SessionID: orEnv(a.SessionID, sessionEnvVar)},
			Claim:     a.Claim,
			Type:      a.Type,
			P:         a.P,
			Criterion: a.Criterion, Deadline: a.Deadline, Project: orEnv(a.Project, projectEnvVar), Tags: a.Tags,
			IdempotencyKey: a.IdempotencyKey,
		})
		if err != nil {
			return toolError(err), true
		}
		return toolJSON(out), true
	case "precommit_settle":
		var a struct {
			ClaimID     string   `json:"claim_id"`
			Outcome     string   `json:"outcome"`
			Evidence    []string `json:"evidence"`
			Method      string   `json:"method"`
			Scorer      string   `json:"scorer"`
			ScorerModel string   `json:"scorer_model"`
			Note        string   `json:"note"`
		}
		if err := decodeArgs(raw, &a); err != nil {
			return toolError(err), true
		}
		if strings.TrimSpace(a.ClaimID) == "" {
			return toolError(errors.New("claim_id is required")), true
		}
		out, err := s.api.settle(a.ClaimID, settlementFromInputs(a.Outcome, a.Evidence, a.Method, a.Scorer, a.ScorerModel, a.Note))
		if err != nil {
			return toolError(err), true
		}
		return toolJSON(out), true
	case "precommit_list":
		var a struct {
			filterArgs
			Status string `json:"status"`
			Limit  int    `json:"limit"`
		}
		if err := decodeArgs(raw, &a); err != nil {
			return toolError(err), true
		}
		q := a.query()
		q.Set("status", strings.TrimSpace(a.Status))
		if a.Limit > 0 {
			q.Set("limit", strconv.Itoa(a.Limit))
		}
		out, err := s.api.list(q)
		if err != nil {
			return toolError(err), true
		}
		return toolJSON(out), true
	case "calibration_summary":
		var a struct {
			filterArgs
			Scorer               string `json:"scorer"`
			GroupBy              string `json:"group_by"`
			IncludeReconstructed bool   `json:"include_reconstructed"`
		}
		if err := decodeArgs(raw, &a); err != nil {
			return toolError(err), true
		}
		q := a.query()
		q.Set("scorer", strings.TrimSpace(a.Scorer))
		q.Set("group_by", strings.TrimSpace(a.GroupBy))
		if a.IncludeReconstructed {
			q.Set("include_reconstructed", "true")
		}
		out, err := s.api.calibration(q)
		if err != nil {
			return toolError(err), true
		}
		var sum struct {
			Context string `json:"context"`
		}
		_ = json.Unmarshal(out, &sum)
		result := toolJSON(out)
		result.Content = append([]mcpContent{{Type: "text", Text: sum.Context}}, result.Content...)
		return result, true
	}
	return mcpToolResult{}, false
}
