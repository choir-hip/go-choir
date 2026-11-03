// Command choir-delegate is how an engineering cell drives the Claude
// delegate from inside its capsule (docs/design/claude-delegate-slice1-2026-10-14.md).
//
//	choir-delegate status
//	choir-delegate start --spec-json '{"objective":"...","paths":["internal/x"],"test":["go","test","./internal/x/..."]}'
//	choir-delegate start --spec /tmp/brief.json
//	choir-delegate wait --id <id> [--seconds 20]
//	choir-delegate wait --id <id> --park [--seconds 1800]
//	choir-delegate stop --id <id>
//
// It never sees the token: start and stop go to the capsule broker's socket,
// wait --park is for a capsule job (choir.Start): it returns when the run ends or
// after --seconds (default 1800, at most 2700), so one job waits where thirty
// choir.Exec waits of 25 s each cost a model turn apiece; the engineer is woken
// when the job ends and reads the result with wait --seconds 0.
// wait reads the run's status file, whose lines the broker redacted.
// Every verb prints one JSON object. Exit codes: 0 ok, 1 error, 3 the
// delegate is unavailable in this capsule.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/yusefmosiah/go-choir/internal/claudedelegate"
)

const usage = "usage: choir-delegate status | start --spec-json JSON | start --spec FILE | wait --id ID [--park] [--seconds N] | stop --id ID"

const unavailable = "the Claude delegate is not available in this capsule: only a claude-mode engineering capsule on the allowlisted computer has it (choir-delegate status names the reason)"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, claudedelegate.SocketPath, claudedelegate.RunRoot))
}

func run(args []string, out io.Writer, socket, root string) int {
	if len(args) == 0 {
		return emit(out, 1, map[string]any{"error": usage})
	}
	verb, rest := args[0], args[1:]
	if verb == "--help" || verb == "-h" || verb == "help" {
		fmt.Fprintln(out, usage)
		return 0
	}
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	specJSON := fs.String("spec-json", "", "spec as JSON")
	specFile := fs.String("spec", "", "spec file")
	id := fs.String("id", "", "run id")
	seconds := fs.Int("seconds", -1, "wait at most this long (default 20, <= 25; with --park default 1800, <= 2700)")
	park := fs.Bool("park", false, "run as a capsule job: wait until the run ends or --seconds, print a one-line summary")
	if err := fs.Parse(rest); err != nil {
		return emit(out, 1, map[string]any{"error": err.Error()})
	}
	switch verb {
	case "status":
		resp, err := claudedelegate.Call(socket, claudedelegate.Request{Op: "status"}, 5*time.Second)
		if err != nil {
			return emit(out, 3, map[string]any{"available": false, "reason": unavailable})
		}
		return emit(out, 0, resp)
	case "start":
		raw := []byte(*specJSON)
		if *specFile != "" {
			data, err := os.ReadFile(*specFile)
			if err != nil {
				return emit(out, 1, map[string]any{"error": err.Error()})
			}
			raw = data
		}
		if len(raw) == 0 {
			return emit(out, 1, map[string]any{"error": "start needs --spec-json or --spec"})
		}
		if _, err := claudedelegate.DecodeSpec(raw); err != nil {
			return emit(out, 1, map[string]any{"error": err.Error()})
		}
		resp, err := claudedelegate.Call(socket, claudedelegate.Request{Op: "start", Spec: raw}, 10*time.Second)
		if err != nil {
			return emit(out, 3, map[string]any{"available": false, "reason": unavailable})
		}
		if !resp.OK {
			return emit(out, 1, resp)
		}
		return emit(out, 0, resp)
	case "wait":
		if *id == "" {
			return emit(out, 1, map[string]any{"error": "wait needs --id"})
		}
		limit, ceiling, tick := *seconds, claudedelegate.WaitMaxSeconds, time.Second
		if *park {
			ceiling, tick = ParkMaxSeconds, 250*time.Millisecond
			if limit < 0 {
				limit = ParkDefaultSeconds
			}
		} else if limit < 0 {
			limit = 20
		}
		if limit > ceiling {
			limit = ceiling
		}
		deadline := time.Now().Add(time.Duration(limit) * time.Second)
		for {
			st, err := claudedelegate.ReadStatus(root, *id)
			if err != nil {
				return emit(out, 1, map[string]any{"error": err.Error()})
			}
			if st.State != claudedelegate.StateRunning || !time.Now().Before(deadline) {
				if *park {
					return emit(out, 0, parkSummary(st))
				}
				return emit(out, 0, st)
			}
			time.Sleep(tick)
		}
	case "stop":
		resp, err := claudedelegate.Call(socket, claudedelegate.Request{Op: "stop", ID: *id}, 10*time.Second)
		if err != nil {
			return emit(out, 3, map[string]any{"available": false, "reason": unavailable})
		}
		if !resp.OK {
			return emit(out, 1, resp)
		}
		return emit(out, 0, resp)
	default:
		return emit(out, 1, map[string]any{"error": fmt.Sprintf("unknown verb %q", verb)})
	}
}

// Park bounds: a capsule job lives at most 45 minutes (capsulejob), and the
// delegate's own wall clock is at most 30.
const (
	ParkDefaultSeconds = 1800
	ParkMaxSeconds     = 2700
)

// parkSummary is what a parked wait prints: a job's output tail is 4 KiB, so
// the full status (written_files, the final message) is read afterwards with
// `wait --seconds 0`.
func parkSummary(st claudedelegate.Status) map[string]any {
	m := map[string]any{"id": st.ID, "state": st.State, "elapsed_sec": st.ElapsedSec, "progress": st.Progress,
		"next": "choir-delegate wait --id " + st.ID + " --seconds 0 prints the full status and result"}
	if st.State == claudedelegate.StateRunning {
		m["note"] = "still running at the park bound: park again"
	}
	if st.Reason != "" {
		m["reason"] = st.Reason
	}
	if st.ExitCode != nil {
		m["exit_code"] = *st.ExitCode
	}
	if st.Result != nil {
		m["subtype"] = st.Result.Subtype
	}
	return m
}

func emit(out io.Writer, code int, v any) int {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		data = []byte(`{"error":"encode output"}`)
	}
	_, _ = out.Write(append(data, '\n'))
	return code
}
