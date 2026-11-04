package types

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A self-development round's receipt: the minutes per phase and the share
// that was the platform's, derived from records that already exist
// (docs/problems/selfdev-round-overhead-unmeasured-2026-11-04.md).
//
//	Texture's Cast ........ management's Cast to engineering   texture_to_cast
//	  ... bind (the capsule is bound to the assignment)        cast_to_bind (spawn, admission waits, latency)
//	  ... Freeze (the engineer's candidate)                    exploration, build, shot, other
//	  ... decision (judge, owner or consensus)                 freeze_to_decision
//	  ... apply, or the reject / fail / abandon                decision_to_end
//
// Platform overhead = (spawn + waits + Freeze-to-decision + cast/bind
// latency) / total. The waits are the tally by kind (WaitTally); the cast/bind
// latency is cast-to-bind less its spawn and its admission waits.

// Job kinds of a round's jobs.
const (
	RoundJobBuild = "build"
	RoundJobShot  = "shot"
	RoundJobOther = "other"
)

// RoundJobKind classifies a job by its argv: a choir-shot run is "shot",
// a delegate wait or a plain sleep is "other", anything else a build.
func RoundJobKind(argv []string) string {
	joined := strings.Join(argv, " ")
	switch {
	case strings.Contains(joined, "choir-shot"):
		return RoundJobShot
	case strings.Contains(joined, "choir-delegate"):
		return RoundJobOther
	}
	return RoundJobBuild
}

// RoundJob is a job's run: when it started and when it ended.
type RoundJob struct {
	Kind       string
	Start, End time.Time
}

// RoundInputs is what a round's receipt is derived from. A zero time is
// unknown; its phases count as zero.
type RoundInputs struct {
	Outcome string
	// TextureCastAt is the Texture Cast management serves; CastAt is
	// management's Cast to engineering; BindAt the bind; FreezeAt the
	// Freeze (zero: the assignment's end, EngineerEndAt); DecisionAt the
	// decision; EndAt the round's end.
	TextureCastAt, CastAt, BindAt, FreezeAt, EngineerEndAt, DecisionAt, EndAt time.Time
	SpawnS                                                                    int64
	// WaitedS is the minutes-waited tally by kind, in seconds;
	// AdmissionWaitedS and JobWaitedS split it into the waits before the
	// bind and the waits of build jobs.
	WaitedS                      map[string]int64
	AdmissionWaitedS, JobWaitedS int64
	Jobs                         []RoundJob
}

// RoundReceipt is a round's phases, in seconds.
type RoundReceipt struct {
	Outcome           string           `json:"outcome"`
	TextureToCastS    int64            `json:"texture_to_cast_s"`
	CastToBindS       int64            `json:"cast_to_bind_s"`
	SpawnS            int64            `json:"spawn_s"`
	AdmissionWaitedS  int64            `json:"admission_waited_s"`
	CastBindLatencyS  int64            `json:"cast_bind_latency_s"`
	ExplorationS      int64            `json:"exploration_s"`
	BuildS            int64            `json:"build_s"`
	ShotS             int64            `json:"shot_s"`
	OtherEngineerS    int64            `json:"other_engineer_s"`
	JobWaitedS        int64            `json:"job_waited_s"`
	FreezeToDecisionS int64            `json:"freeze_to_decision_s"`
	DecisionToEndS    int64            `json:"decision_to_end_s"`
	WaitedS           map[string]int64 `json:"waited_s,omitempty"`
	TotalS            int64            `json:"total_s"`
	EngineerS         int64            `json:"engineer_s"`
	PlatformS         int64            `json:"platform_s"`
	PlatformPct       int              `json:"platform_pct"`
}

func spanS(from, to time.Time) int64 {
	if from.IsZero() || to.IsZero() || !to.After(from) {
		return 0
	}
	return int64(to.Sub(from) / time.Second)
}

