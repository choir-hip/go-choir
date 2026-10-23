package objectgraph

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// One writer per object kind (SDC ARCH-04;
// docs/problems/object-kind-writer-is-a-package-not-a-desk-2026-10-21.md).
//
// The registry names, for each kind, the one writer allowed to write it.
// Texture's sole-writer rule was a private check in the Texture turn; this
// generalizes it to the graph, where a package install can state it for an app
// desk's kinds and the write gate can refuse a different writer.
//
// A write declares who is writing with WithWriter, set by trusted in-process
// code at the method that holds the desk's authority; never from a cell, a
// model or a request body. The gate (CheckWriteAs) runs where CheckWrite does,
// before the event tape.
//
// Mixed batches are normal: a Texture turn commits its revision with the
// runtime's inbox and event bookkeeping in one transaction. So a kind written
// by the runtime store on behalf of any desk (WriterRuntime) is admitted under
// every declared writer; a desk-owned kind is admitted under its desk (or the
// owner where OwnerEdits); a corpusd kind under corpusd.
//
// Rollout. The classification of the VM store's own kinds into per-desk
// writers is the desk manifest's "owned object kinds" field (SDC ARCH-02).
// Until the call sites declare their writer, the default mode only observes:
// a wrong or undeclared writer of a desk or corpusd kind is counted, logged
// once per cause and allowed. enforce refuses; flipping to it is a deliberate
// act after the counts are clean.

// Writer names who writes a kind, or who is writing.
type Writer string

const (
	// Desk writers. A desk is the sole agent writer of the kinds it owns.
	WriterTexture     Writer = "desk:texture"
	WriterManagement  Writer = "desk:management"
	WriterEngineering Writer = "desk:engineering"
	WriterResearch    Writer = "desk:research"
	// WriterAutoradio is reserved for kinds no code writes yet.
	WriterAutoradio Writer = "desk:autoradio"
	// WriterRuntime is the VM store acting for the desks: lifecycle, inbox,
	// event and assignment bookkeeping. Its per-desk split is ARCH-02's.
	WriterRuntime Writer = "system:runtime"
	// WriterCorpusd is the publication layer (Store B).
	WriterCorpusd Writer = "system:corpusd"
	// WriterStore is the VM store acting for itself, not for a desk: the
	// stream_seq lease (choir.stream_seq_lease), written only by the store's
	// allocator behind the capability fence in CheckWriteAs. It is not
	// WriterRuntime: a runtime kind admits every declared writer.
	WriterStore Writer = "system:store"
	// WriterOwner is the owner's own edit path. It is declared by a write; it
	// is never a kind's writer. A kind admits it only with OwnerEdits.
	WriterOwner Writer = "owner"
)

// BuiltinWriters are the writers a built-in kind may name.
var BuiltinWriters = map[Writer]bool{
	WriterTexture: true, WriterManagement: true, WriterEngineering: true, WriterResearch: true,
	WriterAutoradio: true, WriterRuntime: true, WriterCorpusd: true, WriterStore: true,
}

var appDeskWriter = regexp.MustCompile(`^desk:[a-z][a-z0-9_-]{1,31}$`)

// ValidKindWriter reports whether w may be named as a kind's writer: a built-in
// writer, or an app desk (desk:<name>) a package declares.
func ValidKindWriter(w Writer) bool {
	return BuiltinWriters[w] || appDeskWriter.MatchString(string(w))
}

var (
	// ErrWrongWriter refuses a write whose declared writer is not the kind's.
	ErrWrongWriter = errors.New("objectgraph: wrong writer for kind")
	// ErrUndeclaredWriter refuses, in enforce mode, a write of a desk or
	// corpusd kind that declares no writer.
	ErrUndeclaredWriter = errors.New("objectgraph: write declares no writer")
	// ErrWriterConflict refuses registering a kind under a second writer.
	ErrWriterConflict = errors.New("objectgraph: kind is registered under another writer")
	// ErrInvalidWriter refuses registering a kind without exactly one valid writer.
	ErrInvalidWriter = errors.New("objectgraph: kind needs one valid writer")
)

type writerContextKey struct{}

