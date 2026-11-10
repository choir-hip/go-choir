package main

// `choir policy lead` and `choir policy pin`: a pool's members as data in the
// computer's policy file (docs/design/programmatic-model-switching-2026-11-09.md,
// 3.2). Both read the pools as the computer resolves them now, edit the
// [pools.<name>] sections of the file text, and write through the same
// validated PUT as `policy set`, so revisions and rollback work unchanged.

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
)

// poolNameRe and memberRe bound what is written into the file's TOML, so a
// --pool or a model argument cannot carry a quote, newline or bracket.
var (
	poolNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	memberRe   = regexp.MustCompile(`^[A-Za-z0-9_-]+/[A-Za-z0-9._:/@+-]+$`)
)

type policyLeadResult struct {
	policyWriteResult
	Pool    string             `json:"pool"`
	Lead    string             `json:"lead"`
	Members []policyPoolMember `json:"members"`
	Notes   []string           `json:"notes,omitempty"`
}

type policyPinResult struct {
	policyWriteResult
	Pinned []string `json:"pinned"`
	Notes  []string `json:"notes,omitempty"`
}

// poolSectionHeader returns the pool name when line is a [pools.<name>]
// header, and whether the line is any section header.
func poolSectionHeader(line string) (name string, header bool) {
	t := strings.TrimSpace(line)
	if i := strings.Index(t, "#"); i >= 0 && !strings.ContainsAny(t[:i], `"'`) {
		t = strings.TrimSpace(t[:i])
	}
	if !strings.HasPrefix(t, "[") || !strings.HasSuffix(t, "]") || strings.Contains(t, "=") {
		return "", false
	}
	inner := strings.TrimSpace(strings.Trim(t, "[]"))
	if rest, ok := strings.CutPrefix(inner, "pools."); ok {
		return rest, true
	}
	return "", true
}

