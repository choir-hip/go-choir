package types

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// WaitState is the one typed record of a self-development wait
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md). Every
// wait point that stays, contention or policy, produces one, and the same
// record reaches engineering (a job's view, the Start and Poll results),
// management (its cell frame) and Texture (a progress Note), so each can
// tell "blocked on host memory" from "stuck" and decide what to do.
type WaitState struct {
	Kind  WaitKind  `json:"kind"`
	Class WaitClass `json:"class"`
	// Subject is what waits: "job j-…", "assignment a-…".
	Subject string `json:"subject,omitempty"`
	// Resource is what it waits for, in words: "host memory", "the
	// engineering slot", "the platform update to <sha>".
	Resource string `json:"resource"`
	// Need and Have are the wait's figures in words, when not bytes.
	Need string `json:"need,omitempty"`
	Have string `json:"have,omitempty"`
	// NeedBytes and HaveBytes are a memory wait's figures; HasHave says
	// HaveBytes was measured (0 free is a fact, not an absence).
	NeedBytes int64        `json:"need_bytes,omitempty"`
	HaveBytes int64        `json:"have_bytes,omitempty"`
	HasHave   bool         `json:"has_have,omitempty"`
	Holders   []WaitHolder `json:"holders,omitempty"`
	// Since is when this wait began; Until is when it gives up (its bound),
	// zero when only an outer deadline bounds it.
	Since time.Time `json:"since"`
	Until time.Time `json:"until,omitempty"`
	// ExpectedUntil is when the resource is expected back (a provider
	// quota's reset time), zero when unknown; ExpectedEstimated says it is
	// an estimate (the gateway's cooldown) rather than the holder's word.
	ExpectedUntil     time.Time `json:"expected_until,omitempty"`
	ExpectedEstimated bool      `json:"expected_estimated,omitempty"`
	// Provider and Model name an external resource's holder: the model
	// provider whose quota a provider_quota wait waits on.
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	// CheckEverySec is how often the wait is re-checked; 0 when it is
	// woken instead.
	CheckEverySec int `json:"check_every_s,omitempty"`
	// Detail is the wait point's own text (the host's reason, the
	// operation's settling acts).
	Detail string `json:"detail,omitempty"`
	// Sentence is the record rendered at StampedAt (Stamp), for readers
	// that do not render it themselves.
	Sentence  string    `json:"sentence,omitempty"`
	StampedAt time.Time `json:"stamped_at,omitempty"`
}

// WaitHolder is one holder of the resource a wait needs.
type WaitHolder struct {
	Who   string    `json:"who"`
	What  string    `json:"what,omitempty"`
	Bytes int64     `json:"bytes,omitempty"`
	Since time.Time `json:"since,omitempty"`
}

// WaitClass separates true resource contention from a policy wait.
type WaitClass string

const (
	WaitClassContention WaitClass = "contention"
	WaitClassPolicy     WaitClass = "policy"
)

// WaitKind names the cause of a wait; the tally sums time by it.
type WaitKind string

const (
	WaitKindHostMemory       WaitKind = "host_memory"
	WaitKindEngineeringSlot  WaitKind = "engineering_slot"
	WaitKindQueuedAhead      WaitKind = "queued_ahead"
	WaitKindCandidatePending WaitKind = "candidate_pending"
	WaitKindSelfdevOperation WaitKind = "selfdev_operation"
	WaitKindPlatformUpdate   WaitKind = "platform_update"
	WaitKindSourceSettling   WaitKind = "source_settling"
	WaitKindReadError        WaitKind = "read_error"
	// WaitKindNoNextActor is an owner request nothing works on: the Texture
	// desk left it open with no Cast, Wait or Done, and no round,
	// assignment or operation serves it, so it waits on the owner's next
	// revise (docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md).
	WaitKindNoNextActor WaitKind = "no_next_actor"
	// WaitKindProviderQuota is a run whose every model is out of quota at
	// once (a quota otherwise switches models) (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md).
	WaitKindProviderQuota WaitKind = "provider_quota"
	// WaitKindCandidateDecision is a frozen candidate waiting for its
	// decision (management's Verify, then the gate) after its build ended,
	// on a budget of its own
	// (docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md).
	WaitKindCandidateDecision WaitKind = "candidate_decision"
	// WaitKindVerifyOwed is a frozen candidate whose assignment is ready
	// for management's Verify (completed, or its engineer asked for review)
	// and has none yet
	// (docs/problems/management-ends-without-verify-2026-11-04.md).
	WaitKindVerifyOwed WaitKind = "verify_owed"
	// WaitKindShotsOwed is an accepted platform candidate whose host build
	// is done and whose shots management has not judged
	// (docs/design/desk-shots-and-review-2026-11-07.md 3.4).
	WaitKindShotsOwed WaitKind = "shots_owed"
	// WaitKindLock is a wait for an in-process lock another step holds
	// (internal/holdlock): the holder is named, and the wait gives up at
	// its bound.
	WaitKindLock WaitKind = "lock"
)

