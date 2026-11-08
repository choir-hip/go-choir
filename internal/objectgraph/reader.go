package objectgraph

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

// The reader gate (modular architecture plan MA3.3, section 8).
//
// "No cross reads!" (R15). A kind has one custodian (custody.go), and a read
// of it declares who is reading, as a write declares its writer. The gate
// (CheckReadAs) runs in the SQL store's read methods, where the kind of what
// is read is known, so every read of the object graph passes through it
// whether the caller is a reducer, a cell verb, a host loop or an owner route.
//
// The declaration. A read names its reader with WithReader, set by trusted
// in-process code that holds the authority: the run's execution context
// carries the desk of the run record (never the cell's claim), a host loop
// reading a custodian's kind declares that custodian, an owner route declares
// the owner. A context that declares a writer and no reader reads as the
// writer: a write path is the same actor's, and the store reads before it
// writes (an upsert reads the row it replaces). ReaderFrom is that rule; it
// is the one place a reader is resolved.
//
// Who may read a kind (readerAdmits):
//   - a commons kind (system:kernel, E2): every reader, declared or not;
//   - the owner (E1), the kernel and the store: every kind of the built-ins;
//   - a corpusd kind (E4, shared world content): every declared reader;
//   - the kind's custodian;
//   - the readers section 8.2 lists beside the custodian (supplementalReaders);
//   - a package kind (choir.app.<pkg>.*): its package desk and the owner, no
//     one else, not even the kernel (a lease, MA3.4, is the only way in).
//
// Rollout. As the writer gate: a package kind is enforced now (a wrong or
// undeclared reader is refused), a built-in kind only observes. A violation is
// counted under "kind|declared reader (or undeclared)|outcome" and logged
// once per cause, and the read goes on. The cross reads of section 8.4
// therefore show up as reader|wrong_reader keys: X1 as
// choir.texture_document|desk:research|wrong_reader. MA3.7 resolves them and
// flips the built-ins to enforce in code, never by a setting.

var (
	// ErrWrongReader refuses a read whose declared reader is not admitted to
	// the kind.
	ErrWrongReader = errors.New("objectgraph: wrong reader for kind")
	// ErrUndeclaredReader refuses a read of a package kind that declares no
	// reader.
	ErrUndeclaredReader = errors.New("objectgraph: read declares no reader")
)

type readerContextKey struct{}

// WithReader declares who is reading for the reads made with the returned
// context. Call it only from trusted in-process code that holds the
// authority; never from a cell, a model or a request body.
func WithReader(ctx context.Context, r Writer) context.Context {
	return context.WithValue(ctx, readerContextKey{}, r)
}

// WithReaderIfAbsent declares r only when the context names no actor yet, so
// an owner route's declaration never overwrites the desk a ProductAPI call
// already carries in-process: the desk's request reads as the desk.
func WithReaderIfAbsent(ctx context.Context, r Writer) context.Context {
	if _, ok := ReaderFrom(ctx); ok {
		return ctx
	}
	return WithReader(ctx, r)
}

// ReaderFrom resolves the reader of a read: the declared reader, else the
// declared writer (the same actor).
func ReaderFrom(ctx context.Context) (Writer, bool) {
	if ctx == nil {
		return "", false
	}
	if r, ok := ctx.Value(readerContextKey{}).(Writer); ok && r != "" {
		return r, true
	}
	return WriterFrom(ctx)
}

var readerGateMode atomic.Int32

func init() { readerGateMode.Store(int32(WriterGateObserve)) }

// CurrentReaderGateMode is the mode in force (read by the observability route).
func CurrentReaderGateMode() WriterGateMode { return WriterGateMode(readerGateMode.Load()) }

// SetReaderGateMode sets the mode and returns the function that restores the
// previous one. The test injection seam: production runs the default.
func SetReaderGateMode(m WriterGateMode) (restore func()) {
	previous := readerGateMode.Swap(int32(m))
	return func() { readerGateMode.Store(previous) }
}

var (
	readerGateMu     sync.Mutex
	readerGateCounts = map[string]uint64{}
	undeclaredReads  = map[string]uint64{}
	trackUndeclaredR atomic.Bool
	// strictUndeclaredR makes the read oracle exempt only a read whose first
	// caller outside the store and the graph is a test file. The default
	// oracle exempts any stack holding a test frame, which hides every
	// product path a test drives synchronously (a cell turn, a handler call).
	strictUndeclaredR atomic.Bool
)

