package main

// `choir policy`: the computer's model policy as one command each, on the
// ordinary owner-key path (docs/design/paper-control-cli-2026-11-09.md, step
// 1; docs/runbooks/paper-control.md). The policy is the file
// System/model-policy.toml, written through the files route, which validates
// it and keeps the file it replaces as a revision under
// System/model-policy.revisions/.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

const (
	policyFile         = "System/model-policy.toml"
	policyRevisionsDir = "System/model-policy.revisions"
	policyOwnerEdited  = "# owner-edited"
)

// policyRoles are the roles whose resolve a change is read back through.
var policyRoles = []string{deskmanifest.DocumentCustodianName(), deskmanifest.AdmitterName(), deskmanifest.TaskRoleName(), deskmanifest.ResearcherName(), deskmanifest.PerformerName()}

// revisionName is the shape the server gives a revision. Anything else is
// refused before it becomes a path segment, so --to cannot leave the
// revisions directory.
var revisionName = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}\.[0-9]{6}Z(-[0-9]+)?\.toml$`)

type policyGetResult struct {
	File  string            `json:"file"`
	Roles []json.RawMessage `json:"roles"`
	Pools []policyPool      `json:"pools"`
}

// policyPool is a pool as the resolve endpoint reports it: its name, where
// the definition came from ("file" or "release"), failover, and its members.
type policyPool struct {
	Name     string             `json:"name"`
	Source   string             `json:"source"`
	Failover bool               `json:"failover"`
	Members  []policyPoolMember `json:"members"`
}

type policyPoolMember struct {
	Model        string `json:"model"` // provider/model
	Weight       int    `json:"weight"`
	FallbackOnly bool   `json:"fallback_only,omitempty"`
	Spread       string `json:"spread,omitempty"`
	SpreadWeight int    `json:"spread_weight,omitempty"`
}

type policyWriteResult struct {
	Status   string            `json:"status"`
	Revision string            `json:"revision,omitempty"` // empty: nothing was replaced or the file is unchanged
	Receipt  string            `json:"receipt,omitempty"`  // a platform operator act: the id of the receipt on the computer's tape
	Restored string            `json:"restored,omitempty"` // rollback: the revision the file now matches
	Bytes    int               `json:"bytes"`
	Roles    []json.RawMessage `json:"roles"`
	Pools    []policyPool      `json:"pools"`
}

func hasOwnerEditedLine(raw []byte) bool {
	for _, line := range strings.Split(string(raw), "\n") {
		// The platform keeps a file that contains the mark anywhere
		// (modelpolicy), so a mark with a note after it counts.
		if strings.HasPrefix(strings.TrimSpace(line), policyOwnerEdited) {
			return true
		}
	}
	return false
}

// policyResolves resolves each role and reads the pool table once: every
// resolve carries the same pools, so they are lifted out of the role entries.
func policyResolves(x *call) ([]json.RawMessage, []policyPool, error) {
	var out []json.RawMessage
	var pools []policyPool
	for i, role := range policyRoles {
		var raw json.RawMessage
		if err := x.c.do(http.MethodGet, "/api/model-policy/resolve?role="+url.QueryEscape(role), nil, &raw); err != nil {
			return nil, nil, fmt.Errorf("resolve %s: %w", role, err)
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) == nil {
			if i == 0 {
				if err := json.Unmarshal(fields["pools"], &pools); err != nil && fields["pools"] != nil {
					return nil, nil, fmt.Errorf("resolve %s: pools: %w", role, err)
				}
			}
			delete(fields, "pools")
			raw, _ = json.Marshal(fields)
		}
		out = append(out, raw)
	}
	return out, pools, nil
}

func policyGet(x *call) (any, error) {
	raw, err := x.c.doRawBytes(http.MethodGet, "/api/files/"+policyFile, nil)
	if err != nil {
		return nil, err
	}
	roles, pools, err := policyResolves(x)
	if err != nil {
		return nil, err
	}
	return policyGetResult{File: string(raw), Roles: roles, Pools: pools}, nil
}

// policyPut writes raw through the files route, reads it back, and requires
// the bytes to match; the server has validated it and kept the old file.
func policyPut(x *call, raw []byte) (policyWriteResult, error) {
	var resp struct {
		Revision string `json:"revision"`
		Receipt  string `json:"receipt"`
	}
	if err := x.c.doRaw(http.MethodPut, "/api/files/"+policyFile, raw, &resp); err != nil {
		return policyWriteResult{}, err
	}
	back, err := x.c.doRawBytes(http.MethodGet, "/api/files/"+policyFile, nil)
	if err != nil {
		return policyWriteResult{}, fmt.Errorf("saved, but reading it back failed: %w", err)
	}
	if !bytes.Equal(back, raw) {
		return policyWriteResult{}, errors.New("read-back differs from what was written; the saved file is not the one sent (revision " + resp.Revision + " holds the previous file)")
	}
	roles, pools, err := policyResolves(x)
	if err != nil {
		return policyWriteResult{}, err
	}
	return policyWriteResult{Status: "saved", Revision: resp.Revision, Receipt: resp.Receipt, Bytes: len(raw), Roles: roles, Pools: pools}, nil
}

func policySet(x *call) (any, error) {
	var raw []byte
	var err error
	switch {
	case x.surface == "mcp":
		raw = []byte(x.str("content"))
	case x.trim("local") != "":
		if raw, err = os.ReadFile(x.trim("local")); err != nil {
			return nil, err
		}
	default:
		if raw, err = io.ReadAll(x.stdin); err != nil {
			return nil, err
		}
	}
	if !x.flag("allow_unmarked") && !hasOwnerEditedLine(raw) {
		return nil, usagef("the policy has no %q line, so the platform would overwrite it at its next policy version; add the line, or pass --allow-unmarked to write it anyway", policyOwnerEdited)
	}
	return policyPut(x, raw)
}

// policyRevisionNames lists the kept revisions, oldest first. A computer
// that has never replaced its policy has none.
func policyRevisionNames(x *call) ([]string, error) {
	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := x.c.do(http.MethodGet, "/api/files/"+policyRevisionsDir, nil, &entries); err != nil {
		var apiErr *apiErrorResp
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type == "file" && revisionName.MatchString(e.Name) {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func policyRevisions(x *call) (any, error) {
	names, err := policyRevisionNames(x)
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return map[string]any{"revisions": names}, nil
}

func policyRollback(x *call) (any, error) {
	name := x.trim("to")
	if name == "" {
		names, err := policyRevisionNames(x)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			return nil, errors.New("no kept revision to roll back to")
		}
		name = names[len(names)-1]
	} else if !revisionName.MatchString(name) {
		return nil, usagef("--to must be a revision name from `choir policy revisions`, got %q", name)
	}
	raw, err := x.c.doRawBytes(http.MethodGet, "/api/files/"+policyRevisionsDir+"/"+name, nil)
	if err != nil {
		return nil, err
	}
	// The same validated write as set: a revision the validator now refuses
	// stays refused. The marker is not required: a revision is what the
	// file was, and may be a platform default.
	res, err := policyPut(x, raw)
	if err != nil {
		return nil, err
	}
	res.Restored = name
	return res, nil
}

func policyVerify(x *call) (any, error) {
	body := map[string]any{"prompt": x.trim("prompt")}
	if role := x.trim("role"); role != "" {
		body["role"] = role
	}
	return sendRaw(x, http.MethodPost, "/api/model-policy/verify", body)
}

func policyCommands() []*command {
	return []*command{
		{
			path: []string{"policy", "lead"}, summary: "Make one model the lead of a pool on this computer: the pool is written into the policy file, the lead takes weight 1, every other member becomes a fallback",
			params: []param{
				pStr("model", "The new lead, written provider/model (it must be in the catalog)").req().one().orSay("a model is required: choir policy lead <provider/model>"),
				pStr("pool", "Pool to change").dflt("default"),
				pBool("single_lead", "Allow a pool with several weighted members or with failover off: the named model becomes the only weighted member"),
				pBool("allow_paid", "Allow a metered model to lead (it prints the price; a free model needs no flag)"),
			},
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: policyLead,
		},
		{
			path: []string{"policy", "pin"}, summary: "Write every pool, as it resolves now, into the policy file, so a later release does not move this computer's pools",
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: policyPin,
		},
		{
			path: []string{"policy", "get"}, summary: "Show the model policy file and the resolved model for each role",
			scopes: readRuntime, readOnly: true,
			handler: policyGet,
		},
		{
			path: []string{"policy", "set"}, summary: "Replace the model policy: validated by the server, the old file kept as a revision, read back and resolved",
			params: []param{
				pStr("local", "Local policy file (default: read stdin)").onlyCLI("reads the caller's local filesystem; MCP passes content instead (a hosted server has no local files, and a model must not pick local files to upload)"),
				pStr("content", "UTF-8 policy text").req().onlyMCP("the CLI reads bytes from --local or stdin"),
				pBool("allow_unmarked", "Write a policy that lacks the \"# owner-edited\" line (the platform overwrites such a file at its next policy version)"),
			},
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: policySet,
		},
		{
			path: []string{"policy", "revisions"}, summary: "List the kept model policy revisions, oldest first",
			scopes: readRuntime, readOnly: true,
			handler: policyRevisions,
		},
		{
			path: []string{"policy", "rollback"}, summary: "Restore the latest (or --to) kept model policy revision through the validated write",
			params: []param{pStr("to", "Revision name from `policy revisions` (default: the latest)")},
			scopes: writeRuntime, destructive: true, idempotent: true,
			handler: policyRollback,
		},
		{
			path: []string{"policy", "verify"}, summary: "Make one real model call as a role to prove the policy's model answers",
			params: []param{
				pStr("role", "Role to call as (default: the admitting desk's role)"),
				pStr("prompt", "Prompt for the call").dflt("Reply with the single word: ok"),
			},
			scopes: writeRuntime, openWorld: true,
			handler: policyVerify,
		},
	}
}
