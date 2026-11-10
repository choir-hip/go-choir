// Command choir-desk-mcp is the in-capsule MCP server of the coding-agent desk
// (docs/design/coding-agent-desk-over-mcp-2026-11-09.md section 2).
//
//	choir-desk-mcp serve --socket PATH   the desk tools, forwarded to the host over PATH
//	choir-desk-mcp probe-echo            one tool that echoes its arguments; step 0 probe (ii) and (iii)
//
// It speaks MCP over stdio, holds no state, no credential and no model, and
// reads no environment variable. Stdout is JSON-RPC only; diagnostics go to
// stderr. Exit codes: 0 ok, 1 error, 2 usage.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yusefmosiah/go-choir/internal/deskmcp"
)

const usage = "usage: choir-desk-mcp serve --socket PATH | probe-echo"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, usage)
		return 2
	}
	switch args[0] {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		fs.SetOutput(errw)
		socket := fs.String("socket", "", "the host's per-run socket")
		if err := fs.Parse(args[1:]); err != nil || *socket == "" {
			fmt.Fprintln(errw, usage)
			return 2
		}
		s := &deskmcp.Server{Tools: deskmcp.Exposed(), Fwd: deskmcp.SocketForwarder{Path: *socket}, Log: errw}
		if len(s.Tools) == 0 {
			fmt.Fprintln(errw, "choir-desk-mcp: no desk performs assignments; nothing to serve")
			return 1
		}
		return serve(s, in, out, errw)
	case "probe-echo":
		s := &deskmcp.Server{Tools: deskmcp.ProbeTools(), Fwd: echoForwarder{}, Log: errw}
		return serve(s, in, out, errw)
	}
	fmt.Fprintln(errw, usage)
	return 2
}

func serve(s *deskmcp.Server, in io.Reader, out, errw io.Writer) int {
	if err := s.Serve(context.Background(), in, out); err != nil {
		fmt.Fprintln(errw, "choir-desk-mcp:", err)
		return 1
	}
	return 0
}

type echoForwarder struct{}

func (echoForwarder) Forward(_ context.Context, tool string, args json.RawMessage) (deskmcp.Result, error) {
	return deskmcp.Result{Text: "echo " + tool + " " + string(args)}, nil
}