// Label is the kind in words.
func (k WaitKind) Label() string {
	switch k {
	case WaitKindHostMemory:
		return "host memory"
	case WaitKindEngineeringSlot:
		return "engineering slot"
	case WaitKindQueuedAhead:
		return "queue"
	case WaitKindCandidatePending:
		return "pending candidate"
	case WaitKindSelfdevOperation:
		return "open self-development operation"
	case WaitKindPlatformUpdate:
		return "platform update"
	case WaitKindSourceSettling:
		return "source settling"
	case WaitKindReadError:
		return "unreadable state"
	case WaitKindNoNextActor:
		return "a next actor"
	case WaitKindProviderQuota:
		return "provider quota"
	case WaitKindCandidateDecision:
		return "a frozen candidate's decision"
	case WaitKindVerifyOwed:
		return "management's Verify"
	case WaitKindShotsOwed:
		return "management's shot verdict"
	case WaitKindLock:
		return "lock"
	}
	return strings.ReplaceAll(string(k), "_", " ")
}

// Active reports whether w is a wait.
func (w *WaitState) Active() bool { return w != nil && w.Kind != "" }

// Stamp fills Sentence as rendered at now.
func (w *WaitState) Stamp(now time.Time) {
	if !w.Active() {
		return
	}
	w.StampedAt = now.UTC()
	w.Sentence = w.SentenceAt(now)
}

// Clone is a deep copy (nil for nil).
func (w *WaitState) Clone() *WaitState {
	if w == nil {
		return nil
	}
	c := *w
	c.Holders = append([]WaitHolder(nil), w.Holders...)
	return &c
}

// SentenceAt renders the wait as one plain sentence: what it waits for,
// the figures, the holders, the cadence, how long so far and its bound.
func (w *WaitState) SentenceAt(now time.Time) string {
	if !w.Active() {
		return ""
	}
	var b strings.Builder
	b.WriteString("waiting for ")
	b.WriteString(firstNonEmptyString(w.Resource, w.Kind.Label()))
	b.WriteString(w.expectedText(now))
	if w.Class == WaitClassPolicy {
		b.WriteString(" (policy)")
	}
	var parts []string
	subject := "this " + firstNonEmptyString(subjectNoun(w.Subject), "work")
	switch {
	case w.NeedBytes > 0:
		parts = append(parts, fmt.Sprintf("%s needs %d MiB", subject, w.NeedBytes>>20))
	case w.Need != "":
		parts = append(parts, subject+" needs "+w.Need)
	}
	switch {
	case w.HasHave && w.Kind == WaitKindHostMemory:
		parts = append(parts, fmt.Sprintf("%d MiB is free beyond the floor", max(w.HaveBytes, 0)>>20))
	case w.HasHave:
		parts = append(parts, fmt.Sprintf("%d MiB is free", max(w.HaveBytes, 0)>>20))
	case w.Have != "":
		parts = append(parts, w.Have)
	}
	if len(w.Holders) > 0 {
		held := make([]string, 0, len(w.Holders))
		for _, h := range w.Holders {
			held = append(held, h.render(now))
		}
		parts = append(parts, "held by "+joinAnd(held))
	}
	if w.CheckEverySec > 0 {
		parts = append(parts, fmt.Sprintf("checking every %ds", w.CheckEverySec))
	}
	if !w.Since.IsZero() {
		parts = append(parts, "waited "+shortDuration(now.Sub(w.Since)))
	}
	if !w.Until.IsZero() {
		if left := w.Until.Sub(now); left > 0 {
			parts = append(parts, "gives up in "+shortDuration(left))
		} else {
			parts = append(parts, "at its bound")
		}
	}
	if len(parts) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(parts, "; "))
	}
	return b.String()
}

// TextureLine is the owner-facing line: "Waiting 4 min for host memory,
// held by luna's build".
func (w *WaitState) TextureLine(now time.Time) string {
	if !w.Active() {
		return ""
	}
	minutes := int(now.Sub(w.Since).Round(time.Minute) / time.Minute)
	if w.Since.IsZero() || minutes < 1 {
		minutes = 1
	}
	line := fmt.Sprintf("Waiting %d min for %s%s", minutes, firstNonEmptyString(w.Resource, w.Kind.Label()), w.expectedText(now))
	if len(w.Holders) > 0 {
		held := make([]string, 0, len(w.Holders))
		for _, h := range w.Holders {
			if h.What != "" {
				held = append(held, h.Who+" ("+h.What+")")
			} else {
				held = append(held, h.Who)
			}
		}
		line += ", held by " + joinAnd(held)
	}
	return line
}

