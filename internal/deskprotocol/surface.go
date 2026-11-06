package deskprotocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Surface is the extracted desk protocol: what the lock stores.
type Surface struct {
	Schema  string `json:"schema"`
	Version string `json:"version"`
	// SHA256 is the hash of the surface with this field empty, in compact
	// canonical JSON. An in-place edit of a lock is visible without the tool.
	SHA256      string              `json:"sha256"`
	Names       map[string]Name     `json:"names"`
	Verbs       map[string]Verb     `json:"verbs"`
	Desks       map[string]DeskRow  `json:"desks"`
	Transitions map[string][]string `json:"transitions"`
	// Enforced lists the machines the store reads the table for. Removing
	// one loosens the terminal-edge protection, so it is a major change.
	Enforced []string `json:"enforced"`
}

// Name is one declaration of the surface.
type Name struct {
	// Kind is struct, enum, constgroup, const, func, interface or type.
	Kind      string    `json:"kind"`
	Direction Direction `json:"direction"`
	Open      bool      `json:"open,omitempty"`
	Quota     bool      `json:"quota,omitempty"`
	// Underlying is the base type of an enum, const or type.
	Underlying string `json:"underlying,omitempty"`
	// Fields is a struct's fields by Go name.
	Fields map[string]Field `json:"fields,omitempty"`
	// Values is an enum's or constant group's values by constant name.
	Values map[string]string `json:"values,omitempty"`
	// Value is a constant's value.
	Value string `json:"value,omitempty"`
	// Sig is a function's signature; Methods an interface's.
	Sig     *Sig            `json:"sig,omitempty"`
	Methods map[string]*Sig `json:"methods,omitempty"`
}

// Field is one struct field as a decoder sees it.
type Field struct {
	Type     string `json:"type"`
	JSON     string `json:"json"`
	Optional bool   `json:"optional,omitempty"`
	Embedded bool   `json:"embedded,omitempty"`
}

// Sig is a function signature. Names are recorded apart from types, because a
// parameter rename changes nothing a caller can see.
type Sig struct {
	Params   []string `json:"params"`
	Names    []string `json:"names,omitempty"`
	Variadic bool     `json:"variadic,omitempty"`
	Results  []string `json:"results"`
}

// Verb is one choir verb. Closure is true when the verb is a closure in the
// choir exports with no method to read a signature from.
type Verb struct {
	Closure bool `json:"closure,omitempty"`
	Sig     *Sig `json:"sig,omitempty"`
}

// DeskRow is one desk's slice of the registry: its choir verbs and every field
// of the registry row that grants it power (MA2.5 module surface). Choir verbs
// are the protocol's open surface; the authority fields are not, so a change to
// any of them is a major change.
type DeskRow struct {
	Kind  string   `json:"kind"`
	Verbs []string `json:"verbs"`
	// CapsuleVerbs are the broker verbs a capsule-carried desk may call.
	CapsuleVerbs []string `json:"capsule_verbs"`
	// Modules are the observation modules the desk holds beyond the base tier.
	Modules []string `json:"modules"`
	// OwnedKinds are the object kinds the desk is the sole writer of.
	OwnedKinds []string `json:"owned_kinds"`
	Carrier    string   `json:"carrier"`
	Instance   string   `json:"instance"`
	Taint      string   `json:"taint"`
	// Sandbox is the sandbox profile name; SandboxUID the worker uid.
	Sandbox    string `json:"sandbox"`
	SandboxUID uint32 `json:"sandbox_uid"`
	// The rest is the field set deskmanifest.Desk.Equal compares (Name is the
	// map key). A test pins that this row and Equal cannot drift apart.
	ModelRole  string `json:"model_role"`
	Restart    string `json:"restart"`
	Profile    bool   `json:"profile"`
	Prompt     bool   `json:"prompt"`
	Identity   string `json:"identity"`
	OwnChannel bool   `json:"own_channel"`
	// CellDeadlineNs is the host's cell bound in nanoseconds.
	CellDeadlineNs int64 `json:"cell_deadline_ns"`
	// Memory: the worker cgroup bounds and the capsule's requested bytes.
	WorkerMaxBytes        uint64 `json:"worker_max_bytes"`
	WorkerHighBytes       uint64 `json:"worker_high_bytes"`
	CapsuleRequestedBytes int64  `json:"capsule_requested_bytes"`
	// LeaseKinds and LeaseCaptures are the desk's lease ceiling (MA3.4). They
	// are omitted when empty so a lock written before the ceiling existed
	// keeps its hash.
	LeaseKinds    []string `json:"lease_kinds,omitempty"`
	LeaseCaptures bool     `json:"lease_captures,omitempty"`
	// Grants are the desk's authority as name or name(args), sorted (MA4.1);
	// EndsRun marks a role that holds the lifecycle grant its parent lacks.
	Grants  []string `json:"grants,omitempty"`
	EndsRun bool     `json:"ends_run,omitempty"`
}

