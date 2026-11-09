package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func HasUnsettledApply(root string) bool {
	_, unsettled := UnsettledApply(root)
	return unsettled
}

func TestHasUnsettledApplyReadsTheJournals(t *testing.T) {
	root := t.TempDir()
	if name, ok := UnsettledApply(root); ok || name != "" {
		t.Fatal("no operations dir is settled")
	}
	if HasUnsettledApply(root) {
		t.Fatal("no operations dir is not unsettled")
	}
	ops := operationsDir(root)
	if err := os.MkdirAll(ops, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeKeyLockMarker(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ops, "done.json"), []byte(`{"phase":"completed","result":{"outcome":"applied"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if HasUnsettledApply(root) {
		t.Fatal("a journal with a result is settled")
	}
	if err := os.WriteFile(filepath.Join(ops, "open.json"), []byte(`{"phase":"pointer_swapped","result":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !HasUnsettledApply(root) {
		t.Fatal("a journal with no result is unsettled")
	}
	if err := os.WriteFile(filepath.Join(ops, "open.json"), []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !HasUnsettledApply(root) {
		t.Fatal("an unreadable journal is unsettled")
	}
}
