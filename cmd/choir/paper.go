package main

// `choir paper`: a platform computer (the paper) controlled from the
// platform operator's own account (docs/design/paper-control-cli-2026-11-09.md,
// slices O2 and O3; docs/design/platform-operator-account-2026-11-06.md). The
// verbs are the landed policy verbs: each is the same handler run against a
// client whose paths are prefixed with the operator route for one platform
// computer. The server decides admission (the operator roster, the platform
// tier) and keeps the receipt; the CLI sends only the operator's own `choir
// login` credential and holds no platform secret.

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	paperComputersRoute = "/api/platform/operator/computers"
	paperCLIOnly        = "the platform operator path: it runs on the operator's own login and names a platform computer, and a hosted MCP server must not hold platform authority"
)

var paperComputerID = regexp.MustCompile(`^computer-[0-9a-f]{32}$`)

type paperStatusList struct {
	Computers []struct {
		ComputerID string `json:"computer_id"`
	} `json:"computers"`
}

// paperCall returns x with its client aimed at one platform computer: the one
// named by --computer, or the only one the roster has.
func paperCall(x *call) (*call, error) {
	id, err := paperTargetID(x)
	if err != nil {
		return nil, err
	}
	pc := *x.c
	pc.prefix = paperComputersRoute + "/" + id + "/guest"
	// The operator route names the computer in the path; the owner-targeting
	// header means something else and is not sent.
	pc.computerID = ""
	px := *x
	px.c = &pc
	return &px, nil
}

// paperTargetID is the platform computer the verb acts on: the one named by
// --computer, or the only one the roster has.
func paperTargetID(x *call) (string, error) {
	id := strings.TrimSpace(x.c.computerID)
	if id == "" {
		var list paperStatusList
		if err := x.c.do(http.MethodGet, paperComputersRoute, nil, &list); err != nil {
			return "", err
		}
		switch len(list.Computers) {
		case 0:
			return "", fmt.Errorf("the platform roster has no computer")
		case 1:
			id = list.Computers[0].ComputerID
		default:
			var ids []string
			for _, c := range list.Computers {
				ids = append(ids, c.ComputerID)
			}
			return "", usagef("more than one platform computer (%s); name one with --computer", strings.Join(ids, ", "))
		}
	}
	if !paperComputerID.MatchString(id) {
		return "", usagef("--computer must be a computer id like computer-<32 hex>, got %q", id)
	}
	return id, nil
}

func paperWrap(h func(*call) (any, error)) func(*call) (any, error) {
	return func(x *call) (any, error) {
		px, err := paperCall(x)
		if err != nil {
			return nil, err
		}
		return h(px)
	}
}

func paperStatus(x *call) (any, error) {
	return sendRaw(x, http.MethodGet, paperComputersRoute, nil)
}

const paperSchedulesPath = "/api/autopaper/sources/schedules"

// paperScheduleStart is `choir paper schedule start`: the registry source only.
// A feed the registry does not list is the owner's own act and has no flag here.
func paperScheduleStart(x *call) (any, error) {
	body := map[string]any{"source_id": x.str("source")}
	if n := x.num("interval_minutes"); n > 0 {
		body["interval_minutes"] = n
	}
	if n := x.num("max_items"); n > 0 {
		body["max_items"] = n
	}
	if r := x.str("reason"); r != "" {
		body["reason"] = r
	}
	return sendRaw(x, http.MethodPost, paperSchedulesPath, body)
}

func paperScheduleStop(x *call) (any, error) {
	q := url.Values{"source_id": {x.str("source")}}
	if r := x.str("reason"); r != "" {
		q.Set("reason", r)
	}
	return sendRaw(x, http.MethodDelete, paperSchedulesPath+"?"+q.Encode(), nil)
}

// paperRestart is `choir paper restart --reason`: the operator route asks for
// vmctl's planned, marker-backed refresh; the answer's outcome is refreshed (an update applied) or restarted (same version). A refusal (a live capsule, a
// self-development apply, a held or in-flight computer) is an error with the
// server's reason code, and the command exits non-zero; there is no force.
func paperRestart(x *call) (any, error) {
	id, err := paperTargetID(x)
	if err != nil {
		return nil, err
	}
	rc := *x.c
	rc.prefix, rc.computerID = "", ""
	rx := *x
	rx.c = &rc
	return sendRaw(&rx, http.MethodPost, paperComputersRoute+"/"+id+"/restart", map[string]any{"reason": x.str("reason")})
}

// paperCommands are `choir paper status` and the policy verbs under `paper`.
func paperCommands() []*command {
	cmds := []*command{{
		path: []string{"paper", "status"}, summary: "Every platform computer: state, build, source schedules and the resolved model policy per role (operator only)",
		scopes: readRuntime, readOnly: true, cliOnly: paperCLIOnly,
		handler: paperStatus,
	}}
	cmds = append(cmds,
		&command{
			path: []string{"paper", "schedule", "list"}, summary: "Paper: the source schedules (operator only)",
			scopes: readRuntime, readOnly: true, cliOnly: paperCLIOnly,
			handler: paperWrap(func(x *call) (any, error) { return getRaw(x, paperSchedulesPath) }),
		},
		&command{
			path: []string{"paper", "schedule", "start"}, summary: "Paper: start (or re-time) a source schedule; a receipt is recorded on the paper's tape first (operator only)",
			params: []param{
				pStr("source", "Registry source id, such as rss:example").req().orSay("choir paper schedule start --source <id> [--interval-minutes n] [--max-items n] [--reason text]"),
				pInt("interval_minutes", "Minutes between polls (default: the schedule's own)"),
				pInt("max_items", "Items per poll (default: the schedule's own)"),
				pStr("reason", "Why, kept in the receipt (at most 280 bytes)"),
			},
			scopes: writeRuntime, cliOnly: paperCLIOnly,
			handler: paperWrap(paperScheduleStart),
		},
		&command{
			path: []string{"paper", "schedule", "stop"}, summary: "Paper: stop a source schedule; a receipt is recorded on the paper's tape first (operator only)",
			params: []param{
				pStr("source", "Registry source id").req().orSay("choir paper schedule stop --source <id> [--reason text]"),
				pStr("reason", "Why, kept in the receipt (at most 280 bytes)"),
			},
			scopes: writeRuntime, destructive: true, idempotent: true, cliOnly: paperCLIOnly,
			handler: paperWrap(paperScheduleStop),
		},
		&command{
			path: []string{"paper", "restart"}, summary: "Paper: ask for a planned, marker-backed restart (outcome refreshed if an update was pending, restarted if it was already current; one per five minutes); refused mid self-development apply or with a live capsule (operator only)",
			params: []param{
				pStr("reason", "Why, kept in the receipt (at most 280 bytes)").req().orSay("choir paper restart --reason <text>"),
			},
			scopes: writeRuntime, cliOnly: paperCLIOnly,
			handler: paperRestart,
		},
	)
	for _, p := range policyCommands() {
		c := *p
		c.path = append([]string{"paper"}, p.path...)
		c.summary = "Paper: " + p.summary
		c.cliOnly = paperCLIOnly
		c.handler = paperWrap(p.handler)
		// The MCP-only parameters have no meaning on a CLI-only command.
		c.params = nil
		for _, prm := range p.params {
			if prm.mcpOnly == "" {
				c.params = append(c.params, prm)
			}
		}
		cmds = append(cmds, &c)
	}
	return cmds
}
