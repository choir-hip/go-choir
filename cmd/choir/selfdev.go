package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Operation states that never move again; wait stops at them.
var selfDevSettledStates = []string{"applied", "rejected", "rolled_back", "failed"}

type selfDevOperation struct {
	OperationID  string   `json:"operation_id"`
	State        string   `json:"state"`
	BundleDigest string   `json:"bundle_digest"`
	VerifierRefs []string `json:"verifier_refs"`
}

type selfDevHead struct {
	CanonicalEventHead       string `json:"canonical_event_head"`
	Sequence                 uint64 `json:"sequence"`
	DesiredEventHead         string `json:"desired_event_head"`
	EffectiveEventHead       string `json:"effective_event_head"`
	PendingTransitionRef     string `json:"pending_transition_ref"`
	DesiredStateCommitment   string `json:"desired_state_commitment"`
	EffectiveStateCommitment string `json:"effective_state_commitment"`
}

func selfDevPath(computerID, rest string) string {
	return "/api/computers/" + url.PathEscape(strings.TrimSpace(computerID)) + "/self-development/" + rest
}

// stringList is a repeatable flag that also splits commas.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }
func (l *stringList) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// selfDevFlags parses a self-dev subcommand that targets one computer and
// refuses positional arguments.
func selfDevFlags(name string, args []string, stdout, stderr io.Writer, define func(fs *flag.FlagSet)) (*client, string, bool) {
	fs := flag.NewFlagSet("choir self-dev "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	computerID := fs.String("computer", "", "Stable ComputerID")
	if define != nil {
		define(fs)
	}
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", name, err)
		return nil, "", false
	}
	if strings.TrimSpace(*computerID) == "" || len(fs.Args()) != 0 {
		fmt.Fprintf(stderr, "choir self-dev %s: --computer is required and positional arguments are forbidden\n", name)
		return nil, "", false
	}
	return c, strings.TrimSpace(*computerID), true
}

func runSelfDevWait(args []string, stdout, stderr io.Writer) int {
	var operationID *string
	var states stringList
	var interval, deadline *time.Duration
	c, computerID, ok := selfDevFlags("wait", args, stdout, stderr, func(fs *flag.FlagSet) {
		operationID = fs.String("operation", "", "Operation ID")
		fs.Var(&states, "state", "Stop when the operation reaches this state (repeatable or comma-separated)")
		interval = fs.Duration("interval", 10*time.Second, "Poll interval")
		deadline = fs.Duration("deadline", 2*time.Hour, "Give up after this long")
	})
	if !ok || strings.TrimSpace(*operationID) == "" || len(states) == 0 {
		if ok {
			fmt.Fprintln(stderr, "choir self-dev wait: --operation and --state are required")
		}
		return 2
	}
	path := selfDevPath(computerID, "operations/"+url.PathEscape(strings.TrimSpace(*operationID)))
	giveUp := time.Now().Add(*deadline)
	last := ""
	for {
		var raw json.RawMessage
		var operation selfDevOperation
		err := c.do(http.MethodGet, path, nil, &raw)
		if err == nil {
			err = json.Unmarshal(raw, &operation)
		}
		if err != nil {
			fmt.Fprintf(stderr, "choir self-dev wait: %v\n", err)
		} else {
			if operation.State != last {
				fmt.Fprintf(stderr, "%s %s\n", time.Now().UTC().Format(time.RFC3339), operation.State)
				last = operation.State
			}
			if slices.Contains(states, operation.State) {
				return writeJSON(stdout, raw)
			}
			if slices.Contains(selfDevSettledStates, operation.State) {
				fmt.Fprintf(stderr, "choir self-dev wait: operation settled %s, not %s\n", operation.State, strings.Join(states, "|"))
				writeJSON(stdout, raw)
				return 1
			}
		}
		if time.Now().Add(*interval).After(giveUp) {
			fmt.Fprintf(stderr, "choir self-dev wait: deadline passed in state %q\n", last)
			return 1
		}
		time.Sleep(*interval)
	}
}

// selfDevDecisionBinding reads the frozen candidate and the computer's event
// head, and returns the binding a decision on exactly that candidate carries.
func selfDevDecisionBinding(c *client, computerID, operationID string) (selfDevOperation, map[string]any, error) {
	var operation selfDevOperation
	if err := c.do(http.MethodGet, selfDevPath(computerID, "operations/"+url.PathEscape(operationID)), nil, &operation); err != nil {
		return operation, nil, err
	}
	if operation.State != "awaiting_approval" {
		return operation, nil, fmt.Errorf("operation %s is %s, not awaiting_approval", operationID, operation.State)
	}
	if operation.BundleDigest == "" || len(operation.VerifierRefs) == 0 {
		return operation, nil, errors.New("operation has no frozen bundle or verifier")
	}
	var head selfDevHead
	if err := c.do(http.MethodGet, selfDevPath(computerID, "head"), nil, &head); err != nil {
		return operation, nil, err
	}
	binding := map[string]any{
		"bundle_digest":                       operation.BundleDigest,
		"expected_desired_event_head":         head.DesiredEventHead,
		"expected_effective_event_head":       head.EffectiveEventHead,
		"expected_pending_transition_ref":     head.PendingTransitionRef,
		"expected_desired_state_commitment":   head.DesiredStateCommitment,
		"expected_effective_state_commitment": head.EffectiveStateCommitment,
	}
	return operation, binding, nil
}
