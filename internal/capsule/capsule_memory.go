package capsule

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Capsule memory per doctrine (SDC RES-04; docs/choir-doctrine.md:406-408,
// docs/runtime-invariants.md:139-141): memory.high is the request,
// memory.max twice that, admission counts the request, memory.events is
// feedback. Before RES-04 the executor admitted and capped at MemoryMax and
// read no events.
//
// The values cannot simply follow the formula: an engineering capsule's link
// peaks at 1.50 to 1.80 GiB anonymous with no swap, so a memory.high below
// that throttles the build for minutes. So the shape is a policy. Observe
// (the default) keeps the old shape and writes a memory receipt at destroy
// that says what doctrine would have done; doctrine applies the formula to
// specs that declare a request. Designed with its failure modes (Q1-Q10) in
// docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md, "Second
// batch".

// CapsuleMemoryPolicyEnv selects the capsule memory shape: "observe" (also
// unset and any unknown value) or "doctrine".
const CapsuleMemoryPolicyEnv = "CHOIR_CAPSULE_MEMORY_POLICY"

// CapsuleMemoryPolicy is the capsule memory shape.
type CapsuleMemoryPolicy string

const (
	// CapsuleMemoryObserve: memory.max = MemoryMax, admission against it, no
	// memory.high; a memory receipt at destroy.
	CapsuleMemoryObserve CapsuleMemoryPolicy = "observe"
	// CapsuleMemoryDoctrine: for a spec with MemoryRequest, memory.high =
	// request, memory.max = 2 x request, admission against the request, and
	// the capsules aggregate capped at the capsule budget.
	CapsuleMemoryDoctrine CapsuleMemoryPolicy = "doctrine"
)

// CapsuleMemoryPolicyFromEnv reads the policy. A value it does not know is
// observe, the narrower shape (Q8).
func CapsuleMemoryPolicyFromEnv(getenv func(string) string) CapsuleMemoryPolicy {
	if getenv == nil {
		getenv = os.Getenv
	}
	if CapsuleMemoryPolicy(strings.ToLower(strings.TrimSpace(getenv(CapsuleMemoryPolicyEnv)))) == CapsuleMemoryDoctrine {
		return CapsuleMemoryDoctrine
	}
	return CapsuleMemoryObserve
}

// capsuleMemoryPlan is what a spawn admits and writes. High 0 leaves
// memory.high unset ("max").
type capsuleMemoryPlan struct {
	Admit int64
	High  int64
	Max   int64
}

// planCapsuleMemory resolves a spec under a policy. A spec with no request
// keeps the old shape under either policy (Q3).
func planCapsuleMemory(policy CapsuleMemoryPolicy, spec SpawnSpec) (capsuleMemoryPlan, error) {
	if spec.MemoryMax <= 0 {
		return capsuleMemoryPlan{}, fmt.Errorf("capsule spawn requires a positive memory limit")
	}
	if spec.MemoryRequest < 0 {
		return capsuleMemoryPlan{}, fmt.Errorf("capsule memory request %d is negative", spec.MemoryRequest)
	}
	if policy != CapsuleMemoryDoctrine || spec.MemoryRequest == 0 {
		return capsuleMemoryPlan{Admit: spec.MemoryMax, Max: spec.MemoryMax}, nil
	}
	if spec.MemoryRequest > math.MaxInt64/2 {
		return capsuleMemoryPlan{}, fmt.Errorf("capsule memory request %d overflows twice itself", spec.MemoryRequest)
	}
	r := spec.MemoryRequest
	return capsuleMemoryPlan{Admit: r, High: r, Max: 2 * r}, nil
}

// cgroupMemoryHigh is a capsule cgroup that can set memory.high.
type cgroupMemoryHigh interface {
	SetHigh(bytes int64) error
}

func (d cgroupMemoryDir) SetHigh(bytes int64) error {
	return writeCgroupFile(filepath.Join(string(d), "memory.high"), strconv.FormatInt(bytes, 10))
}

// cgroupMemoryFeedback is a capsule cgroup's memory.events and memory.peak.
type cgroupMemoryFeedback interface {
	MemoryEvents() (string, error)
	MemoryPeak() (int64, error)
}

func (d cgroupMemoryDir) MemoryEvents() (string, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "memory.events"))
	return string(raw), err
}