// withoutPoolSections removes the [pools.<name>] sections named in names (all
// pool sections when names is nil) from the file text.
func withoutPoolSections(text string, names map[string]bool) string {
	var out []string
	skipping := false
	for _, line := range strings.Split(text, "\n") {
		if name, header := poolSectionHeader(line); header {
			skipping = name != "" && (names == nil || names[name])
		}
		if !skipping {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// renderPoolSection writes a pool in the explicit form: every member a table,
// failover said.
func renderPoolSection(pool policyPool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[pools.%s]\nmembers = [\n", pool.Name)
	for _, m := range pool.Members {
		if m.FallbackOnly {
			fmt.Fprintf(&b, "  { model = %q, fallback_only = true },\n", m.Model)
			continue
		}
		weight := m.Weight
		if weight < 1 {
			weight = 1
		}
		fmt.Fprintf(&b, "  { model = %q, weight = %d },\n", m.Model, weight)
	}
	fmt.Fprintf(&b, "]\nfailover = %t\n", pool.Failover)
	return b.String()
}

// policyWithPools returns the file text with the given pools written as
// sections (replacing any section of the same name) and the owner-edited mark.
// Sections named in drop are removed and not written back.
func policyWithPools(file string, pools []policyPool, drop ...string) string {
	names := map[string]bool{}
	for _, p := range pools {
		names[p.Name] = true
	}
	for _, name := range drop {
		names[name] = true
	}
	text := strings.TrimRight(withoutPoolSections(file, names), "\n")
	if !hasOwnerEditedLine([]byte(text)) {
		text = policyOwnerEdited + "\n" + text
	}
	for _, p := range pools {
		text += "\n\n" + strings.TrimRight(renderPoolSection(p), "\n")
	}
	return text + "\n"
}

// modelCost says whether provider/model is positively known to be free: an id
// ending in -free or :free, or an explicit zero price in the pricing table.
// Anything else is metered, priced or not, so a model the table cannot price
// needs --allow-paid; text is the price when known and "price unknown"
// otherwise (empty for a free model).
func modelCost(model string) (free bool, text string) {
	provider, id, _ := strings.Cut(model, "/")
	if strings.HasSuffix(id, "-free") || strings.HasSuffix(id, ":free") {
		return true, ""
	}
	price, found := llmcost.LookupProviderPricing(provider, id)
	switch {
	case !found:
		return false, "price unknown"
	case price.InputPerMillion == 0 && price.OutputPerMillion == 0:
		return true, ""
	}
	return false, fmt.Sprintf("$%g per million input tokens, $%g per million output tokens", price.InputPerMillion, price.OutputPerMillion)
}

func findPool(pools []policyPool, name string) (policyPool, bool) {
	for _, p := range pools {
		if p.Name == name {
			return p, true
		}
	}
	return policyPool{}, false
}

func policyLead(x *call) (any, error) {
	model, poolName := x.trim("model"), x.trim("pool")
	if poolName == "" {
		poolName = "default"
	}
	if !poolNameRe.MatchString(poolName) {
		return nil, usagef("--pool %q is not a pool name", poolName)
	}
	if !memberRe.MatchString(model) {
		return nil, usagef("the model must be written provider/model, got %q", model)
	}
	file, err := x.c.doRawBytes(http.MethodGet, "/api/files/"+policyFile, nil)
	if err != nil {
		return nil, err
	}
	_, pools, err := policyResolves(x)
	if err != nil {
		return nil, err
	}
	pool, ok := findPool(pools, poolName)
	if !ok {
		return nil, usagef("this computer has no pool %q", poolName)
	}
	weighted, weightedIsModel := 0, 0
	for _, m := range pool.Members {
		if !m.FallbackOnly {
			weighted++
			if m.Model == model {
				weightedIsModel++
			}
		}
	}
	if (!pool.Failover || weighted > 1) && !x.flag("single_lead") {
		return nil, usagef("pool %q has %s; making %s its lead would flatten it to one lead with fallbacks. Pass --single-lead to do that", poolName, describePoolShape(pool.Failover, weighted), model)
	}
	soleLead := weighted == 1 && weightedIsModel == 1
	if free, price := modelCost(model); !free && !soleLead && !x.flag("allow_paid") {
		return nil, usagef("%s is not known to be free (%s); every thread in pool %q would run on it. Pass --allow-paid to make it the lead", model, price, poolName)
	}
	next := policyPool{Name: poolName, Source: "file", Failover: pool.Failover, Members: []policyPoolMember{{Model: model, Weight: 1}}}
	for _, m := range pool.Members {
		if m.Model != model {
			next.Members = append(next.Members, policyPoolMember{Model: m.Model, FallbackOnly: true})
		}
	}
	res, err := policyPutPools(x, file, []policyPool{next})
	if err != nil {
		return nil, err
	}
	return policyLeadResult{
		policyWriteResult: res,
		Pool:              poolName,
		Lead:              model,
		Members:           next.Members,
		Notes:             []string{"Every thread, and every Texture document, moves to the new lead once and loses its prompt cache."},
	}, nil
}

func describePoolShape(failover bool, weighted int) string {
	var parts []string
	if weighted > 1 {
		parts = append(parts, fmt.Sprintf("%d weighted members", weighted))
	}
	if !failover {
		parts = append(parts, "failover off")
	}
	return strings.Join(parts, " and ")
}

// policyPutPools writes the file with pools as sections through the validated
// PUT. The server refuses a member the catalog does not list.
func policyPutPools(x *call, file []byte, pools []policyPool, drop ...string) (policyWriteResult, error) {
	res, err := policyPut(x, []byte(policyWithPools(string(file), pools, drop...)))
	if err != nil {
		var apiErr *apiErrorResp
		if errors.As(err, &apiErr) {
			return policyWriteResult{}, fmt.Errorf("the server refused the policy, nothing was saved: %w", err)
		}
		return policyWriteResult{}, err
	}
	return res, nil
}

func policyPin(x *call) (any, error) {
	file, err := x.c.doRawBytes(http.MethodGet, "/api/files/"+policyFile, nil)
	if err != nil {
		return nil, err
	}
	_, pools, err := policyResolves(x)
	if err != nil {
		return nil, err
	}
	if len(pools) == 0 {
		return nil, errors.New("the computer reported no pools; it may be running an older release")
	}
	var names, dropped, notes []string
	var keep []policyPool
	for _, p := range pools {
		if len(p.Members) == 0 {
			// An empty members list does not parse. The pool has no usable
			// member, so its section is removed rather than written back.
			dropped = append(dropped, p.Name)
			notes = append(notes, fmt.Sprintf("pool %q has no usable member and was left out of the pin; its section was removed from the file", p.Name))
			continue
		}
		p.Source = "file"
		keep = append(keep, p)
		names = append(names, p.Name)
	}
	res, err := policyPutPools(x, file, keep, dropped...)
	if err != nil {
		return nil, err
	}
	return policyPinResult{policyWriteResult: res, Pinned: names, Notes: notes}, nil
}
