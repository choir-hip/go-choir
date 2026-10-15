package vmmanager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

func TestVisualCaptureGrantIsPerComputerOrClassTest(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == visualCaptureComputersEnv {
				return v
			}
			return ""
		}
	}
	// Off by default: nothing for any computer of any class.
	off := loadVisualCaptureConfig(env(""))
	if files, _ := off.visualCaptureMaterial("computer-a", "test", now); files != nil {
		t.Fatal("an empty list granted a test computer")
	}
	cfg := loadVisualCaptureConfig(env("computer-a, class:test"))
	cases := []struct {
		id, class, basis string
	}{
		{"computer-a", "primary", visualcapture.BasisComputer},
		{"computer-b", "test", visualcapture.BasisClassTest},
		{"computer-c", "primary", ""},
		{"computer-d", "critical_protected", ""},
		{"", "test", ""},
	}
	for _, tc := range cases {
		files, status := cfg.visualCaptureMaterial(tc.id, tc.class, now)
		if tc.basis == "" {
			if files != nil || status != "" {
				t.Fatalf("%s/%s granted: %s", tc.id, tc.class, status)
			}
			continue
		}
		var g visualcapture.Grant
		if err := json.Unmarshal(files[visualcapture.GrantFile], &g); err != nil {
			t.Fatal(err)
		}
		if g.ComputerID != tc.id || g.Status != visualcapture.GrantGranted || g.Basis != tc.basis || g.GrantedAt != "2026-10-15T12:00:00Z" || len(files) != 1 {
			t.Fatalf("%s grant = %+v", tc.id, g)
		}
	}
	// Without class:test only the listed id is granted.
	ids := loadVisualCaptureConfig(env("computer-a"))
	if files, _ := ids.visualCaptureMaterial("computer-b", "test", now); files != nil {
		t.Fatal("a test computer was granted without class:test")
	}
}
