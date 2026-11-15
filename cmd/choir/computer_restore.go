package main

// `choir computer restore` and `choir computer rematerialize-from-tape` submit
// an accepted act and wait for its end. The restore of a long tail runs for up
// to an hour and a half (docs/problems/restore-cannot-finish-inside-its-ten-
// minute-request-2026-11-14.md), so no request carries it: the guest answers
// 202 with an act id, and this polls the act's status route until it ends, the
// way the request used to hold the connection.
//
// stdout is the act's result, the report the synchronous route returned, and
// the exit code is the one it had: 0 on success, 1 on a refusal (named on
// stderr) or a wait that ran out. Progress goes to stderr, one line per state
// or count change. --no-wait prints the acceptance (act id, status URL,
// estimate) and exits.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// restoreWaitDefault is how long a restore is waited for when no --timeout
	// is given: the longest tail the recovery bound allows replays in 80
	// minutes at the slowest rate on record, and the scan and install come
	// before it.
	restoreWaitDefault = 2 * time.Hour
	// restorePollRequestTimeout bounds one poll: a poll is a small read, and
	// the wait's own deadline is the budget.
	restorePollRequestTimeout = 60 * time.Second
	restoreMinPollInterval    = 100 * time.Millisecond
)

// restoreActView is the part of an act the CLI reads.
type restoreActView struct {
	ActID                     string          `json:"act_id"`
	State                     string          `json:"state"`
	Reason                    string          `json:"reason"`
	StatusURL                 string          `json:"status_url"`
	TailEventsTotal           uint64          `json:"tail_events_total"`
	TailEventsApplied         uint64          `json:"tail_events_applied"`
	ReplayRate                float64         `json:"replay_rate"`
	EstimatedSecondsRemaining float64         `json:"estimated_seconds_remaining"`
	Result                    json.RawMessage `json:"result"`
	Refusal                   *struct {
		Name       string `json:"name"`
		HTTPStatus int    `json:"http_status"`
		Message    string `json:"message"`
	} `json:"refusal"`
}

func (v restoreActView) ended() bool {
	return v.State == "succeeded" || v.State == "failed" || v.State == "interrupted"
}

// progressLine is the one line printed when the act's state or count moves.
func (v restoreActView) progressLine(label string) string {
	line := fmt.Sprintf("%s: act %s %s", label, v.ActID, v.State)
	if v.State == "replaying" && v.TailEventsTotal > 0 {
		line += fmt.Sprintf(" %d/%d events", v.TailEventsApplied, v.TailEventsTotal)
		if v.ReplayRate > 0 {
			line += fmt.Sprintf(" (%.0f/s, about %s left)", v.ReplayRate, (time.Duration(v.EstimatedSecondsRemaining) * time.Second).Round(time.Second))
		}
	}
	return line
}

