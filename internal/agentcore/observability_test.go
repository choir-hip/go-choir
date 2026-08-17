package agentcore

import "testing"

func TestBootLogRingBounded(t *testing.T) {
	r := newBootLogRing(3)
	for _, line := range []string{"a", "b", "c", "d", "e"} {
		if _, err := r.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	got := r.snapshot()
	want := []string{"c", "d", "e"}
	if len(got) != len(want) {
		t.Fatalf("ring = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ring = %v, want %v", got, want)
		}
	}
}

func TestBootLogRingMultiLineAndBlank(t *testing.T) {
	r := newBootLogRing(4)
	if _, err := r.Write([]byte("a\nb\n\nc\n")); err != nil {
		t.Fatal(err)
	}
	got := r.snapshot()
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("ring = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ring = %v, want %v", got, want)
		}
	}
}

func TestBootLogRingNilSafe(t *testing.T) {
	var r *bootLogRing
	if _, err := r.Write([]byte("x\n")); err != nil {
		t.Fatal(err)
	}
	if r.snapshot() != nil {
		t.Fatal("nil ring snapshot must be nil")
	}
}
