package vmmanager

import "strings"

// Per-computer selection for staged host features (FP8 of the launch first
// pass). Each feature that changes how a computer launches is off unless its
// *_ONLY list names the computer, so the deploy that ships the feature
// changes no running computer; the operator then names one test computer,
// then class:test, then "*".
//
// Tokens: a VM id, a computer id, class:<warmness class> (the class vmctl
// booted the computer under, VMConfig.WarmnessClass; "class:test" is every
// test account's computer), or "*" (every computer). The selection is read
// at each launch, and a snapshot restore validates against the same
// decision, so a computer changes at its next launch only.

const computerSelectorClassPrefix = "class:"

// parseComputerSelector splits a comma, space or newline separated list.
func parseComputerSelector(raw string) []string {
	var out []string
	for _, tok := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if tok = strings.TrimSpace(tok); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// selectsComputer reports whether the list names cfg's computer.
func selectsComputer(only []string, cfg VMConfig) bool {
	vmID := strings.TrimSpace(cfg.VMID)
	computerID := strings.TrimSpace(cfg.ComputerID)
	class := strings.TrimSpace(cfg.WarmnessClass)
	for _, tok := range only {
		tok = strings.TrimSpace(tok)
		switch {
		case tok == "":
			continue
		case tok == "*":
			return true
		case strings.HasPrefix(tok, computerSelectorClassPrefix):
			if want := strings.TrimPrefix(tok, computerSelectorClassPrefix); want != "" && want == class {
				return true
			}
		case tok == vmID || (computerID != "" && tok == computerID):
			return true
		}
	}
	return false
}
