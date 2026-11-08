package yaegikernel

import "testing"

func goodReceipt() DeskCellSelfTestReceipt {
	return DeskCellSelfTestReceipt{
		Mode:    string(CellConfinementRequired),
		Workers: []DeskCellSelfTestWorker{{Role: "research", Profile: "gvisor"}},
		Gvisor:  &GvisorStatus{Available: true},
		OK:      true,
	}
}

func TestDeskSandboxReadyJudgement(t *testing.T) {
	if got := JudgeDeskSandboxReady(goodReceipt()); len(got) != 0 {
		t.Fatalf("good receipt refused: %v", got)
	}
	cases := map[string]func(*DeskCellSelfTestReceipt){
		"auto mode":       func(r *DeskCellSelfTestReceipt) { r.Mode = string(CellConfinementAuto) },
		"failed selftest": func(r *DeskCellSelfTestReceipt) { r.OK = false; r.Failures = []string{"x"} },
		"no gvisor":       func(r *DeskCellSelfTestReceipt) { r.Gvisor = &GvisorStatus{Available: false, Reason: "no runsc"} },
		"nil gvisor":      func(r *DeskCellSelfTestReceipt) { r.Gvisor = nil },
		"process worker":  func(r *DeskCellSelfTestReceipt) { r.Workers[0].Profile = "process" },
		"no workers":      func(r *DeskCellSelfTestReceipt) { r.Workers = nil },
	}
	for name, mut := range cases {
		r := goodReceipt()
		mut(&r)
		if got := JudgeDeskSandboxReady(r); len(got) == 0 {
			t.Errorf("%s: judged ready", name)
		}
	}
}

func TestDeskSandboxStateFollowsLastSelfTest(t *testing.T) {
	t.Cleanup(ResetDeskSandboxState)
	ResetDeskSandboxState()
	if st := DeskSandboxStatusNow(); st.Ready || st.Reason == "" {
		t.Fatalf("before any selftest: %+v", st)
	}
	if st := RecordDeskSandboxSelfTest(goodReceipt(), "boot"); !st.Ready || !DeskSandboxReady() {
		t.Fatalf("passed selftest: %+v", st)
	}
	bad := goodReceipt()
	bad.Workers[0].Profile = "process"
	if st := RecordDeskSandboxSelfTest(bad, "api"); st.Ready || DeskSandboxReady() {
		t.Fatalf("a later failing selftest must close it: %+v", st)
	}
}

func TestDeskSandboxBootSelfTestNotRequiredSpawnsNothing(t *testing.T) {
	t.Cleanup(ResetDeskSandboxState)
	st := RunDeskSandboxBootSelfTest(t.Context(), DeskCellSelfTestConfig{})
	if st.Ready {
		t.Fatalf("auto mode must not be ready: %+v", st)
	}
}
