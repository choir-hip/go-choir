package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A sender spelled management_task is the same sender as management-task.
func TestInboxClassAcceptsEitherTaskSpelling(t *testing.T) {
	c := InboxClass{Senders: []string{agentprofile.Management, types.TaskInboxDesk}}
	for _, from := range []string{"management", "management-task", "management_task"} {
		if !c.accepts(from) {
			t.Errorf("accepts(%q) = false", from)
		}
	}
	if c.accepts("texture") {
		t.Error("accepts texture")
	}
	only := InboxClass{Senders: []string{agentprofile.Management}}
	if only.accepts("management_task") {
		t.Error("a class that does not name the task accepted it")
	}
}
