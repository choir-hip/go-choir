package computertiers

import (
	"strings"
	"testing"
)

// Ways the tier file could fail, written before the code
// (docs/problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md):
// a malformed file ships and every computer silently falls to the free
// tier; an id is mistyped and protects nothing; an id appears twice with
// two meanings; an unknown field (a typo of a list name) is ignored.

func TestEmbeddedTiersLoadAndNameThePaperComputer(t *testing.T) {
	tiers, err := Load()
	if err != nil {
		t.Fatalf("embedded computer-tiers.json: %v", err)
	}
	if !tiers.PlatformComputer("computer-1355b83bced1613eb048ee28d66688a8") {
		t.Fatal("the Autopaper paper computer is not a platform computer")
	}
	if len(tiers.PaidUserIDs()) == 0 {
		t.Fatal("no paid accounts: the owner account must be an ordinary paid account")
	}
}

func TestParseRefusesMalformedFiles(t *testing.T) {
	cases := map[string]string{
		"not json":           `{`,
		"wrong schema":       `{"schema":"other","paid_accounts":[],"platform_computers":[]}`,
		"unknown field":      `{"schema":"choir.computer_tiers.v1","paid_acounts":[],"platform_computers":[]}`,
		"bad user id":        `{"schema":"choir.computer_tiers.v1","paid_accounts":[{"user_id":"owner","note":"x"}],"platform_computers":[]}`,
		"bad computer id":    `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[{"computer_id":"vm-123","note":"x"}]}`,
		"duplicate user":     `{"schema":"choir.computer_tiers.v1","paid_accounts":[{"user_id":"5bd6de97-0000-4000-8000-000000000001","note":"a"},{"user_id":"5bd6de97-0000-4000-8000-000000000001","note":"b"}],"platform_computers":[]}`,
		"missing note":       `{"schema":"choir.computer_tiers.v1","paid_accounts":[{"user_id":"5bd6de97-0000-4000-8000-000000000001"}],"platform_computers":[]}`,
		"duplicate computer": `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[{"computer_id":"computer-1355b83bced1613eb048ee28d66688a8","note":"a"},{"computer_id":"computer-1355b83bced1613eb048ee28d66688a8","note":"b"}]}`,
	}
	for name, body := range cases {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}

func TestParseAcceptsAWellFormedFile(t *testing.T) {
	body := `{"schema":"choir.computer_tiers.v1",
	  "paid_accounts":[{"user_id":"5bd6de97-0000-4000-8000-000000000001","note":"owner account"}],
	  "platform_computers":[{"computer_id":"computer-1355b83bced1613eb048ee28d66688a8","note":"Autopaper"}]}`
	tiers, err := Parse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if !tiers.Paid("5bd6de97-0000-4000-8000-000000000001") || tiers.Paid("someone-else") {
		t.Fatal("paid membership wrong")
	}
	if !tiers.PlatformComputer("computer-1355b83bced1613eb048ee28d66688a8") || tiers.PlatformComputer("computer-0") {
		t.Fatal("platform membership wrong")
	}
	if got := strings.Join(tiers.PlatformComputerIDs(), ","); got != "computer-1355b83bced1613eb048ee28d66688a8" {
		t.Fatalf("platform ids = %q", got)
	}
}

// Test computers (docs/problems/vm-state-retention-skips-test-computers-
// and-quarantines-2026-10-22.md): a registered account's computer that the
// operator names as a test computer. Ways it could fail: a platform
// computer or a paid account's computer is also listed as test and so
// becomes disposable; a mistyped id names nothing; the list is required
// and an older file stops vmctl.
func TestParseTestComputers(t *testing.T) {
	ok := `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[],
	  "test_computers":[{"computer_id":"computer-2bd23366dc4bc1f9fd3aed0b299491a5","note":"FP0 proof computer"}]}`
	tiers, err := Parse([]byte(ok))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tiers.TestComputerIDs(), ","); got != "computer-2bd23366dc4bc1f9fd3aed0b299491a5" {
		t.Fatalf("test ids = %q", got)
	}
	if _, err := Parse([]byte(`{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[]}`)); err != nil {
		t.Fatalf("a file without test_computers must still parse: %v", err)
	}
	bad := map[string]string{
		"bad id":        `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[],"test_computers":[{"computer_id":"vm-1","note":"x"}]}`,
		"no note":       `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[],"test_computers":[{"computer_id":"computer-2bd23366dc4bc1f9fd3aed0b299491a5"}]}`,
		"twice":         `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[],"test_computers":[{"computer_id":"computer-2bd23366dc4bc1f9fd3aed0b299491a5","note":"a"},{"computer_id":"computer-2bd23366dc4bc1f9fd3aed0b299491a5","note":"b"}]}`,
		"also platform": `{"schema":"choir.computer_tiers.v1","paid_accounts":[],"platform_computers":[{"computer_id":"computer-1355b83bced1613eb048ee28d66688a8","note":"a"}],"test_computers":[{"computer_id":"computer-1355b83bced1613eb048ee28d66688a8","note":"b"}]}`,
	}
	for name, body := range bad {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}