// Canonical returns the surface as compact JSON with the hash field blank.
func (s *Surface) canonical() ([]byte, error) {
	c := *s
	c.SHA256 = ""
	return json.Marshal(&c)
}

// Seal sets the surface's hash.
func (s *Surface) Seal() error {
	b, err := s.canonical()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	s.SHA256 = hex.EncodeToString(sum[:])
	return nil
}

// VerifySeal reports whether the stored hash matches the body.
func (s *Surface) VerifySeal() error {
	b, err := s.canonical()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != s.SHA256 {
		return fmt.Errorf("lock %s: body hash %s does not match its sha256 %s (edited in place)", s.Version, got, s.SHA256)
	}
	return nil
}

// Marshal renders the lock file bytes: indented, sorted, newline-ended.
func (s *Surface) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Equal reports whether two surfaces are the same apart from the hash.
func (s *Surface) Equal(o *Surface) bool {
	a, err1 := s.canonical()
	b, err2 := o.canonical()
	return err1 == nil && err2 == nil && bytes.Equal(a, b)
}

// ---- versions ----

// Level is the size of a change.
type Level int

const (
	LevelNone Level = iota
	LevelPatch
	LevelMinor
	LevelMajor
)

func (l Level) String() string {
	return [...]string{"none", "patch", "minor", "major"}[l]
}

// ParseVersion parses MAJOR.MINOR.PATCH.
func ParseVersion(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", v)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strconv.Itoa(n) != p {
			return out, fmt.Errorf("version %q is not MAJOR.MINOR.PATCH", v)
		}
		out[i] = n
	}
	return out, nil
}

