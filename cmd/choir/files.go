// Owner files surface (P5-roster overlay install path).
//
// `choir files` speaks the autoputer files API (/api/files/{path}) with the
// same owner-scoped Bearer auth as every other verb: mkdir (POST), put (PUT
// raw bytes, creating or replacing), get (GET raw bytes). Precedent:
// compaction-eval used PUT into the guest store.

package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func runFiles(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir files: subcommand required (mkdir|put|get)")
		return 2
	}
	sub := args[0]
	switch sub {
	case "mkdir":
		return runFilesMkdir(args[1:], stdout, stderr)
	case "put":
		return runFilesPut(args[1:], stdout, stderr)
	case "get":
		return runFilesGet(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir files: unknown subcommand %q\n", sub)
		return 2
	}
}

func runFilesMkdir(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir files mkdir", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir files mkdir: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir files mkdir: remote path required")
		return 2
	}
	var resp map[string]any
	if err := c.do(http.MethodPost, "/api/files/"+strings.TrimPrefix(strings.TrimSpace(rest[0]), "/"), nil, &resp); err != nil {
		fmt.Fprintf(stderr, "choir files mkdir: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runFilesPut(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir files put", flag.ContinueOnError)
	fs.SetOutput(stderr)
	local := fs.String("local", "", "Local file to upload (default: read stdin)")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir files put: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir files put: remote path required")
		return 2
	}
	var body io.Reader
	if strings.TrimSpace(*local) == "" {
		body = os.Stdin
	} else {
		f, err := os.Open(strings.TrimSpace(*local))
		if err != nil {
			fmt.Fprintf(stderr, "choir files put: %v\n", err)
			return 1
		}
		defer f.Close()
		body = f
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		fmt.Fprintf(stderr, "choir files put: %v\n", err)
		return 1
	}
	var resp map[string]any
	if err := c.doRaw(http.MethodPut, "/api/files/"+strings.TrimPrefix(strings.TrimSpace(rest[0]), "/"), raw, &resp); err != nil {
		fmt.Fprintf(stderr, "choir files put: %v\n", err)
		return 1
	}
	return writeJSON(stdout, resp)
}

func runFilesGet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir files get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir files get: %v\n", err)
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
		fmt.Fprintln(stderr, "choir files get: remote path required")
		return 2
	}
	raw, err := c.doRawBytes(http.MethodGet, "/api/files/"+strings.TrimPrefix(strings.TrimSpace(rest[0]), "/"), nil)
	if err != nil {
		fmt.Fprintf(stderr, "choir files get: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(raw); err != nil {
		fmt.Fprintf(stderr, "choir files get: %v\n", err)
		return 1
	}
	return 0
}