// expectedText is " (about N min)" while the resource is expected back
// later, " (expected about now)" once that time passed, "" when unknown.
func (w *WaitState) expectedText(now time.Time) string {
	if w.ExpectedUntil.IsZero() {
		return ""
	}
	left := w.ExpectedUntil.Sub(now)
	if left <= 0 {
		return " (expected about now)"
	}
	minutes := int((left + time.Minute - 1) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	if minutes >= 120 {
		return fmt.Sprintf(" (about %d h)", (minutes+30)/60)
	}
	return fmt.Sprintf(" (about %d min)", minutes)
}

// ProviderDisplayName is a model provider's name as the owner reads it.
func ProviderDisplayName(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "synthetic":
		return "Synthetic"
	case "chatgpt":
		return "ChatGPT"
	case "openrouter":
		return "OpenRouter"
	case "river":
		return "River"
	case "devin":
		return "Devin"
	case "opencode-go", "opencode":
		return "OpenCode"
	case "zai":
		return "Z.ai"
	case "bedrock":
		return "Bedrock"
	case "":
		return "the provider"
	}
	return strings.TrimSpace(provider)
}

func quotaResource(provider string) string {
	if strings.TrimSpace(provider) == "" {
		return "the provider quota to reset"
	}
	return "the " + ProviderDisplayName(provider) + " quota to reset"
}

// NewProviderQuotaWait is the incident of a run whose every model is out of
// quota: true contention on an external resource, expected back at expected
// (zero when unknown), retried every checkEvery.
func NewProviderQuotaWait(subject, provider, model string, since, expected time.Time, estimated bool, checkEvery time.Duration, detail string) *WaitState {
	w := &WaitState{
		Kind: WaitKindProviderQuota, Class: WaitClassContention, Subject: subject,
		Resource: quotaResource(provider),
		Provider: strings.TrimSpace(provider), Model: strings.TrimSpace(model),
		Since: since.UTC(), ExpectedEstimated: estimated && !expected.IsZero(), Detail: detail,
	}
	if !expected.IsZero() {
		w.ExpectedUntil = expected.UTC()
	}
	if checkEvery > 0 {
		w.CheckEverySec = max(int(checkEvery/time.Second), 1)
	}
	return w
}

func (h WaitHolder) render(now time.Time) string {
	var in []string
	if h.What != "" {
		in = append(in, h.What)
	}
	if h.Bytes > 0 {
		in = append(in, fmt.Sprintf("%d MiB", h.Bytes>>20))
	}
	if !h.Since.IsZero() {
		in = append(in, shortDuration(now.Sub(h.Since)))
	}
	if len(in) == 0 {
		return h.Who
	}
	return h.Who + " (" + strings.Join(in, ", ") + ")"
}

// WaitTally sums time waited by kind: a round's "minutes waited by kind".
type WaitTally struct {
	Seconds map[string]int64 `json:"seconds,omitempty"`
}

// Add counts d against kind; a non-positive d counts nothing.
func (t *WaitTally) Add(kind WaitKind, d time.Duration) {
	if t == nil || kind == "" || d <= 0 {
		return
	}
	if t.Seconds == nil {
		t.Seconds = map[string]int64{}
	}
	t.Seconds[string(kind)] += int64(d.Round(time.Second) / time.Second)
}

// Total is the time waited across kinds.
func (t WaitTally) Total() time.Duration {
	var s int64
	for _, v := range t.Seconds {
		s += v
	}
	return time.Duration(s) * time.Second
}

// Clone is a copy.
func (t WaitTally) Clone() WaitTally {
	if len(t.Seconds) == 0 {
		return WaitTally{}
	}
	c := WaitTally{Seconds: make(map[string]int64, len(t.Seconds))}
	for k, v := range t.Seconds {
		c.Seconds[k] = v
	}
	return c
}

// Minutes is the tally in whole minutes by kind, rounded (for frames and
// records).
func (t WaitTally) Minutes() map[string]int64 {
	if len(t.Seconds) == 0 {
		return nil
	}
	out := make(map[string]int64, len(t.Seconds))
	for k, v := range t.Seconds {
		out[k] = (v + 30) / 60
	}
	return out
}

// Summary renders the tally: "waited 13 min: host memory 10 min, platform
// update 3 min", largest first; "" when nothing was waited.
func (t WaitTally) Summary() string {
	total := t.Total()
	if total <= 0 {
		return ""
	}
	kinds := make([]string, 0, len(t.Seconds))
	for k, v := range t.Seconds {
		if v > 0 {
			kinds = append(kinds, k)
		}
	}
	sort.Slice(kinds, func(i, j int) bool {
		if t.Seconds[kinds[i]] != t.Seconds[kinds[j]] {
			return t.Seconds[kinds[i]] > t.Seconds[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s %s", WaitKind(k).Label(), minutesText(t.Seconds[k])))
	}
	return fmt.Sprintf("waited %s: %s", minutesText(int64(total/time.Second)), strings.Join(parts, ", "))
}

func minutesText(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d s", seconds)
	}
	return fmt.Sprintf("%d min", (seconds+30)/60)
}

// shortDuration renders d at minute grain past a minute, second grain below.
func shortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	s := strings.TrimSuffix(d.Round(time.Minute).String(), "0s")
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func subjectNoun(subject string) string {
	noun, _, _ := strings.Cut(strings.TrimSpace(subject), " ")
	return noun
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
