// Package computertiers is the authority for computer tiers until billing
// exists (owner ruling 2026-10-13,
// docs/problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md).
//
// A paid account's computers are always on. A platform computer, such as
// the Autopaper paper computer, is a tier above every user tier and is
// always on too. Nobody is charged yet, so the tiers are set by a reviewed
// commit to computer-tiers.json: the file is embedded in vmctl and ships
// with each deploy. Once billing exists, paid status moves onto the account
// and this file keeps only the platform computers.
package computertiers

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Schema is the file's schema string.
const Schema = "choir.computer_tiers.v1"

//go:embed computer-tiers.json
var embedded []byte

var (
	userIDRE     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	computerIDRE = regexp.MustCompile(`^computer-[0-9a-f]{32}$`)
)

type paidAccount struct {
	UserID string `json:"user_id"`
	Note   string `json:"note"`
}

type platformComputer struct {
	ComputerID string `json:"computer_id"`
	Note       string `json:"note"`
}

// testComputer is a registered account's computer the operator names as a
// test computer: below the free tier, and disposable once idle for the
// retention rule's minimum (docs/problems/vm-state-retention-skips-test-
// computers-and-quarantines-2026-10-22.md). Optional in the file.
type testComputer struct {
	ComputerID string `json:"computer_id"`
	Note       string `json:"note"`
}

type file struct {
	Schema            string             `json:"schema"`
	PaidAccounts      []paidAccount      `json:"paid_accounts"`
	PlatformComputers []platformComputer `json:"platform_computers"`
	TestComputers     []testComputer     `json:"test_computers,omitempty"`
}

// Tiers is a parsed, validated tier file.
type Tiers struct {
	paid     map[string]bool
	platform map[string]bool
	test     map[string]bool
}

// Load parses the embedded tier file.
func Load() (Tiers, error) { return Parse(embedded) }

// Parse validates a tier file: the schema, no unknown fields, well-formed
// and unique ids, and a note on every entry saying why it has the tier.
func Parse(body []byte) (Tiers, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var f file
	if err := dec.Decode(&f); err != nil {
		return Tiers{}, fmt.Errorf("computer tiers: %w", err)
	}
	if f.Schema != Schema {
		return Tiers{}, fmt.Errorf("computer tiers: schema %q, want %q", f.Schema, Schema)
	}
	t := Tiers{paid: map[string]bool{}, platform: map[string]bool{}, test: map[string]bool{}}
	for _, a := range f.PaidAccounts {
		id := strings.TrimSpace(a.UserID)
		if !userIDRE.MatchString(id) {
			return Tiers{}, fmt.Errorf("computer tiers: paid account %q is not a user id", a.UserID)
		}
		if strings.TrimSpace(a.Note) == "" {
			return Tiers{}, fmt.Errorf("computer tiers: paid account %s has no note", id)
		}
		if t.paid[id] {
			return Tiers{}, fmt.Errorf("computer tiers: paid account %s listed twice", id)
		}
		t.paid[id] = true
	}
	for _, c := range f.PlatformComputers {
		id := strings.TrimSpace(c.ComputerID)
		if !computerIDRE.MatchString(id) {
			return Tiers{}, fmt.Errorf("computer tiers: platform computer %q is not a computer id", c.ComputerID)
		}
		if strings.TrimSpace(c.Note) == "" {
			return Tiers{}, fmt.Errorf("computer tiers: platform computer %s has no note", id)
		}
		if t.platform[id] {
			return Tiers{}, fmt.Errorf("computer tiers: platform computer %s listed twice", id)
		}
		t.platform[id] = true
	}
	for _, c := range f.TestComputers {
		id := strings.TrimSpace(c.ComputerID)
		if !computerIDRE.MatchString(id) {
			return Tiers{}, fmt.Errorf("computer tiers: test computer %q is not a computer id", c.ComputerID)
		}
		if strings.TrimSpace(c.Note) == "" {
			return Tiers{}, fmt.Errorf("computer tiers: test computer %s has no note", id)
		}
		if t.test[id] {
			return Tiers{}, fmt.Errorf("computer tiers: test computer %s listed twice", id)
		}
		if t.platform[id] {
			return Tiers{}, fmt.Errorf("computer tiers: %s is listed as both platform and test", id)
		}
		t.test[id] = true
	}
	return t, nil
}

// Paid reports whether an account is on the paid tier.
func (t Tiers) Paid(userID string) bool { return t.paid[strings.TrimSpace(userID)] }

// PlatformComputer reports whether a computer is on the platform tier.
func (t Tiers) PlatformComputer(computerID string) bool {
	return t.platform[strings.TrimSpace(computerID)]
}

// TestComputerIDs lists the named test computers, sorted.
func (t Tiers) TestComputerIDs() []string { return sortedKeys(t.test) }

// PaidUserIDs lists the paid accounts, sorted.
func (t Tiers) PaidUserIDs() []string { return sortedKeys(t.paid) }

// PlatformComputerIDs lists the platform computers, sorted.
func (t Tiers) PlatformComputerIDs() []string { return sortedKeys(t.platform) }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
