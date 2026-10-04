package builder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// nowRFC3339 returns the current UTC time in RFC3339.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// WriteReceipt serializes the closure result as a canonical builder evidence
// receipt (JSON) at path, atomically.
func (r *ClosureResult) WriteReceipt(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'), 0o444)
}