// ReaderGateCounts returns the violations seen, keyed
// "kind|declared reader (or undeclared)|outcome", where the outcome is
// wrong_reader or undeclared_reader.
func ReaderGateCounts() map[string]uint64 {
	readerGateMu.Lock()
	defer readerGateMu.Unlock()
	out := make(map[string]uint64, len(readerGateCounts))
	for k, v := range readerGateCounts {
		out[k] = v
	}
	return out
}

// ResetReaderGateCounts clears the counts (tests).
func ResetReaderGateCounts() {
	readerGateMu.Lock()
	defer readerGateMu.Unlock()
	readerGateCounts = map[string]uint64{}
	readerGateSites.reset()
}

// TrackUndeclaredReads turns on the undeclared-read tally for a test process
// and returns the function that turns it off: the oracle behind "every read
// declares its reader", as TrackUndeclaredWrites is for writes. A desk-owned
// or service kind read with no declared reader is tallied against the first
// caller outside the store and the graph, unless that caller is a test file.
// Commons kinds are not tallied (every reader is admitted). Production never
// turns it on.
func TrackUndeclaredReads() (stop func()) {
	trackUndeclaredR.Store(true)
	return func() { trackUndeclaredR.Store(false) }
}

// TrackUndeclaredReadsFromProduct is TrackUndeclaredReads with the strict
// attribution: a read is a fixture's only when its direct caller is a test
// file, so a product path that a test drives (a Texture turn, a research run)
// is tallied when it reads with no declared reader (MA3.7c).
func TrackUndeclaredReadsFromProduct() (stop func()) {
	trackUndeclaredR.Store(true)
	strictUndeclaredR.Store(true)
	return func() { trackUndeclaredR.Store(false); strictUndeclaredR.Store(false) }
}

// UndeclaredReads returns the tally, keyed "kind <- caller function (file:line)".
func UndeclaredReads() map[string]uint64 {
	readerGateMu.Lock()
	defer readerGateMu.Unlock()
	out := make(map[string]uint64, len(undeclaredReads))
	for k, v := range undeclaredReads {
		out[k] = v
	}
	return out
}

// ResetUndeclaredReads clears the tally. Only a test that reads without a
// reader on purpose calls it, after the read.
func ResetUndeclaredReads() {
	readerGateMu.Lock()
	defer readerGateMu.Unlock()
	undeclaredReads = map[string]uint64{}
}

// supplementalReaders are the readers section 8.2 lists beside a custodian:
// management's supervisory read of engineering's slots, lineage and
// self-development operations. The owner (E1) and the commons are admitted
// elsewhere. A lease (lease_gate.go) extends a read; this table is not it.
var supplementalReaders = map[Writer][]Writer{
	WriterEngineering: {WriterManagement},
}

// supplementalKindReaders are readers admitted to one kind of another
// custodian: a projection the custodian publishes for them (MA3.7a). Unlike
// supplementalReaders it never admits a reader to the custodian's other
// kinds, so the crossing into Texture's documents stays visible.
var supplementalKindReaders = map[ObjectKind][]Writer{
	"choir.story_index": {WriterManagement},
	// E4 (MA3.7c): world events, their memberships and capture labels are
	// shared world content. Management keeps them (the intake loops write
	// them); research and Texture read them, as the corpus search and the
	// story prompts do. The owner-private labelling of a capture's body is
	// the capture frame handler's, not these index kinds'.
	"choir.world_event":      {WriterResearch, WriterTexture},
	"choir.event_membership": {WriterResearch, WriterTexture},
	"choir.capture_label":    {WriterResearch, WriterTexture},
	// Enforce flip D3: the source cards Texture renders resolve refs a
	// desk's record carries. A content_item is the shell custodian's text,
	// which Texture already renders; an agent_evidence record is research's,
	// and the owner's 2026-11-06 ruling expects Texture to use it. Both are
	// admitted to Texture alone.
	"choir.content_item":   {WriterTexture},
	"choir.agent_evidence": {WriterTexture},
}

// readerAdmits reports whether a read declared as (declared, ok) may read a
// kind registered as reg.
func readerAdmits(reg KindRegistration, declared Writer, ok bool) bool {
	if strings.HasPrefix(string(reg.Kind), packageKindNamespace) {
		return ok && (declared == reg.Writer || declared == WriterOwner)
	}
	switch {
	case reg.Writer == WriterKernel:
		return true
	case !ok:
		return false
	case declared == WriterOwner, declared == WriterKernel, declared == WriterStore:
		return true
	case reg.Writer == WriterCorpusd:
		return true
	case declared == reg.Writer:
		return true
	}
	for _, r := range supplementalReaders[reg.Writer] {
		if r == declared {
			return true
		}
	}
	for _, r := range supplementalKindReaders[reg.Kind] {
		if r == declared {
			return true
		}
	}
	return false
}

