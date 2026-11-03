package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// `choir mcp` is a stdio MCP server (newline-delimited JSON-RPC 2.0 on
// stdin/stdout, diagnostics on stderr). Its tools are the command registry
// (registry.go, commands.go): every command that is not declared CLI-only
// is a tool with the same parameters, handler and output as the CLI
// command. It reuses the CLI's host, key and computer configuration; the
// key and host never come from a tool argument, since tool arguments are
// written by the model. Design: docs/reports/choir-cli-mcp-parity-2026-10-14.md.

const mcpServerVersion = "0.2.0"

// mcpProtocolVersions are the initialize-era revisions this server answers,
// newest first.
var mcpProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const mcpInstructions = `Choir: your persistent Choir computer. These tools are the choir CLI's commands; each tool's annotations say whether it reads, writes or is destructive, and its _meta lists the API key scopes it needs.
- Read before you write: trajectories_list, trajectory_get, lifecycle_events, run_list, texture_read, self_dev_head.
- Writes that take an idempotency_key replay safely with the same key; choose a new key only for a new intent.
- Precommitments: precommit_add before you assert, forecast or commit to something checkable (state p, the probability it is true, and how it settles); precommit_settle once the evidence exists (one settlement per scorer, never edited); calibration_summary before stating new probabilities (paste its first text block into your working context).`

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

type mcpServer struct {
	client *client
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
	srv := &mcpServer{client: c, stderr: stderr, out: stdout}
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
			"serverInfo":      map[string]any{"name": "choir", "title": "Choir", "version": mcpServerVersion},
			"instructions":    mcpInstructions,
		}})
	case "ping":
		s.write(rpcResponse{ID: msg.ID, Result: map[string]any{}})
	case "tools/list":
		var tools []mcpTool
		for _, c := range mcpCommands() {
			tools = append(tools, c.mcpTool())
		}
		s.write(rpcResponse{ID: msg.ID, Result: map[string]any{"tools": tools}})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			s.fail(msg.ID, -32602, "invalid params")
			return
		}
		cmd := mcpCommand(params.Name)
		if cmd == nil {
			s.fail(msg.ID, -32602, "unknown tool "+params.Name)
			return
		}
		s.write(rpcResponse{ID: msg.ID, Result: s.callTool(cmd, params.Arguments)})
	default:
		s.fail(msg.ID, -32601, "method not found: "+msg.Method)
	}
}

// callTool runs one tool. Tool failures (bad arguments, an HTTP error from
// the computer) are isError results, not protocol errors, so the model can
// read and correct them.
func (s *mcpServer) callTool(cmd *command, raw json.RawMessage) mcpToolResult {
	vals, given, err := cmd.mcpArgs(raw)
	if err != nil {
		return toolError(err)
	}
	c := *s.client
	if target := strings.TrimSpace(fmt.Sprint(vals["computer"])); vals["computer"] != nil && target != "" {
		c.computerID = target
	}
	x := &call{cmd: cmd, c: &c, vals: vals, given: given, surface: "mcp"}
	result, err := cmd.handler(x)
	if err != nil {
		return toolError(err)
	}
	return cmd.mcpResult(result, x)
}