// WithWriter declares who is writing for the writes made with the returned
// context. Call it only from trusted in-process code that holds the authority.
func WithWriter(ctx context.Context, w Writer) context.Context {
	return context.WithValue(ctx, writerContextKey{}, w)
}

// WriterFrom reads the declared writer.
func WriterFrom(ctx context.Context) (Writer, bool) {
	if ctx == nil {
		return "", false
	}
	w, ok := ctx.Value(writerContextKey{}).(Writer)
	return w, ok && w != ""
}

// WriterGateMode is how the writer check treats a violation.
type WriterGateMode int32

const (
	// WriterGateOff does not check writers.
	WriterGateOff WriterGateMode = iota
	// WriterGateObserve counts and logs a violation once per cause and allows
	// the write. The default.
	WriterGateObserve
	// WriterGateEnforce refuses a violation.
	WriterGateEnforce
)

func (m WriterGateMode) String() string {
	switch m {
	case WriterGateOff:
		return "off"
	case WriterGateEnforce:
		return "enforce"
	default:
		return "observe"
	}
}

// WriterGateModeFromEnv reads CHOIR_OG_WRITER_GATE (off, observe, enforce). An
// empty or unknown value is observe: a typo never turns the gate off.
func WriterGateModeFromEnv() WriterGateMode {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CHOIR_OG_WRITER_GATE"))) {
	case "off":
		return WriterGateOff
	case "enforce":
		return WriterGateEnforce
	default:
		return WriterGateObserve
	}
}

var writerGateMode atomic.Int32

func init() { writerGateMode.Store(int32(WriterGateModeFromEnv())) }

// CurrentWriterGateMode is the mode in force (read by the observability route).
func CurrentWriterGateMode() WriterGateMode { return WriterGateMode(writerGateMode.Load()) }

// SetWriterGateMode sets the mode and returns the function that restores the
// previous one. It is the test injection seam; production reads the environment.
func SetWriterGateMode(m WriterGateMode) (restore func()) {
	previous := writerGateMode.Swap(int32(m))
	return func() { writerGateMode.Store(previous) }
}

var (
	writerGateMu     sync.Mutex
	writerGateCounts = map[string]uint64{}
)

// WriterGateCounts returns the violations seen, keyed
// "kind|declared writer (or undeclared)|reason". It is the oracle for which
// call sites still need to declare a writer.
func WriterGateCounts() map[string]uint64 {
	writerGateMu.Lock()
	defer writerGateMu.Unlock()
	out := make(map[string]uint64, len(writerGateCounts))
	for k, v := range writerGateCounts {
		out[k] = v
	}
	return out
}

// ResetWriterGateCounts clears the counts (tests).
func ResetWriterGateCounts() {
	writerGateMu.Lock()
	defer writerGateMu.Unlock()
	writerGateCounts = map[string]uint64{}
}

func recordWriterViolation(kind ObjectKind, declared Writer, ok bool, reason string) {
	who := "undeclared"
	if ok {
		who = string(declared)
	}
	key := fmt.Sprintf("%s|%s|%s", kind, who, reason)
	writerGateMu.Lock()
	writerGateCounts[key]++
	first := writerGateCounts[key] == 1
	writerGateMu.Unlock()
	if first {
		log.Printf("objectgraph: writer gate (%s): %s", WriterGateMode(writerGateMode.Load()), key)
	}
}

// writerAdmits reports whether a write declared as (declared, ok) may write
// kind: the kind's own writer, any writer for a runtime kind, the owner where
// the kind admits owner edits.
func writerAdmits(reg KindRegistration, declared Writer, ok bool) bool {
	switch {
	case reg.Writer == WriterRuntime:
		return true
	case !ok:
		return false
	case declared == reg.Writer:
		return true
	case declared == WriterOwner && reg.OwnerEdits:
		return true
	}
	return false
}

