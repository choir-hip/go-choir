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
	id := strings.TrimSpace(x.c.computerID)
	if id == "" {
		var list paperStatusList
		if err := x.c.do(http.MethodGet, paperComputersRoute, nil, &list); err != nil {
			return nil, err
		}
		switch len(list.Computers) {
		case 0:
			return nil, fmt.Errorf("the platform roster has no computer")
		case 1:
			id = list.Computers[0].ComputerID
		default:
			var ids []string
			for _, c := range list.Computers {
				ids = append(ids, c.ComputerID)
			}
			return nil, usagef("more than one platform computer (%s); name one with --computer", strings.Join(ids, ", "))
		}
	}
	if !paperComputerID.MatchString(id) {
		return nil, usagef("--computer must be a computer id like computer-<32 hex>, got %q", id)
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

// paperCommands are `choir paper status` and the policy verbs under `paper`.
func paperCommands() []*command {
	cmds := []*command{{
		path: []string{"paper", "status"}, summary: "Every platform computer: state, build, source schedules and the resolved model policy per role (operator only)",
		scopes: readRuntime, readOnly: true, cliOnly: paperCLIOnly,
		handler: paperStatus,
	}}
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
