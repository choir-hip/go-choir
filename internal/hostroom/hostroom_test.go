package hostroom

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const gib = uint64(1) << 30

// The bound on sign-ups is what the host can hold, not a number somebody
// chose. Failure modes guarded:
//  1. an account with no computer holding room (owner, 2026-11-24: "dont
//     count any account without a computer. all real accounts have a
//     computer. we just have records of email adddresses where we deleted
//     the computers for testing"): 898 such records, each charged a
//     footprint, closed sign-up on staging for good;
//  2. the promise released the moment the computer exists, while its disk is
//     still a fraction of the footprint it will grow to: the room then grows
//     as computers open, and the host is admitted past its reserve;
//  3. a free reading under the reserve wrapping to a huge unsigned room;
//  4. a place offered when less than one footprint is left;
//  5. a missing, stale, future-dated or broken owners file read as a reading;
//  6. a half-written owners file seen by the reader.
func TestPlacesCountsTheRoomHeldBack(t *testing.T) {
	p := Plan{ReserveBytes: 100 * gib, FootprintBytes: 9 * gib}
	cases := []struct {
		name string
		free uint64
		held uint64
		want int
	}{
		{"54 GiB above the reserve holds six", 154 * gib, 0, 6},
		{"two footprints held take two places", 154 * gib, 18 * gib, 4},
		{"all held, none left", 154 * gib, 54 * gib, 0},
		{"more held than room is zero, not negative", 154 * gib, 360 * gib, 0},
		{"under the reserve is zero, not a wrap", 90 * gib, 0, 0},
		{"exactly the reserve is zero", 100 * gib, 0, 0},
		{"one footprint above holds one", 109 * gib, 0, 1},
		{"a byte short of a footprint holds none", 109*gib - 1, 0, 0},
	}
	for _, c := range cases {
		if got := p.Places(c.free, c.held); got != c.want {
			t.Errorf("%s: places = %d, want %d", c.name, got, c.want)
		}
	}
	if (Plan{ReserveBytes: 100 * gib}).Places(500*gib, 0) != 0 {
		t.Error("a plan with no footprint offered places")
	}
}

func TestHeldChargesAnOwnerItsGrowthGapAndNobodyElseAnything(t *testing.T) {
	p := Plan{ReserveBytes: 100 * gib, FootprintBytes: 9 * gib}
	owners := map[string]uint64{"small": gib, "grown": 12 * gib, "exact": 9 * gib, "paper": 80 * gib}
	want := (9 - 1) * gib
	if got := p.Held(owners); got != want {
		t.Fatalf("held = %d GiB, want %d GiB (8 for the small computer's gap, none for a grown or exact one)", got>>30, want>>30)
	}
	if got := p.Held(nil); got != 0 {
		t.Fatalf("no computers hold %d GiB back", got>>30)
	}
}

// Many accounts with no computer (nothing names them here) leave the room as
// the free reading says: they are not an input.
func TestAccountsWithoutAComputerHoldNothing(t *testing.T) {
	p := Plan{ReserveBytes: 100 * gib, FootprintBytes: 2 * gib}
	if got := p.Places(142*gib, p.Held(map[string]uint64{})); got != 21 {
		t.Fatalf("places = %d, want 21 ((142-100)/2)", got)
	}
	if got := p.Places(101*gib, 0); got != 0 {
		t.Fatalf("less than a footprint over the reserve gave %d places, want 0 (closed)", got)
	}
}

