package storeschema

// The store floor (event-table design 4.5, deploy 2 review finding A): once
// a binary has stamped an op for choir_events, no binary below the floor
// epoch may run on the store. Ways it can fail, written before the code:
//   - a floor is lowered, by a later write or by a binary below it;
//   - the epoch receipt is rewritten downward by an older binary's Write,
//     so the updater's own check reads the older epoch;
//   - a corrupt floor reads as "no floor" (it must fail closed);
//   - the floor lives somewhere the updater does not look.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFloorIsWrittenOnceAndNeverLowered(t *testing.T) {
	dir := t.TempDir()
	if _, found, err := ReadFloor(filepath.Join(dir, FloorFile)); err != nil || found {
		t.Fatalf("absent floor found=%v err=%v", found, err)
	}
	if err := WriteFloor(dir, 2, "the event-table writer is on"); err != nil {
		t.Fatal(err)
	}
	if err := WriteFloor(dir, 1, "a lower floor"); err != nil {
		t.Fatal(err)
	}
	floor, found, err := ReadFloor(filepath.Join(dir, FloorFile))
	if err != nil || !found || floor.Version != 2 || floor.Schema != FloorName || !strings.Contains(floor.Reason, "event-table") {
		t.Fatalf("floor = %+v found=%v err=%v; want 2, kept", floor, found, err)
	}
	if err := WriteFloor(dir, 3, "a later floor"); err != nil {
		t.Fatal(err)
	}
	if floor, _, _ := ReadFloor(filepath.Join(dir, FloorFile)); floor.Version != 3 {
		t.Fatalf("floor = %d, want raised to 3", floor.Version)
	}
	if got := FloorPath(filepath.Join(dir, File)); got != filepath.Join(dir, FloorFile) {
		t.Fatalf("FloorPath = %s; the updater must find the floor beside the receipt", got)
	}
}

func TestCorruptFloorFailsClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FloorFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFloor(filepath.Join(dir, FloorFile)); err == nil {
		t.Fatal("a corrupt floor decoded without error")
	}
	if err := CheckFloor(dir); err == nil {
		t.Fatal("a corrupt floor passed CheckFloor")
	}
	if err := WriteFloor(dir, 2, "x"); err == nil {
		t.Fatal("WriteFloor replaced a corrupt floor it could not read")
	}
}

func TestCheckFloorRefusesAFloorAboveThisEpoch(t *testing.T) {
	dir := t.TempDir()
	if err := CheckFloor(dir); err != nil {
		t.Fatalf("no floor: %v", err)
	}
	if err := WriteFloor(dir, Version, "at this epoch"); err != nil {
		t.Fatal(err)
	}
	if err := CheckFloor(dir); err != nil {
		t.Fatalf("a floor at this epoch: %v", err)
	}
	if err := WriteFloor(dir, Version+1, "a later binary stamped"); err != nil {
		t.Fatal(err)
	}
	if err := CheckFloor(dir); err == nil || !strings.Contains(err.Error(), "floor") {
		t.Fatalf("a floor above this epoch: err=%v, want a refusal naming the floor", err)
	}
}

// Review finding A, second question: a binary never rewrites a newer epoch
// receipt downward. (An older binary that predates this rule still does;
// the floor, which no older binary touches, is the fence for that case.)
func TestWriteNeverLowersANewerReceipt(t *testing.T) {
	dir := t.TempDir()
	newer := `{"schema":"` + Name + `","version":` + strconv.FormatUint(Version+1, 10) + `}`
	if err := os.WriteFile(filepath.Join(dir, File), []byte(newer), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	receipt, _, err := Read(filepath.Join(dir, File))
	if err != nil || receipt.Version != Version+1 {
		t.Fatalf("receipt = %+v err=%v; a newer receipt was rewritten down to %d", receipt, err, Version)
	}
	// An older or absent receipt is raised to this epoch, as before.
	if err := os.WriteFile(filepath.Join(dir, File), []byte(`{"schema":"`+Name+`","version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir); err != nil {
		t.Fatal(err)
	}
	if receipt, _, _ := Read(filepath.Join(dir, File)); receipt.Version != Version {
		t.Fatalf("receipt = %d, want raised to %d", receipt.Version, Version)
	}
}
