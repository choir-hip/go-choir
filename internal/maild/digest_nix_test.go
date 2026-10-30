package maild

// Failure modes this file guards (design section 4.2 and the problem note's
// M14). The host unit cannot be run here, so its declaration is checked as
// text. Written before the nix change.
//
//	N1  The timer fires a stale digest after an outage. It must declare
//	    Persistent = false (CLAUDE.md "Restarts End Work": a missed run is
//	    closed, not resumed).
//	N2  The timer calls the run route on loopback alone. The unit must load
//	    the host-internal credential header (LoadCredential) and the script
//	    must send it, and must fail the unit, not call without it, when the
//	    credential file is unreadable.
//	N3  The call hides a failure. curl must fail on HTTP errors
//	    (--fail-with-body) so a 503 from a missing Resend key fails the unit
//	    and shows in the journal.
//	N4  The timer drifts off the approved hour. 12:00 UTC.
//	N5  maild is not told where corpusd is, or the digest kill switch is not
//	    visible in the unit.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func digestNixBlock(t *testing.T, src, marker string) string {
	t.Helper()
	i := strings.Index(src, marker)
	if i < 0 {
		t.Fatalf("node-b.nix has no %q", marker)
	}
	rest := src[i:]
	// A block ends at the first line that is exactly "  };" (two-space
	// indent), which closes a top-level systemd.* attribute in this file.
	end := strings.Index(rest, "\n  };\n")
	if end < 0 {
		t.Fatalf("no end of block for %q", marker)
	}
	return rest[:end]
}

func TestDigestHostUnitDeclaration(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "nix", "node-b.nix"))
	if err != nil {
		t.Fatalf("read node-b.nix: %v", err)
	}
	src := string(raw)

	timer := digestNixBlock(t, src, "systemd.timers.go-choir-digest =")
	if !regexp.MustCompile(`Persistent\s*=\s*false;`).MatchString(timer) {
		t.Errorf("N1: digest timer must declare Persistent = false:\n%s", timer)
	}
	if !strings.Contains(timer, `"*-*-* 12:00:00 UTC"`) {
		t.Errorf("N4: digest timer must start at 12:00 UTC daily (retries: digest_retry_test.go):\n%s", timer)
	}

	service := digestNixBlock(t, src, "systemd.services.go-choir-digest =")
	if !strings.Contains(service, `LoadCredential = [ "choir-internal-token.header:${internalTokenHeaderFile}" ];`) {
		t.Errorf("N2: digest service must load the internal credential header:\n%s", service)
	}
	for _, want := range []string{
		`Type = "oneshot";`,
		`/internal/maild/digest/run`,
		`http://127.0.0.1:8087`,
		`X-Internal-Caller: true`,
		`@''${CREDENTIALS_DIRECTORY}/choir-internal-token.header`,
		`--fail-with-body`,
	} {
		if !strings.Contains(service, want) {
			t.Errorf("digest service lacks %q:\n%s", want, service)
		}
	}
	// The script must refuse to call without the credential file.
	if !regexp.MustCompile(`(?s)\[ ! -r .*choir-internal-token\.header.* \].*exit 1`).MatchString(service) {
		t.Errorf("N2: digest script must exit 1 when the credential file is unreadable:\n%s", service)
	}
	if strings.Contains(service, "internal_auth=()") {
		t.Errorf("N2: digest script must not treat the credential as optional (disk-gc style):\n%s", service)
	}

	maild := digestNixBlock(t, src, "systemd.services.go-choir-maild =")
	if !strings.Contains(maild, `"MAILD_CORPUSD_URL=http://127.0.0.1:8086"`) {
		t.Errorf("N5: maild unit must set MAILD_CORPUSD_URL:\n%s", maild)
	}
}