func (d cgroupMemoryDir) MemoryPeak() (int64, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "memory.peak"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
}

// CapsuleMemoryReceipt is a capsule's memory at its end: what was asked and
// set, how high it went, and what the kernel did at its limits. -1 is a value
// that could not be read (Q9).
type CapsuleMemoryReceipt struct {
	CapsuleID     string              `json:"capsule_id,omitempty"`
	Policy        CapsuleMemoryPolicy `json:"policy"`
	Request       int64               `json:"request_bytes"`
	Admitted      int64               `json:"admitted_bytes"`
	High          int64               `json:"high_bytes"` // 0: memory.high was not set
	Max           int64               `json:"max_bytes"`
	Peak          int64               `json:"peak_bytes"`
	EventsHigh    int64               `json:"events_high"` // times the cgroup was throttled at memory.high
	EventsMax     int64               `json:"events_max"`  // times it reached memory.max
	EventsOOM     int64               `json:"events_oom"`
	EventsOOMKill int64               `json:"events_oom_kill"`
	// DoctrineWouldThrottle: the peak passed the request, so doctrine's
	// memory.high would have throttled this capsule. Under doctrine it is
	// what happened (EventsHigh says how often).
	DoctrineWouldThrottle bool `json:"doctrine_would_throttle"`
	// Pids and CPU next to memory (docs/problems/engineering-cell-never-returns-2026-11-03.md):
	// PidsMax is the capsule's pids.max (-1 unknown); PidsPeak its
	// pids.peak; PidsEventsMax how often a fork or a thread creation was
	// refused at pids.max (cgroup v2 counts threads). CPUNrThrottled and
	// CPUThrottledUsec are cpu.stat's throttled periods and time.
	PidsMax          int64 `json:"pids_max"`
	PidsPeak         int64 `json:"pids_peak"`
	PidsEventsMax    int64 `json:"pids_events_max"`
	CPUNrThrottled   int64 `json:"cpu_nr_throttled"`
	CPUThrottledUsec int64 `json:"cpu_throttled_usec"`
	// BrokerExit is the broker's exit status and time when it died before
	// the capsule's destroy; empty when it ran until then.
	BrokerExit string `json:"broker_exit,omitempty"`
}

// cgroupResourceFeedback is a capsule cgroup's pids.events, pids.peak and
// cpu.stat.
type cgroupResourceFeedback interface {
	PidsEvents() (string, error)
	PidsPeak() (int64, error)
	CPUStat() (string, error)
}

func (d cgroupMemoryDir) PidsEvents() (string, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "pids.events"))
	return string(raw), err
}

func (d cgroupMemoryDir) PidsPeak() (int64, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "pids.peak"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
}

func (d cgroupMemoryDir) CPUStat() (string, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "cpu.stat"))
	return string(raw), err
}

// flatKeyed reads a cgroup flat-keyed file ("key value" lines) into the
// named fields; a field whose key is absent keeps its value.
func flatKeyed(text string, fields map[string]*int64) {
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		if dst, ok := fields[f[0]]; ok {
			if n, err := strconv.ParseInt(f[1], 10, 64); err == nil {
				*dst = n
			}
		}
	}
}