// CheckReadAs checks a read of kind against its declared reader. A package
// kind's violation is refused; a built-in kind's is, in observe mode, counted
// and allowed. In off mode nothing is checked, and an unregistered kind has no
// custodian to cross.
func CheckReadAs(ctx context.Context, kind ObjectKind) error {
	return checkReadKind(ctx, kind)
}

func checkReadKinds(ctx context.Context, kinds []ObjectKind) error {
	for _, k := range kinds {
		if err := checkReadKind(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

func checkReadKind(ctx context.Context, kind ObjectKind) error {
	global := WriterGateMode(readerGateMode.Load())
	if global == WriterGateOff {
		return nil
	}
	declared, ok := ReaderFrom(ctx)
	if ok && (declared == WriterOwner || declared == WriterKernel || declared == WriterStore) && !strings.HasPrefix(string(kind), packageKindNamespace) {
		return nil
	}
	reg, err := writeRegistry.LookupKind(kind)
	if err != nil || readerAdmits(reg, declared, ok) {
		return nil
	}
	// A live lease adds a reader the custody table did not admit (MA3.4).
	if ok && LeaseAdmitsKind(ctx, declared, kind) {
		NoteLeaseRead(declared, string(kind))
		return nil
	}
	mode := modeForKind(kind, global)
	if !ok && trackUndeclaredR.Load() && reg.Writer != WriterKernel {
		recordUndeclaredRead(kind)
	}
	reason, refusal := "wrong_reader", ErrWrongReader
	if !ok {
		reason, refusal = "undeclared_reader", ErrUndeclaredReader
	}
	recordReaderViolation(kind, declared, ok, reason, mode)
	if mode == WriterGateEnforce {
		return fmt.Errorf("%w: kind %q is kept by %q; the read declares %q", refusal, kind, reg.Writer, writerLabel(declared, ok))
	}
	return nil
}

// readableObjects checks the kinds of a result whose query named none (a
// whole-scope snapshot, a list with no kind filter). Each kind the reader is
// not admitted to is counted once; in enforce mode (a package kind) its
// objects are dropped from the result.
func readableObjects(ctx context.Context, objs []Object) []Object {
	if len(objs) == 0 || WriterGateMode(readerGateMode.Load()) == WriterGateOff {
		return objs
	}
	var refused map[ObjectKind]bool
	seen := map[ObjectKind]bool{}
	for _, obj := range objs {
		if seen[obj.ObjectKind] {
			continue
		}
		seen[obj.ObjectKind] = true
		if checkReadKind(ctx, obj.ObjectKind) != nil {
			if refused == nil {
				refused = map[ObjectKind]bool{}
			}
			refused[obj.ObjectKind] = true
		}
	}
	if refused == nil {
		return objs
	}
	out := make([]Object, 0, len(objs))
	for _, obj := range objs {
		if !refused[obj.ObjectKind] {
			out = append(out, obj)
		}
	}
	return out
}

func recordReaderViolation(kind ObjectKind, declared Writer, ok bool, reason string, mode WriterGateMode) {
	who := "undeclared"
	if ok {
		who = string(declared)
	}
	key := fmt.Sprintf("%s|%s|%s", kind, who, reason)
	readerGateSites.tally(kind, reason)
	readerGateMu.Lock()
	readerGateCounts[key]++
	first := readerGateCounts[key] == 1
	readerGateMu.Unlock()
	if first {
		log.Printf("objectgraph: reader gate (%s): %s", mode, key)
	}
}

func recordUndeclaredRead(kind ObjectKind) {
	key, ok := undeclaredCaller(kind, !strictUndeclaredR.Load())
	if !ok {
		return
	}
	readerGateMu.Lock()
	undeclaredReads[key]++
	readerGateMu.Unlock()
}

// NoteWrongReader counts a read the kind check cannot see: the frame handler
// of a by-id read whose record carries the label that decides it (a capture
// with an owner-private label, MA3.3). The read goes on in observe mode, as
// every built-in kind's does. Production-only; the outcome is wrong_reader.
func NoteWrongReader(kind ObjectKind, reader Writer, ok bool) {
	if WriterGateMode(readerGateMode.Load()) == WriterGateOff {
		return
	}
	recordReaderViolation(kind, reader, ok, "wrong_reader", modeForKind(kind, WriterGateMode(readerGateMode.Load())))
}