// BuildRoundReceipt derives the receipt.
func BuildRoundReceipt(in RoundInputs) RoundReceipt {
	r := RoundReceipt{Outcome: in.Outcome, SpawnS: max(in.SpawnS, 0), WaitedS: map[string]int64{}}
	var waited int64
	for k, v := range in.WaitedS {
		if v > 0 {
			r.WaitedS[k] = v
			waited += v
		}
	}
	if len(r.WaitedS) == 0 {
		r.WaitedS = nil
	}
	start := in.TextureCastAt
	if start.IsZero() {
		start = in.CastAt
	}
	r.TextureToCastS = spanS(in.TextureCastAt, in.CastAt)
	// The engineer's window: bind to Freeze, or to the assignment's end.
	engineerEnd := in.FreezeAt
	if engineerEnd.IsZero() || !engineerEnd.After(in.BindAt) {
		engineerEnd = in.EngineerEndAt
	}
	if in.BindAt.IsZero() {
		engineerEnd = in.BindAt
	}
	r.CastToBindS = spanS(in.CastAt, in.BindAt)
	r.AdmissionWaitedS = max(in.AdmissionWaitedS, 0)
	r.CastBindLatencyS = max(r.CastToBindS-r.SpawnS-r.AdmissionWaitedS, 0)

	window := spanS(in.BindAt, engineerEnd)
	var firstJob time.Time
	for _, j := range in.Jobs {
		s, e := j.Start, j.End
		if s.IsZero() || e.IsZero() {
			continue
		}
		if s.Before(in.BindAt) {
			s = in.BindAt
		}
		if e.After(engineerEnd) {
			e = engineerEnd
		}
		d := spanS(s, e)
		switch j.Kind {
		case RoundJobBuild:
			r.BuildS += d
		case RoundJobShot:
			r.ShotS += d
		}
		if (j.Kind == RoundJobBuild || j.Kind == RoundJobShot) && d > 0 && (firstJob.IsZero() || s.Before(firstJob)) {
			firstJob = s
		}
	}
	jobs := min(r.BuildS+r.ShotS, window)
	explore := window - jobs
	if !firstJob.IsZero() {
		explore = min(spanS(in.BindAt, firstJob), window-jobs)
	}
	other := window - jobs - explore
	// A job's wait ends before the job starts, inside the engineer's window:
	// it is counted as a wait, not as engineer time.
	r.JobWaitedS = min(max(in.JobWaitedS, 0), window)
	cut := min(r.JobWaitedS, other)
	other -= cut
	explore -= min(r.JobWaitedS-cut, explore)
	r.ExplorationS, r.OtherEngineerS = explore, other
	r.EngineerS = r.ExplorationS + r.BuildS + r.ShotS + r.OtherEngineerS

	freeze := engineerEnd
	if freeze.IsZero() {
		freeze = in.FreezeAt
	}
	decision := in.DecisionAt
	if decision.IsZero() || decision.After(in.EndAt) {
		decision = in.EndAt
	}
	r.FreezeToDecisionS = spanS(freeze, decision)
	r.DecisionToEndS = spanS(decision, in.EndAt)
	r.TotalS = spanS(start, in.EndAt)
	r.PlatformS = r.SpawnS + waited + r.FreezeToDecisionS + r.CastBindLatencyS
	if r.TotalS > 0 {
		r.PlatformS = min(r.PlatformS, r.TotalS)
		r.PlatformPct = int((r.PlatformS*100 + r.TotalS/2) / r.TotalS)
	}
	return r
}

func wholeMinutes(s int64) int64 {
	if s <= 0 {
		return 0
	}
	return max((s+30)/60, 1)
}

// Line is Texture's one line: "round took 12 min: 9 engineer, 2 platform (17%)".
func (r RoundReceipt) Line() string {
	return fmt.Sprintf("round took %d min: %d engineer, %d platform (%d%%)",
		wholeMinutes(r.TotalS), wholeMinutes(r.EngineerS), wholeMinutes(r.PlatformS), r.PlatformPct)
}