// Ten accounts are admitted, then all ten open their computers. Each new
// computer's disk is a fraction of a footprint, and the free reading falls by
// exactly that. The places before and after are the same: opening a computer
// never makes room.
func TestTenNewComputersOpenAndTheRoomDoesNotGrow(t *testing.T) {
	p := Plan{ReserveBytes: 100 * gib, FootprintBytes: 9 * gib}
	free := 190 * gib // ten places
	if got := p.Places(free, p.Held(nil)); got != 10 {
		t.Fatalf("ten places expected before any computer, got %d", got)
	}
	const fresh = 644245094 // 0.6 GiB, a fresh computer's allocated disk
	owners := map[string]uint64{}
	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		owners[id] = fresh
		free -= fresh
		// Each opening takes its place and leaves the rest as they were.
		if got := p.Places(free, p.Held(owners)); got != 10-(i+1) {
			t.Fatalf("after %d computers opened the room is %d places, want %d", i+1, got, 10-(i+1))
		}
	}
	// Each computer grows to its footprint: the free reading falls, the
	// held room falls with it, the places stay 0.
	for id := range owners {
		free -= 9*gib - owners[id]
		owners[id] = 9 * gib
	}
	if got := p.Places(free, p.Held(owners)); got != 0 {
		t.Fatalf("after ten computers grew to a footprint: %d places, want 0", got)
	}
}

func TestOwnersAreReadOnlyWhenFreshAndWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "computer-owners.json")
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	if _, err := ReadOwners(path, now, MaxOwnersAge); err == nil {
		t.Fatal("a missing file read as a reading")
	}
	if err := WriteOwners(path, now.Add(-time.Minute), []Owner{{UserID: "a", DiskBytes: 5}, {UserID: "b"}}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadOwners(path, now, MaxOwnersAge)
	if _, hasA := got["a"]; err != nil || !hasA || got["a"] != 5 || len(got) != 2 {
		t.Fatalf("fresh read = %v, %v", got, err)
	}
	if _, err := ReadOwners(path, now.Add(MaxOwnersAge+time.Minute), MaxOwnersAge); err == nil {
		t.Fatal("a stale file read as a reading")
	}
	if _, err := ReadOwners(path, now.Add(-time.Hour), MaxOwnersAge); err == nil {
		t.Fatal("a file from the future read as a reading")
	}
	if err := os.WriteFile(path, []byte(`{"written_at":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadOwners(path, now, MaxOwnersAge); err == nil {
		t.Fatal("a torn file read as a reading")
	}
	// An empty list written now is a real reading: nobody has a computer.
	if err := WriteOwners(path, now, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadOwners(path, now, MaxOwnersAge); err != nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v", got, err)
	}
	// The write leaves no temporary file beside it.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory holds %d entries after a write, want 1", len(entries))
	}
}

func TestFreeBytesReadsARealDirectory(t *testing.T) {
	n, err := FreeBytes(t.TempDir())
	if err != nil || n == 0 {
		t.Fatalf("free = %d, %v", n, err)
	}
	if _, err := FreeBytes(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("a path that does not exist read as free space")
	}
}

// A computer's disk is its data image and, while it sleeps, its memory
// snapshot. Waking it deletes the snapshot: the free reading rises by the
// snapshot and the computer's disk falls by the same, so the place count is
// unchanged. (Counting the data image alone, a wake would open a place that
// the next hibernate takes back.)
func TestAWakeAndAHibernateDoNotOpenAPlace(t *testing.T) {
	p := Plan{ReserveBytes: 100 * gib, FootprintBytes: 15 * gib}
	const data, snapshot = 3 * gib, 4 * gib
	free := 151 * gib
	hibernated := map[string]uint64{"a": data + snapshot, "b": data + snapshot}
	atSleep := p.Places(free, p.Held(hibernated))
	// Both wake: the snapshots are deleted.
	free += 2 * snapshot
	awake := map[string]uint64{"a": data, "b": data}
	if got := p.Places(free, p.Held(awake)); got != atSleep {
		t.Fatalf("waking changed the places from %d to %d", atSleep, got)
	}
	// Both sleep again.
	free -= 2 * snapshot
	if got := p.Places(free, p.Held(hibernated)); got != atSleep {
		t.Fatalf("hibernating changed the places from %d to %d", atSleep, got)
	}
	// Reporting the data image alone is the bug: the same wake opens a place.
	dataOnly := map[string]uint64{"a": data, "b": data}
	if p.Places(free+2*snapshot, p.Held(dataOnly)) == p.Places(free, p.Held(dataOnly)) {
		t.Fatal("the control did not show the bug: a data-image-only reading is meant to open a place on a wake")
	}
}
