package types

import "testing"

func TestTaskStateTerminal(t *testing.T) {
	terminalStates := []RunState{RunCompleted, RunFailed, RunCancelled}
	for _, s := range terminalStates {
		if !s.Terminal() {
			t.Errorf("expected %q to be terminal", s)
		}
	}

	nonTerminalStates := []RunState{RunPending, RunRunning, RunBlocked, RunPassivated}
	for _, s := range nonTerminalStates {
		if s.Terminal() {
			t.Errorf("expected %q to not be terminal", s)
		}
	}
}

func TestTaskStateActive(t *testing.T) {
	activeStates := []RunState{RunPending, RunRunning, RunBlocked}
	for _, s := range activeStates {
		if !s.Active() {
			t.Errorf("expected %q to be active", s)
		}
	}

	inactiveStates := []RunState{RunCompleted, RunFailed, RunCancelled, RunPassivated}
	for _, s := range inactiveStates {
		if s.Active() {
			t.Errorf("expected %q to not be active", s)
		}
	}
}