// CheckWriteAs is CheckWrite plus the writer check: the kinds must be
// registered, and the declared writer must be the kind's writer. Tombstones
// pass, as in CheckWrite. Edges carry no writer. In observe mode a violation
// is counted and allowed; in off mode it is not checked.
func CheckWriteAs(ctx context.Context, objects []Object, edges []Edge) error {
	if err := CheckWrite(objects, edges); err != nil {
		return err
	}
	// The hard fence, ahead of the mode check so it holds in off, observe and
	// enforce alike: a lease written by anyone but the store's allocator can
	// burn the number space (a forged high R) or reissue numbers (a low one).
	if err := checkStreamSeqLeaseFence(ctx, objects); err != nil {
		return err
	}
	mode := WriterGateMode(writerGateMode.Load())
	if mode == WriterGateOff {
		return nil
	}
	declared, ok := WriterFrom(ctx)
	for _, obj := range objects {
		if obj.Tombstone {
			continue
		}
		reg, err := writeRegistry.LookupKind(obj.ObjectKind)
		if err != nil || writerAdmits(reg, declared, ok) {
			continue
		}
		reason, refusal := "wrong_writer", ErrWrongWriter
		if !ok {
			reason, refusal = "undeclared_writer", ErrUndeclaredWriter
		}
		recordWriterViolation(obj.ObjectKind, declared, ok, reason)
		if mode == WriterGateEnforce {
			return fmt.Errorf("%w: kind %q is written by %q; the write declares %q (object %s)",
				refusal, obj.ObjectKind, reg.Writer, writerLabel(declared, ok), obj.CanonicalID)
		}
	}
	return nil
}

func writerLabel(w Writer, ok bool) string {
	if !ok {
		return "no writer"
	}
	return string(w)
}

// RegisterKindChecked is the seam a package install uses for an app desk's
// kinds. It refuses a kind without exactly one valid writer, and a kind that is
// already registered under a different writer. An identical re-registration is
// a no-op. (RegisterKind remains the unchecked registration DefaultRegistry
// uses; the coverage test holds its table to the same rule.)
func (r *Registry) RegisterKindChecked(k KindRegistration) error {
	if strings.TrimSpace(string(k.Kind)) == "" {
		return fmt.Errorf("%w: kind is empty", ErrInvalidWriter)
	}
	if !ValidKindWriter(k.Writer) {
		return fmt.Errorf("%w: kind %q names writer %q", ErrInvalidWriter, k.Kind, k.Writer)
	}
	if k.IdentityMode == "" {
		k.IdentityMode = IdentityContentHash
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, found := r.kinds[k.Kind]; found {
		if existing.Writer != k.Writer {
			return fmt.Errorf("%w: kind %q is written by %q, not %q", ErrWriterConflict, k.Kind, existing.Writer, k.Writer)
		}
		return nil
	}
	r.kinds[k.Kind] = k
	return nil
}

// ErrLeaseFence refuses a write of the stream_seq lease kind by anything but
// the store's own allocator.
var ErrLeaseFence = errors.New("objectgraph: stream_seq lease written outside the store's allocator")

// KindStreamSeqLease is the kind of the stream_seq lease object
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md): the highest
// stream_seq this computer may hand out, written ahead in blocks through the
// projection interceptor.
const KindStreamSeqLease ObjectKind = "choir.stream_seq_lease"

type streamSeqLeaseCapabilityKey struct{}

// WithStreamSeqLeaseCapability returns a context that admits a write of
// KindStreamSeqLease. Its only non-test caller is
// internal/store/stream_seq_lease.go (a source guard pins it). The key is
// unexported, so nothing outside this package can forge the capability.
func WithStreamSeqLeaseCapability(ctx context.Context) context.Context {
	return context.WithValue(ctx, streamSeqLeaseCapabilityKey{}, true)
}

func hasStreamSeqLeaseCapability(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	ok, _ := ctx.Value(streamSeqLeaseCapabilityKey{}).(bool)
	return ok
}

// checkStreamSeqLeaseFence refuses any non-tombstone lease object unless ctx
// carries the lease capability. A tombstone passes, as in CheckWrite.
func checkStreamSeqLeaseFence(ctx context.Context, objects []Object) error {
	for _, obj := range objects {
		if obj.ObjectKind != KindStreamSeqLease || obj.Tombstone {
			continue
		}
		if !hasStreamSeqLeaseCapability(ctx) {
			return fmt.Errorf("%w: object %s", ErrLeaseFence, obj.CanonicalID)
		}
	}
	return nil
}
