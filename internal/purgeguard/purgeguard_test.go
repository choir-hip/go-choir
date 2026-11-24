package purgeguard

import (
	"errors"
	"testing"
	"time"
)

func TestCheckClaim(t *testing.T) {
	now := time.Now().UTC()
	if err := CheckClaim(time.Time{}, now); !errors.Is(err, ErrNoRecord) {
		t.Fatal(err)
	}
	if err := CheckClaim(now.Add(-GracePeriod+time.Second), now); !errors.Is(err, ErrTooEarly) {
		t.Fatal(err)
	}
	if err := CheckClaim(now.Add(time.Hour), now); !errors.Is(err, ErrTooEarly) {
		t.Fatal("a future request time was accepted")
	}
	if err := CheckClaim(now.Add(-GracePeriod), now); err != nil {
		t.Fatal(err)
	}
}

func TestClassRefused(t *testing.T) {
	for class, want := range map[string]bool{"critical_protected": true, "public_platform": true, " public_platform ": true, "primary": false, "premium_always_on": false, "test": false, "": false} {
		if ClassRefused(class) != want {
			t.Fatalf("%q", class)
		}
	}
}

func TestProtectedComputerList(t *testing.T) {
	for id := range ProtectedComputers {
		if what, ok := ProtectedComputer(" " + id + " "); !ok || what == "" {
			t.Fatalf("%s is not refused", id)
		}
	}
	if len(ProtectedComputers) != 2 {
		t.Fatalf("the hard list is the paper and the owner's computer; it holds %d", len(ProtectedComputers))
	}
	if _, ok := ProtectedComputer("computer-0000"); ok {
		t.Fatal("an ordinary computer is on the hard list")
	}
}