// awaitRestoreAct is what a restore does with the guest's answer. An answer
// with no act id is the report of a guest that still restores inside the
// request (a guest not yet refreshed onto the accepted-act release): it is the
// result, as it always was.
func awaitRestoreAct(x *call, label string, answer json.RawMessage) (any, error) {
	var accepted restoreActView
	if err := json.Unmarshal(answer, &accepted); err != nil || accepted.ActID == "" {
		return answer, nil
	}
	if x.flag("no_wait") {
		return answer, nil
	}
	interval := x.dur("interval")
	if interval < restoreMinPollInterval {
		return nil, usagef("--interval %s is below the %s floor (a shorter poll is a busy loop against the guest)", interval, restoreMinPollInterval)
	}
	wait := restoreWaitDefault
	if x.c.timeoutGiven {
		wait = x.c.http.Timeout
	}
	if x.c.http.Timeout > restorePollRequestTimeout || x.c.http.Timeout == 0 {
		x.c.http.Timeout = restorePollRequestTimeout
	}
	statusPath := computerPath(x, "/lifecycle/restore-status?"+url.Values{"act": {accepted.ActID}}.Encode())
	fmt.Fprintf(x.c.stderr, "%s: act %s accepted; reading %s every %s for up to %s\n", label, accepted.ActID, "choir computer restore-status", interval, wait.Round(time.Second))
	deadline := time.Now().Add(wait)
	var last, lastError string
	for {
		var view restoreActView
		var raw json.RawMessage
		err := x.c.do(http.MethodGet, statusPath, nil, &raw)
		if err == nil {
			err = json.Unmarshal(raw, &view)
		}
		switch {
		case err == nil:
			lastError = ""
			if line := view.progressLine(label); line != last {
				fmt.Fprintln(x.c.stderr, line)
				last = line
			}
			if view.ended() {
				return restoreActOutcome(label, view)
			}
		case restoreStatusTransient(err):
			// The guest restarts, or the host is between answers: the act's
			// record is on the volume, so keep reading.
			if msg := err.Error(); msg != lastError {
				fmt.Fprintf(x.c.stderr, "%s: act %s: status not readable yet (%v); still waiting\n", label, accepted.ActID, err)
				lastError = msg
			}
		default:
			return nil, fmt.Errorf("act %s: reading its status: %w", accepted.ActID, err)
		}
		if time.Now().Add(interval).After(deadline) {
			return nil, fmt.Errorf("gave up waiting after %s: act %s is still running on the computer (last seen: %s); read it with `choir computer restore-status --computer %s --act %s`",
				wait.Round(time.Second), accepted.ActID, strings.TrimPrefix(last, label+": "), x.str("computer"), accepted.ActID)
		}
		time.Sleep(interval)
	}
}

// restoreStatusTransient is a failure of the read that says nothing about the
// act: the guest is restarting (502, 503, 504) or the request did not arrive.
func restoreStatusTransient(err error) bool {
	var api *apiErrorResp
	if errors.As(err, &api) {
		return api.Status == http.StatusBadGateway || api.Status == http.StatusServiceUnavailable || api.Status == http.StatusGatewayTimeout
	}
	return true
}

// restoreActOutcome is the end of the wait: the result, or the refusal.
func restoreActOutcome(label string, view restoreActView) (any, error) {
	switch view.State {
	case "succeeded":
		if len(view.Result) == 0 {
			return nil, fmt.Errorf("act %s succeeded and carries no result", view.ActID)
		}
		return view.Result, nil
	case "failed":
		if view.Refusal != nil {
			return nil, fmt.Errorf("act %s failed: %s (http %d): %s", view.ActID, view.Refusal.Name, view.Refusal.HTTPStatus, view.Refusal.Message)
		}
		return nil, fmt.Errorf("act %s failed: %s", view.ActID, view.Reason)
	default:
		return nil, fmt.Errorf("act %s was %s: %s; the computer is on the state it had before the restore, and nothing resumes it: submit the restore again", view.ActID, view.State, view.Reason)
	}
}

// restoreStatusCommand is `choir computer restore-status`.
func restoreStatusCommand() *command {
	return ownerAct(&command{
		path:    []string{"computer", "restore-status"},
		summary: "Read a restore act: its state, progress, estimate and, at the end, its result or refusal",
		desc:    "Read the status of a restore or rematerialize-from-tape act on the computer: state (accepted, fetching_base, replaying, verifying, installing, succeeded, failed, interrupted), tail events total and applied, the replay rate and where it came from, the estimated seconds remaining, and at the end the result report or the named refusal. Without --act it reads the running act, else the newest. An owner act: the key needs owner:acts, which admin does not imply.",
		params: []param{
			computerParam(),
			pStr("act", "Act id (default: the running act, else the newest)"),
		},
		usage:  "--computer is required",
		scopes: lifecycle, readOnly: true,
		handler: func(x *call) (any, error) {
			path := computerPath(x, "/lifecycle/restore-status")
			if act := x.trim("act"); act != "" {
				path += "?" + url.Values{"act": {act}}.Encode()
			}
			return getRaw(x, path)
		},
	})
}