// CompareVersions orders two versions.
func CompareVersions(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Bump is the size of the step from one version to a higher one.
func Bump(from, to string) (Level, error) {
	a, err := ParseVersion(from)
	if err != nil {
		return 0, err
	}
	b, err := ParseVersion(to)
	if err != nil {
		return 0, err
	}
	switch c := CompareVersions(a, b); {
	case c == 0:
		return LevelNone, nil
	case c > 0:
		return 0, fmt.Errorf("version %s is below %s", to, from)
	}
	switch {
	case a[0] != b[0]:
		return LevelMajor, nil
	case a[1] != b[1]:
		return LevelMinor, nil
	}
	return LevelPatch, nil
}

// ---- classification ----

// Change is one difference between two surfaces and its size.
type Change struct {
	Level Level
	What  string
}

// Result is a classification: the largest change and every change.
type Result struct {
	Level   Level
	Changes []Change
}

func (r *Result) add(l Level, format string, args ...any) {
	r.Changes = append(r.Changes, Change{l, fmt.Sprintf(format, args...)})
	if l > r.Level {
		r.Level = l
	}
}

// Classify sizes the change from old to new. The rules mirror the SDK
// checker (frontend/scripts/sdk-surface.mjs) for a Go surface:
//
//	major  a name, field, enum value, verb, transition or desk
//	       leaves; a type, tag, kind or direction changes; a required field
//	       is added to a type a desk emits; a value is added to a closed enum;
//	       a signature changes other than by a trailing variadic parameter
//	minor  a name, optional field, value of an open enum, verb, transition,
//	       desk is added; a quota rises; a required field is added
//	       to a type only received; a field turns optional
//	patch  only parameter names changed
func Classify(old, cur *Surface) Result {
	var r Result
	for _, n := range sortedKeys(old.Names) {
		o := old.Names[n]
		c, ok := cur.Names[n]
		if !ok {
			r.add(LevelMajor, "name %s left", n)
			continue
		}
		classifyName(&r, n, o, c)
	}
	for _, n := range sortedKeys(cur.Names) {
		if _, ok := old.Names[n]; !ok {
			r.add(LevelMinor, "name %s added", n)
		}
	}
	for _, v := range sortedKeys(old.Verbs) {
		c, ok := cur.Verbs[v]
		if !ok {
			r.add(LevelMajor, "verb %s left", v)
			continue
		}
		o := old.Verbs[v]
		switch {
		case o.Closure != c.Closure:
			r.add(LevelMajor, "verb %s changed between closure and method", v)
		case o.Sig != nil && c.Sig != nil:
			classifySig(&r, "verb "+v, o.Sig, c.Sig)
		}
	}
	for _, v := range sortedKeys(cur.Verbs) {
		if _, ok := old.Verbs[v]; !ok {
			r.add(LevelMinor, "verb %s added", v)
		}
	}
	for _, d := range sortedKeys(old.Desks) {
		c, ok := cur.Desks[d]
		if !ok {
			r.add(LevelMajor, "desk %s left", d)
			continue
		}
		o := old.Desks[d]
		if o.Kind != c.Kind {
			r.add(LevelMajor, "desk %s kind %s -> %s", d, o.Kind, c.Kind)
		}
		setDiff(&r, "desk "+d+" verb", o.Verbs, c.Verbs)
		// Authority: any change, an addition included, is major.
		setDiffMajor(&r, "desk "+d+" capsule verb", o.CapsuleVerbs, c.CapsuleVerbs)
		setDiffMajor(&r, "desk "+d+" module", o.Modules, c.Modules)
		setDiffMajor(&r, "desk "+d+" owned kind", o.OwnedKinds, c.OwnedKinds)
		// The lease ceiling is authority: a widening is major.
		setDiffMajor(&r, "desk "+d+" lease kind", o.LeaseKinds, c.LeaseKinds)
		setDiffMajor(&r, "desk "+d+" grant", o.Grants, c.Grants)
		for _, f := range []struct{ what, o, c string }{
			{"carrier", o.Carrier, c.Carrier}, {"instance", o.Instance, c.Instance},
			{"taint", o.Taint, c.Taint}, {"sandbox", o.Sandbox, c.Sandbox},
			{"sandbox uid", fmt.Sprint(o.SandboxUID), fmt.Sprint(c.SandboxUID)},
			{"model role", o.ModelRole, c.ModelRole}, {"restart", o.Restart, c.Restart},
			{"profile", fmt.Sprint(o.Profile), fmt.Sprint(c.Profile)},
			{"prompt", fmt.Sprint(o.Prompt), fmt.Sprint(c.Prompt)},
			{"identity", o.Identity, c.Identity},
			{"own channel", fmt.Sprint(o.OwnChannel), fmt.Sprint(c.OwnChannel)},
			{"lease captures", fmt.Sprint(o.LeaseCaptures), fmt.Sprint(c.LeaseCaptures)},
			{"ends run", fmt.Sprint(o.EndsRun), fmt.Sprint(c.EndsRun)},
			{"cell deadline ns", fmt.Sprint(o.CellDeadlineNs), fmt.Sprint(c.CellDeadlineNs)},
			{"worker max bytes", fmt.Sprint(o.WorkerMaxBytes), fmt.Sprint(c.WorkerMaxBytes)},
			{"worker high bytes", fmt.Sprint(o.WorkerHighBytes), fmt.Sprint(c.WorkerHighBytes)},
			{"capsule requested bytes", fmt.Sprint(o.CapsuleRequestedBytes), fmt.Sprint(c.CapsuleRequestedBytes)},
		} {
			if f.o != f.c {
				r.add(LevelMajor, "desk %s %s %q -> %q", d, f.what, f.o, f.c)
			}
		}
	}
	for _, d := range sortedKeys(cur.Desks) {
		if _, ok := old.Desks[d]; !ok {
			r.add(LevelMinor, "desk %s added", d)
		}
	}
	for _, m := range sortedKeys(old.Transitions) {
		setDiff(&r, "transition "+m, old.Transitions[m], cur.Transitions[m])
		if Enforced(Machine(m)) {
			// The store reads an enforced machine for terminality: a new edge
			// out of a state that was terminal loosens a guarantee.
			from, terminal := map[string]bool{}, map[string]bool{}
			for _, e := range old.Transitions[m] {
				f, t, _ := strings.Cut(e, ">")
				from[f], terminal[t] = true, true
			}
			for _, e := range cur.Transitions[m] {
				f, _, _ := strings.Cut(e, ">")
				if terminal[f] && !from[f] {
					r.add(LevelMajor, "transition %s %s leaves terminal state %s", m, e, f)
				}
			}
		}
	}
	setDiff(&r, "machine enforcement", old.Enforced, cur.Enforced)
	for _, m := range sortedKeys(cur.Transitions) {
		if _, ok := old.Transitions[m]; !ok {
			r.add(LevelMinor, "machine %s added", m)
		}
	}
	return r
}

// setDiffMajor: a set that grants authority; an element that arrives or leaves
// is major.
func setDiffMajor(r *Result, what string, old, cur []string) {
	in := func(set []string, s string) bool {
		for _, x := range set {
			if x == s {
				return true
			}
		}
		return false
	}
	for _, s := range old {
		if !in(cur, s) {
			r.add(LevelMajor, "%s %s left", what, s)
		}
	}
	for _, s := range cur {
		if !in(old, s) {
			r.add(LevelMajor, "%s %s added", what, s)
		}
	}
}

// setDiff: an element that leaves is major, one that arrives minor.
func setDiff(r *Result, what string, old, cur []string) {
	in := func(set []string, s string) bool {
		for _, x := range set {
			if x == s {
				return true
			}
		}
		return false
	}
	for _, s := range old {
		if !in(cur, s) {
			r.add(LevelMajor, "%s %s left", what, s)
		}
	}
	for _, s := range cur {
		if !in(old, s) {
			r.add(LevelMinor, "%s %s added", what, s)
		}
	}
}

func classifyName(r *Result, n string, o, c Name) {
	if o.Kind != c.Kind {
		r.add(LevelMajor, "%s kind %s -> %s", n, o.Kind, c.Kind)
		return
	}
	if o.Direction != c.Direction {
		r.add(LevelMajor, "%s direction %s -> %s", n, o.Direction, c.Direction)
	}
	if o.Underlying != c.Underlying {
		r.add(LevelMajor, "%s underlying type %s -> %s", n, o.Underlying, c.Underlying)
	}
	switch o.Kind {
	case "struct":
		emitted := c.Direction != Received
		for _, f := range sortedKeys(o.Fields) {
			of := o.Fields[f]
			cf, ok := c.Fields[f]
			if !ok {
				r.add(LevelMajor, "%s field %s left", n, f)
				continue
			}
			if of.Type != cf.Type {
				r.add(LevelMajor, "%s.%s type %s -> %s", n, f, of.Type, cf.Type)
			}
			if of.JSON != cf.JSON {
				r.add(LevelMajor, "%s.%s json tag %q -> %q", n, f, of.JSON, cf.JSON)
			}
			if of.Embedded != cf.Embedded {
				r.add(LevelMajor, "%s.%s embedding changed", n, f)
			}
			switch {
			case of.Optional && !cf.Optional:
				if emitted {
					r.add(LevelMajor, "%s.%s became required", n, f)
				} else {
					r.add(LevelMinor, "%s.%s became required (received)", n, f)
				}
			case !of.Optional && cf.Optional:
				r.add(LevelMinor, "%s.%s became optional", n, f)
			}
		}
		for _, f := range sortedKeys(c.Fields) {
			if _, ok := o.Fields[f]; ok {
				continue
			}
			switch {
			case c.Fields[f].Optional:
				r.add(LevelMinor, "%s optional field %s added", n, f)
			case emitted:
				r.add(LevelMajor, "%s required field %s added to an emitted type", n, f)
			default:
				r.add(LevelMinor, "%s required field %s added (received)", n, f)
			}
		}
	case "enum", "constgroup":
		if o.Open && !c.Open {
			r.add(LevelMajor, "%s closed (was open)", n)
		} else if !o.Open && c.Open {
			r.add(LevelMinor, "%s opened", n)
		}
		for _, v := range sortedKeys(o.Values) {
			cv, ok := c.Values[v]
			switch {
			case !ok:
				r.add(LevelMajor, "%s value %s left", n, v)
			case cv != o.Values[v]:
				r.add(LevelMajor, "%s value %s changed %s -> %s", n, v, o.Values[v], cv)
			}
		}
		for _, v := range sortedKeys(c.Values) {
			if _, ok := o.Values[v]; ok {
				continue
			}
			if c.Open {
				r.add(LevelMinor, "%s value %s added (open)", n, v)
			} else {
				r.add(LevelMajor, "%s value %s added to a closed enum", n, v)
			}
		}
	case "const":
		if o.Value == c.Value {
			break
		}
		ov, e1 := strconv.ParseFloat(o.Value, 64)
		cv, e2 := strconv.ParseFloat(c.Value, 64)
		if c.Quota && e1 == nil && e2 == nil && cv > ov {
			r.add(LevelMinor, "%s quota raised %s -> %s", n, o.Value, c.Value)
		} else {
			r.add(LevelMajor, "%s value %s -> %s", n, o.Value, c.Value)
		}
	case "func":
		if o.Sig != nil && c.Sig != nil {
			classifySig(r, n, o.Sig, c.Sig)
		}
	case "interface":
		for _, m := range sortedKeys(o.Methods) {
			cm, ok := c.Methods[m]
			if !ok {
				r.add(LevelMajor, "%s method %s left", n, m)
				continue
			}
			classifySig(r, n+"."+m, o.Methods[m], cm)
		}
		for _, m := range sortedKeys(c.Methods) {
			if _, ok := o.Methods[m]; !ok {
				r.add(LevelMinor, "%s method %s added", n, m)
			}
		}
	}
}

func classifySig(r *Result, what string, o, c *Sig) {
	same := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if !same(o.Results, c.Results) {
		r.add(LevelMajor, "%s results changed", what)
		return
	}
	if same(o.Params, c.Params) && o.Variadic == c.Variadic {
		if !same(o.Names, c.Names) {
			r.add(LevelPatch, "%s parameter names changed", what)
		}
		return
	}
	// A trailing variadic parameter added: every old call still compiles.
	if !o.Variadic && c.Variadic && len(c.Params) == len(o.Params)+1 && same(o.Params, c.Params[:len(o.Params)]) {
		r.add(LevelMinor, "%s gained a trailing variadic parameter", what)
		return
	}
	r.add(LevelMajor, "%s parameters changed", what)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- locks on disk ----

// LockDir is where the locks live, relative to the repository root.
const LockDir = "internal/deskprotocol/lock"

// TestdataDir holds the golden corpus, immutable per version like the locks.
const TestdataDir = "internal/deskprotocol/testdata"

// LoadLocks reads every lock in dir, sorted by version. Each lock must carry
// its own name as its version and a body that matches its hash.
func LoadLocks(dir string) ([]*Surface, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*Surface
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		s, err := ParseLock(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if want := strings.TrimSuffix(e.Name(), ".json"); s.Version != want {
			return nil, fmt.Errorf("%s: the file is named for version %s but holds version %s", e.Name(), want, s.Version)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := ParseVersion(out[i].Version)
		b, _ := ParseVersion(out[j].Version)
		return CompareVersions(a, b) < 0
	})
	return out, nil
}

// ParseLock decodes a lock strictly and verifies its schema, version and seal.
func ParseLock(raw []byte) (*Surface, error) {
	var s Surface
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if s.Schema != SurfaceSchema {
		return nil, fmt.Errorf("schema %q, want %q", s.Schema, SurfaceSchema)
	}
	if _, err := ParseVersion(s.Version); err != nil {
		return nil, err
	}
	if err := s.VerifySeal(); err != nil {
		return nil, err
	}
	return &s, nil
}

// CheckLockSet verifies every adjacent pair of locks is stepped by at least
// the bump its change needs, and that each lock is internally sound (floors).
func CheckLockSet(locks []*Surface) []string {
	var problems []string
	for i, l := range locks {
		if len(l.Names) < MinNames || len(l.Verbs) < MinVerbs || len(l.Desks) < MinDesks {
			problems = append(problems, fmt.Sprintf("lock %s is below the floors (names %d/%d, verbs %d/%d, desks %d/%d)",
				l.Version, len(l.Names), MinNames, len(l.Verbs), MinVerbs, len(l.Desks), MinDesks))
		}
		if i == 0 {
			continue
		}
		prev := locks[i-1]
		got, err := Bump(prev.Version, l.Version)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		res := Classify(prev, l)
		if got < res.Level {
			problems = append(problems, fmt.Sprintf("%s -> %s is a %s bump but the change is %s:\n%s", prev.Version, l.Version, got, res.Level, formatChanges(res)))
		}
	}
	return problems
}

func formatChanges(r Result) string {
	var b strings.Builder
	for _, c := range r.Changes {
		fmt.Fprintf(&b, "    %s: %s\n", c.Level, c.What)
	}
	return b.String()
}

// FormatChanges renders a classification for a human.
func FormatChanges(r Result) string { return formatChanges(r) }

// parseNoSeal decodes a surface without checking its seal (tests mutate
// surfaces they have not sealed).
func parseNoSeal(raw []byte) (*Surface, error) {
	var s Surface
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