// Summary is the receipt in words, for management's acting pack: the line,
// then every phase.
func (r RoundReceipt) Summary() string {
	phases := []struct {
		name string
		s    int64
	}{
		{"Texture to cast", r.TextureToCastS}, {"cast to bind", r.CastToBindS}, {"capsule spawn", r.SpawnS},
		{"exploration before the first build", r.ExplorationS}, {"build jobs", r.BuildS}, {"choir-shot jobs", r.ShotS},
		{"other engineer time", r.OtherEngineerS}, {"Freeze to decision", r.FreezeToDecisionS}, {"decision to end", r.DecisionToEndS},
	}
	parts := make([]string, 0, len(phases)+2)
	for _, p := range phases {
		parts = append(parts, p.name+" "+minutesText(p.s))
	}
	out := r.Line() + " (" + r.Outcome + "). Phases: " + strings.Join(parts, ", ") + "."
	if t := (WaitTally{Seconds: r.WaitedS}); t.Total() > 0 {
		out += " " + strings.ToUpper(t.Summary()[:1]) + t.Summary()[1:] + "."
	}
	out += fmt.Sprintf(" Platform: spawn, waits, Freeze to decision and cast/bind latency (%s) of %s.", minutesText(r.PlatformS), minutesText(r.TotalS))
	return out
}

// Refs is the receipt's exact fields as a record's subject refs.
func (r RoundReceipt) Refs() map[string]string {
	refs := map[string]string{"outcome": r.Outcome, "platform_pct": strconv.Itoa(r.PlatformPct)}
	for k, v := range map[string]int64{
		"texture_to_cast_s": r.TextureToCastS, "cast_to_bind_s": r.CastToBindS, "spawn_s": r.SpawnS,
		"admission_waited_s": r.AdmissionWaitedS, "cast_bind_latency_s": r.CastBindLatencyS,
		"exploration_s": r.ExplorationS, "build_s": r.BuildS, "shot_s": r.ShotS, "other_engineer_s": r.OtherEngineerS,
		"job_waited_s": r.JobWaitedS, "freeze_to_decision_s": r.FreezeToDecisionS, "decision_to_end_s": r.DecisionToEndS,
		"total_s": r.TotalS, "engineer_s": r.EngineerS, "platform_s": r.PlatformS,
	} {
		refs[k] = strconv.FormatInt(v, 10)
	}
	kinds := make([]string, 0, len(r.WaitedS))
	for k := range r.WaitedS {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		refs["waited_s:"+k] = strconv.FormatInt(r.WaitedS[k], 10)
	}
	return refs
}

// Subject-ref keys the wait-summary record carries beyond the tally.
const (
	RoundRefSpawnS     = "spawn_s"
	RoundRefJobWaitedS = "job_waited_s"
)

// RoundWaitedFromRefs reads the tally by kind from subject refs.
func RoundWaitedFromRefs(refs map[string]string) map[string]int64 {
	out := map[string]int64{}
	for k, v := range refs {
		if kind, ok := strings.CutPrefix(k, "waited_s:"); ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				out[kind] = n
			}
		}
	}
	return out
}

// RoundRefInt reads one integer ref; 0 when absent.
func RoundRefInt(refs map[string]string, key string) int64 {
	n, _ := strconv.ParseInt(refs[key], 10, 64)
	return n
}

// RoundReceiptRecordID is the one receipt of a self-development operation.
func RoundReceiptRecordID(operationID string) string {
	return "selfdev-round:" + strings.TrimSpace(operationID)
}

// RoundLineFromRefs is Texture's one line from a receipt record's subject
// refs; "" when they carry no total.
func RoundLineFromRefs(refs map[string]string) string {
	total := RoundRefInt(refs, "total_s")
	if total <= 0 {
		return ""
	}
	return RoundReceipt{
		TotalS: total, EngineerS: RoundRefInt(refs, "engineer_s"), PlatformS: RoundRefInt(refs, "platform_s"),
		PlatformPct: int(RoundRefInt(refs, "platform_pct")),
	}.Line()
}
