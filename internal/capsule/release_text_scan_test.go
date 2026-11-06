package capsule

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The freeze-time scan of a text release file must read lines of any
// length: a minified frontend bundle is one line well over 1 MiB, and a
// scanner token cap turned every freeze on d@e into "token too long"
// (2026-11-06, selfdev-c3b2e851).
func TestTextReleaseScanReadsLinesOfAnyLength(t *testing.T) {
	secret := "ghp_" + strings.Repeat("Zq8xW2vR", 3)
	big := func(n int) string { return strings.Repeat("a.b(c);", n/7+1)[:n] }
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"short clean lines", "const a = 1;\nconst b = 2;\n", false},
		{"short line with a secret", "x\n" + secret + "\ny\n", true},
		{"clean line over 1 MiB", big(3<<20) + "\n", false},
		{"secret mid a line over 1 MiB", big(2<<20) + " " + secret + " " + big(2<<20), true},
		{"secret straddling a window", big(textScanWindow-len(secret)/2) + " " + secret + " " + big(1<<20), true},
		{"secret after a long line, no final newline", big(3<<20) + "\n" + secret, true},
		{"crlf lines", "a\r\n" + secret + "\r\n", true},
		// An unbounded blank run between a pattern's anchors, placed across
		// a window boundary (Devin review of this fix, finding 1).
		{"bearer gap straddling a window", big(textScanWindow-textScanCarry-200) + " Bearer" + strings.Repeat(" ", 8<<10) + "eyJhbGciOiJIUzI1NiJ9.abcdefgh " + big(1<<20), true},
		{"password gap straddling a window", big(textScanWindow-textScanCarry-200) + " password" + strings.Repeat("\t", 8<<10) + "=" + strings.Repeat(" ", 8<<10) + "hunter2abcdef " + big(1<<20), true},
		{"pem block straddling a window", big(textScanWindow-50) + "\\n-----BEGIN RSA PRIVATE KEY-----" + strings.Repeat("QUJD", 2<<10) + "-----END RSA PRIVATE KEY-----" + big(1<<20), true},
		// An inflated PEM label: wider than any window, and straddling one
		// (Devin review r1, finding 1).
		{"pem label wider than a window", "-----BEGIN " + strings.Repeat("A", 1200<<10) + " PRIVATE KEY-----" + strings.Repeat("QUJD", 64) + "\n", true},
		{"pem label straddling a window", big(textScanWindow-textScanCarry-200) + "-----BEGIN " + strings.Repeat("A", 70<<10) + " PRIVATE KEY-----" + strings.Repeat("QUJD", 64) + " " + big(1<<20), true},
		{"long clean blank runs", big(textScanWindow-100) + strings.Repeat(" ", 8<<10) + big(1<<20) + "\n", false},
	}
	for _, tc := range cases {
		if !tc.want {
			if got := computerevent.DetectRefusalSecrets([]byte(tc.body)); len(got) != 0 {
				t.Fatalf("%s: fixture is not clean for the whole-line detector: %v", tc.name, got)
			}
		} else if got := computerevent.DetectRefusalSecrets([]byte(tc.body)); len(got) == 0 {
			t.Fatalf("%s: fixture has no secret for the whole-line detector", tc.name)
		}
		t.Run(tc.name, func(t *testing.T) {
			got, err := textHasRefusalSecret(bytes.NewReader([]byte(tc.body)))
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if got != tc.want {
				t.Fatalf("found secret = %v, want %v", got, tc.want)
			}
		})
	}
}

type failingReader struct{ after int }

func (r *failingReader) Read(p []byte) (int, error) {
	if r.after <= 0 {
		return 0, errors.New("disk went away")
	}
	n := min(len(p), r.after)
	for i := range p[:n] {
		p[i] = 'a'
	}
	r.after -= n
	return n, nil
}

// A read error fails closed: the caller refuses the freeze.
func TestTextReleaseScanFailsClosedOnReadError(t *testing.T) {
	for _, after := range []int{0, 10, 3 << 20} {
		if _, err := textHasRefusalSecret(&failingReader{after: after}); err == nil {
			t.Fatalf("after %d bytes: want the read error, got nil", after)
		}
	}
}
