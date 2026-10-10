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

func runSelfDevList(args []string, stdout, stderr io.Writer) int {
	var states stringList
	c, computerID, ok := selfDevFlags("list", args, stdout, stderr, func(fs *flag.FlagSet) {
		fs.Var(&states, "state", "Only operations in this state (repeatable or comma-separated)")
	})
	if !ok {
		return 2
	}
	path := selfDevPath(computerID, "operations")
	if len(states) > 0 {
		query := url.Values{"state": states}
		path += "?" + query.Encode()
	}
	var response json.RawMessage
	if err := c.do(http.MethodGet, path, nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev list: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevShow(args []string, stdout, stderr io.Writer) int {
	var operationID *string
	c, computerID, ok := selfDevFlags("show", args, stdout, stderr, func(fs *flag.FlagSet) {
		operationID = fs.String("operation", "", "Operation ID")
	})
	if !ok || strings.TrimSpace(*operationID) == "" {
		if ok {
			fmt.Fprintln(stderr, "choir self-dev show: --operation is required")
		}
		return 2
	}
	var response json.RawMessage
	if err := c.do(http.MethodGet, selfDevPath(computerID, "operations/"+url.PathEscape(strings.TrimSpace(*operationID))), nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev show: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevHead(args []string, stdout, stderr io.Writer) int {
	c, computerID, ok := selfDevFlags("head", args, stdout, stderr, nil)
	if !ok {
		return 2
	}
	var response json.RawMessage
	if err := c.do(http.MethodGet, selfDevPath(computerID, "head"), nil, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev head: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
}

func runSelfDevStart(args []string, stdout, stderr io.Writer) int {
	var prompt, idempotencyKey *string
	c, computerID, ok := selfDevFlags("start", args, stdout, stderr, func(fs *flag.FlagSet) {
		prompt = fs.String("prompt", "", "What the computer should change about itself")
		idempotencyKey = fs.String("idempotency-key", "", "Unique idempotency key (a retry with the same key replays)")
	})
	if !ok || strings.TrimSpace(*prompt) == "" || strings.TrimSpace(*idempotencyKey) == "" {
		if ok {
			fmt.Fprintln(stderr, "choir self-dev start: --prompt and --idempotency-key are required")
		}
		return 2
	}
	var response json.RawMessage
	body := map[string]any{"idempotency_key": strings.TrimSpace(*idempotencyKey), "prompt": *prompt}
	if err := c.do(http.MethodPost, selfDevPath(computerID, "operations"), body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev start: %v\n", err)
		return 1
	}
	return writeJSON(stdout, response)
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

func runSelfDevDecide(decision string, args []string, stdout, stderr io.Writer) int {
	var operationID, reason, idempotencyKey *string
	var window *time.Duration
	c, computerID, ok := selfDevFlags(decision, args, stdout, stderr, func(fs *flag.FlagSet) {
		operationID = fs.String("operation", "", "Operation ID awaiting approval")
		idempotencyKey = fs.String("idempotency-key", "", "Decision idempotency key (default derives from the operation)")
		if decision == "reject" {
			reason = fs.String("reason", "", "Why the candidate is rejected (required)")
		} else {
			window = fs.Duration("window", 15*time.Minute, "How long the owner's single approval stays armed")
		}
	})
	if !ok || strings.TrimSpace(*operationID) == "" || (decision == "reject" && strings.TrimSpace(*reason) == "") {
		if ok {
			fmt.Fprintf(stderr, "choir self-dev %s: --operation is required%s\n", decision, map[bool]string{true: " and so is --reason", false: ""}[decision == "reject"])
		}
		return 2
	}
	id := strings.TrimSpace(*operationID)
	operation, binding, err := selfDevDecisionBinding(c, computerID, id)
	if err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", decision, err)
		return 1
	}
	if decision == "approve" {
		// The owner's single approval: arm accept_once bound to exactly this
		// candidate and head; the decision consumes it.
		var mode struct {
			Generation uint64 `json:"generation"`
		}
		if err := c.do(http.MethodGet, selfDevPath(computerID, "mode"), nil, &mode); err != nil {
			fmt.Fprintf(stderr, "choir self-dev approve: read mode: %v\n", err)
			return 1
		}
		arm := map[string]any{
			"mode": "accept_once", "expected_generation": mode.Generation, "idempotency_key": "cli-accept-once:" + id,
			"operation_id": id, "expires_at": time.Now().UTC().Add(*window).Truncate(time.Second).Format(time.RFC3339Nano),
		}
		for field, value := range binding {
			arm[field] = value
		}
		if err := c.do(http.MethodPut, selfDevPath(computerID, "mode"), arm, nil); err != nil {
			fmt.Fprintf(stderr, "choir self-dev approve: arm accept_once: %v\n", err)
			return 1
		}
	}
	body := map[string]any{"decision": decision, "verifier_ref": operation.VerifierRefs[0]}
	for field, value := range binding {
		body[field] = value
	}
	body["idempotency_key"] = strings.TrimSpace(*idempotencyKey)
	if body["idempotency_key"] == "" {
		body["idempotency_key"] = "cli-" + decision + ":" + id
	}
	if decision == "reject" {
		body["reason"] = strings.TrimSpace(*reason)
	}
	var response json.RawMessage
	if err := c.do(http.MethodPost, selfDevPath(computerID, "operations/"+url.PathEscape(id)+"/decision"), body, &response); err != nil {
		fmt.Fprintf(stderr, "choir self-dev %s: %v\n", decision, err)
		return 1
	}
	return writeJSON(stdout, response)
}
