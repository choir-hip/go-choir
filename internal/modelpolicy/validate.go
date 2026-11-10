package modelpolicy

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// policyKeys are the keys a policy section carries; requires is read by the
// desk shots gate, not by applyValue.
var policyKeys = map[string]bool{
	"provider": true, "fallback_provider": true, "model": true, "fallback_model": true,
	"reasoning": true, "reasoning_effort": true, "max_tokens": true, "max_output_tokens": true,
	"requires": true,
}

// ValidateText checks policy text before it is saved: the loader's own parse
// (syntax, known roles, known pools, pool sections with a member
// the catalog does not list), then what the loader would only
// downgrade or ignore at run start (an unknown key, an unknown effort, a
// model the catalog does not list for a catalogued provider). The loader
// falls back silently to the last valid policy; a writer is told instead.
// It makes no network call.
func ValidateText(raw string) error {
	policy, err := parsePolicy(raw, "")
	if err != nil {
		return err
	}
	section, lineNo := "", 0
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(stripTOMLComment(scanner.Text()))
		if line == "" {
			continue
		}
		if name, isHeader := sectionHeader(line); isHeader {
			section = name
			continue
		}
		if strings.HasPrefix(section, poolSectionPrefix) {
			// parsePolicy read the pool sections whole (a members list spans
			// lines) and refused unknown pool keys.
			continue
		}
		if key, _, ok := parseAssignment(line); ok && !policyKeys[key] {
			return fmt.Errorf("line %d: unknown key %q in [%s]", lineNo, key, section)
		}
	}
	if len(policy.Diagnostics) > 0 {
		return errors.New(strings.Join(policy.Diagnostics, "; "))
	}
	check := func(where string, selection provideriface.LLMSelection) error {
		provider, model := strings.TrimSpace(selection.Provider), strings.TrimSpace(selection.Model)
		if provider == "" || strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) || !modelcatalog.ProviderCatalogued(provider) {
			return nil
		}
		if _, ok := modelcatalog.Lookup(provider, model); !ok {
			return fmt.Errorf("%s: model %q is not in the catalog for provider %q", where, model, provider)
		}
		return nil
	}
	if err := check("defaults", policy.Defaults); err != nil {
		return err
	}
	for _, role := range sortedRoleNames(policy.Roles) {
		if err := check("role "+role, policy.Roles[role]); err != nil {
			return err
		}
	}
	return nil
}
