package shotsandbox

import (
	"strings"
	"testing"
)

// apply-proof-4 r2 on d@e (selfdev-70be92c5, 2026-11-07): Chromium puts its
// profile under TMPDIR, the stage's work directory, and its SingletonSocket
// path there exceeded the 107-byte unix socket limit, so the browser exited
// FATAL before the first shot. A work directory too long for that socket is
// refused by name before anything starts.
func TestCheckWorkDirRefusesADirTooLongForChromiumsSocket(t *testing.T) {
	long := "/mnt/persistent/choir-shot/selfdev-70be92c52c642159ebd9b45721f9441e/before/work"
	err := CheckWorkDirLength(long)
	if err == nil || !strings.Contains(err.Error(), "socket") {
		t.Fatalf("a %d-byte work dir: err = %v", len(long), err)
	}
	short := "/mnt/persistent/choir-shot/0123456789abcdef/before/work"
	if err := CheckWorkDirLength(short); err != nil {
		t.Fatalf("a %d-byte work dir: %v", len(short), err)
	}
}