// readCapsuleResources fills the receipt's pids and CPU fields; what it
// cannot read is -1.
func readCapsuleResources(r *CapsuleMemoryReceipt, cg any) {
	r.PidsMax, r.PidsPeak, r.PidsEventsMax, r.CPUNrThrottled, r.CPUThrottledUsec = -1, -1, -1, -1, -1
	fb, ok := cg.(cgroupResourceFeedback)
	if !ok || fb == nil {
		return
	}
	if peak, err := fb.PidsPeak(); err == nil {
		r.PidsPeak = peak
	}
	if events, err := fb.PidsEvents(); err == nil {
		r.PidsEventsMax = 0
		flatKeyed(events, map[string]*int64{"max": &r.PidsEventsMax})
	}
	if stat, err := fb.CPUStat(); err == nil {
		r.CPUNrThrottled, r.CPUThrottledUsec = 0, 0
		flatKeyed(stat, map[string]*int64{"nr_throttled": &r.CPUNrThrottled, "throttled_usec": &r.CPUThrottledUsec})
	}
}

// readCapsuleMemoryReceipt reads the receipt from a capsule cgroup that is
// empty and not yet removed. It never fails: what it cannot read is -1.
func readCapsuleMemoryReceipt(policy CapsuleMemoryPolicy, plan capsuleMemoryPlan, request int64, cg any) CapsuleMemoryReceipt {
	r := CapsuleMemoryReceipt{Policy: policy, Request: request, Admitted: plan.Admit, High: plan.High, Max: plan.Max,
		Peak: -1, EventsHigh: -1, EventsMax: -1, EventsOOM: -1, EventsOOMKill: -1}
	readCapsuleResources(&r, cg)
	fb, ok := cg.(cgroupMemoryFeedback)
	if !ok || fb == nil {
		return r
	}
	if peak, err := fb.MemoryPeak(); err == nil {
		r.Peak = peak
	}
	if events, err := fb.MemoryEvents(); err == nil {
		r.EventsHigh, r.EventsMax, r.EventsOOM, r.EventsOOMKill = 0, 0, 0, 0
		for _, line := range strings.Split(events, "\n") {
			f := strings.Fields(line)
			if len(f) != 2 {
				continue
			}
			n, err := strconv.ParseInt(f[1], 10, 64)
			if err != nil {
				continue
			}
			switch f[0] {
			case "high":
				r.EventsHigh = n
			case "max":
				r.EventsMax = n
			case "oom":
				r.EventsOOM = n
			case "oom_kill":
				r.EventsOOMKill = n
			}
		}
	}
	r.DoctrineWouldThrottle = request > 0 && r.Peak > request
	return r
}

func receiptMiB(v int64) string {
	switch {
	case v < 0:
		return "unknown"
	case v == 0:
		return "-"
	}
	return strconv.FormatInt(v>>20, 10) + "MiB"
}

func receiptCount(v int64) string {
	if v < 0 {
		return "unknown"
	}
	return strconv.FormatInt(v, 10)
}

// String is the receipt's log line.
func (r CapsuleMemoryReceipt) String() string {
	pidsMax := r.PidsMax
	if pidsMax <= 0 {
		pidsMax = -1
	}
	line := fmt.Sprintf("policy=%s request=%s admitted=%s high=%s max=%s peak=%s events high=%d max=%d oom=%d oom_kill=%d doctrine_would_throttle=%t"+
		" pids max=%s peak=%s events max=%s cpu nr_throttled=%s throttled_usec=%s",
		r.Policy, receiptMiB(r.Request), receiptMiB(r.Admitted), receiptMiB(r.High), receiptMiB(r.Max), receiptMiB(r.Peak),
		r.EventsHigh, r.EventsMax, r.EventsOOM, r.EventsOOMKill, r.DoctrineWouldThrottle,
		receiptCount(pidsMax), receiptCount(r.PidsPeak), receiptCount(r.PidsEventsMax), receiptCount(r.CPUNrThrottled), receiptCount(r.CPUThrottledUsec))
	if r.BrokerExit != "" {
		line += fmt.Sprintf(" broker_exit=%q", r.BrokerExit)
	}
	return line
}
