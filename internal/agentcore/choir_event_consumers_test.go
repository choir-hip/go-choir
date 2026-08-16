package agentcore

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// choirEventDurableConsumers are live readers/writers of OG kind choir.event.
// Retiring that kind without migrating these paths breaks reconnect, run
// history, acceptance, and worker recovery. Bus fanout is not a replacement.
// This list is the freeze; it is not permission to delete choir.event.
var choirEventDurableConsumers = []string{
	"internal/store/store.go",
	"internal/store/graph_store.go",
	"internal/agentcore/product_events.go",
	"internal/agentcore/live_ws.go",
	"internal/agentcore/run_acceptance.go",
	"internal/agentcore/run_memory.go",
}

func TestChoirEventDurableConsumersStillExist(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	needles := []string{"AppendEvent", "choir.event", "ogKindEvent", "ListEvents"}
	for _, rel := range choirEventDurableConsumers {
		path := filepath.Join(root, rel)
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", rel, err)
		}
		found := false
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			for _, needle := range needles {
				if strings.Contains(line, needle) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		_ = f.Close()
		if !found {
			t.Fatalf("%s no longer mentions choir.event consumers; migrate before deleting the kind", rel)
		}
	}
}
