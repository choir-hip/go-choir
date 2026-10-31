package bundle

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The tape scan of VerifyBundle is a pipeline. A reader cuts receipts.jsonl
// into batches. Workers check each row on its own: decode and bind the
// event, parse and bind the head and pin receipts, hash the payloads and
// check every signature. One consumer, in sequence order, checks only what
// depends on order: sequence continuity, the previous-head chain, genesis,
// the snapshot's naming event, and the unique-object counts. At most
// 2 x workers batches are in flight, so memory is bounded by the batch size
// and the worker count, never by the tape.

const scanBatch = 256

// rowCheck is what a worker proves about one receipt row.
type rowCheck struct {
	sequence     int64
	computerID   string
	eventDigest  string
	previousHead string
	kind         computerevent.EventKind
	idempotency  string
	reducer      int
	payloads     []string
	pins         []string
	head         computerevent.Receipt
	signatures   int64
	err          error
}

type scanBatchJob struct {
	lines [][]byte
	out   chan []rowCheck
}

type scanResult struct {
	got          Totals
	signatures   int64
	last         *rowCheck
	revocations  uint64
	snapNamedAt  int64
	end          int64
	endDigest    string
	receiptsHash string
}

func scanTape(dir string, m Manifest, objs objects, trust *TrustSet) (scanResult, error) {
	var res scanResult
	f, err := os.Open(filepath.Join(dir, ReceiptsName))
	if err != nil {
		return res, fmt.Errorf("bundle: open receipts: %w", err)
	}
	defer f.Close()
	fileHash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(f, fileHash))
	scanner.Buffer(make([]byte, 64<<10), maxReceiptLine)

	workers := max(runtime.GOMAXPROCS(0), 1)
	jobs := make(chan scanBatchJob, workers)
	order := make(chan scanBatchJob, 2*workers)
	done := make(chan struct{})
	var readErr error
	go func() {
		defer close(order)
		defer close(jobs)
		b := scanBatchJob{out: make(chan []rowCheck, 1)}
		send := func() bool {
			select {
			case order <- b:
			case <-done:
				return false
			}
			select {
			case jobs <- b:
			case <-done:
				return false
			}
			b = scanBatchJob{out: make(chan []rowCheck, 1)}
			return true
		}
		for scanner.Scan() {
			b.lines = append(b.lines, append([]byte(nil), scanner.Bytes()...))
			if len(b.lines) == scanBatch && !send() {
				return
			}
		}
		readErr = scanner.Err()
		if len(b.lines) > 0 {
			send()
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range jobs {
				out := make([]rowCheck, len(b.lines))
				for i, line := range b.lines {
					select {
					case <-done:
						out[i].err = fmt.Errorf("bundle: scan stopped")
					default:
						out[i] = checkRow(line, m.ComputerID, objs, trust)
					}
				}
				b.out <- out
			}
		}()
	}
	stop := func() {
		close(done)
		for range order { // let the reader exit
		}
		wg.Wait()
	}

	prev, seq := m.Tape.StartHead, m.Tape.StartSeq
	seenPayload, seenPin := newSeen(objs), newSeen(objs)
	snapKey := ""
	if m.SnapshotRoot != nil {
		snapKey = "snapshot:" + m.SnapshotRoot.Root
	}
	var last rowCheck
	for b := range order {
		for _, r := range <-b.out {
			if err := r.inOrder(m, prev, seq); err != nil {
				stop()
				return res, err
			}
			if r.kind == computerevent.EventKeyRevoked {
				res.revocations++
			}
			if snapKey != "" && res.snapNamedAt == 0 && r.kind == computerevent.EventSnapshotRecorded && r.idempotency == snapKey && r.sequence > m.SnapshotRoot.HeadSequence {
				res.snapNamedAt = r.sequence
			}
			res.got.Events++
			res.got.Receipts++
			for _, d := range r.payloads {
				if !seenPayload.has(PayloadsDir, d) {
					seenPayload.add(PayloadsDir, d)
					res.got.Payloads++
				}
			}
			for _, d := range r.pins {
				if !seenPin.has(PinsDir, d) {
					seenPin.add(PinsDir, d)
					res.got.PinReceipts++
				}
			}
			res.signatures += r.signatures
			prev, seq = r.eventDigest, r.sequence
			r.payloads, r.pins = nil, nil
			last = r
			res.last = &last
		}
	}
	wg.Wait()
	if readErr != nil {
		return res, fmt.Errorf("bundle: read receipts: %w", readErr)
	}
	res.end, res.endDigest = seq, prev
	res.receiptsHash = hex.EncodeToString(fileHash.Sum(nil))
	return res, nil
}

// inOrder checks what depends on the row's place in the tape.
func (r rowCheck) inOrder(m Manifest, prev string, seq int64) error {
	if r.err != nil {
		return r.err
	}
	if r.sequence != seq+1 {
		return unauthentic("receipt after %d is out of sequence or scope (sequence %d of %q)", seq, r.sequence, r.computerID)
	}
	if r.previousHead != prev {
		return unauthentic("event %d does not chain to %s", r.sequence, prev)
	}
	if r.sequence == 1 {
		if r.kind != computerevent.EventGenesisImported || r.previousHead != computerevent.ZeroHead {
			return unauthentic("event 1 is %s, not genesis_imported", r.kind)
		}
		if m.Genesis != "" && m.Genesis != r.eventDigest {
			return unauthentic("the manifest names genesis %s, the tape's event 1 is %s", m.Genesis, r.eventDigest)
		}
	} else if r.kind == computerevent.EventGenesisImported {
		return unauthentic("event %d is a second genesis", r.sequence)
	}
	return nil
}

// checkRow proves everything about one receipt line that does not depend on
// its neighbours. Its error is reported when the consumer reaches the row,
// so the failure named is the first in tape order.
func checkRow(line []byte, computerID string, objs objects, trust *TrustSet) rowCheck {
	var row ReceiptRow
	if err := json.Unmarshal(line, &row); err != nil {
		return rowCheck{err: unauthentic("receipt line: %v", err)}
	}
	r := rowCheck{sequence: row.Sequence, computerID: row.ComputerID, eventDigest: row.EventDigest}
	fail := func(err error) rowCheck { r.err = err; return r }
	if row.ComputerID != computerID {
		return fail(unauthentic("receipt %d is of %q, not %s", row.Sequence, row.ComputerID, computerID))
	}
	if !validDigest(row.EventDigest) || !validDigest(row.EventPinReceiptDigest) {
		return fail(unauthentic("receipt %d has an invalid event or pin digest", row.Sequence))
	}
	raw, err := objs.get(EventsDir, row.EventDigest)
	if err != nil {
		return fail(unauthentic("event %d: %v", row.Sequence, err))
	}
	event, err := decodeEvent(raw, row.EventDigest)
	if err != nil {
		return fail(fmt.Errorf("%w: %w", ErrUnauthentic, err))
	}
	if event.ComputerID != computerID || int64(event.Sequence) != row.Sequence || string(event.EventKind) != row.EventKind || event.IdempotencyKey != row.IdempotencyKey || event.RequestCommitment != row.RequestCommitment {
		return fail(unauthentic("event %d does not match its append receipt", row.Sequence))
	}
	r.previousHead, r.kind, r.idempotency, r.reducer = event.PreviousHead, event.EventKind, event.IdempotencyKey, event.ReducerVersion
	receipt, pins, err := headReceipt(row)
	if err != nil {
		return fail(fmt.Errorf("%w: %w", ErrUnauthentic, err))
	}
	if receipt.ReceiptKind != "EventHeadReceipt" || receipt.Issuer != ReceiptIssuer || receipt.ReceiptID != row.EventHeadReceiptID ||
		receipt.KindFields["computer_id"] != computerID || receipt.KindFields["event_digest"] != row.EventDigest ||
		receipt.KindFields["previous_head"] != event.PreviousHead || fmt.Sprint(receipt.KindFields["sequence"]) != fmt.Sprint(row.Sequence) ||
		receipt.KindFields["request_commitment"] != row.RequestCommitment || receipt.KindFields["event_kind"] != row.EventKind {
		return fail(unauthentic("head receipt %d does not bind its event", row.Sequence))
	}
	if err := receipt.RequireKindFields(headReceiptFields...); err != nil {
		return fail(fmt.Errorf("%w: head receipt %d: %w", ErrUnauthentic, row.Sequence, err))
	}
	if err := receipt.Verify(trust); err != nil {
		return fail(fmt.Errorf("%w: head receipt %d: %w", ErrUnauthentic, row.Sequence, err))
	}
	r.signatures++
	payloads, err := eventPayloadDigests(event)
	if err != nil {
		return fail(fmt.Errorf("%w: %w", ErrUnauthentic, err))
	}
	if len(pins) != 1+len(payloads) {
		return fail(unauthentic("event %d names %d payloads and %d payload pins", row.Sequence, len(payloads), len(pins)-1))
	}
	for _, digest := range payloads {
		if _, err := objs.get(PayloadsDir, digest); err != nil {
			return fail(unauthentic("payload of event %d: %v", row.Sequence, err))
		}
	}
	at := sequenceResolver{trust: trust, sequence: uint64(row.Sequence)}
	for i, digest := range pins {
		want := row.EventDigest
		if i > 0 {
			want = payloads[i-1]
		}
		pin, err := checkPin(objs, digest, computerID, want)
		if err != nil {
			return fail(unauthentic("event %d: %v", row.Sequence, err))
		}
		if i == 0 {
			if rc, ok := pin.KindFields["request_commitment"]; ok && rc != row.RequestCommitment {
				return fail(unauthentic("event %d: its pin receipt binds another request commitment", row.Sequence))
			}
		}
		if err := pin.Verify(at); err != nil {
			return fail(fmt.Errorf("%w: pin receipt %s of event %d: %w", ErrUnauthentic, pin.ReceiptID, row.Sequence, err))
		}
		r.signatures++
	}
	r.payloads, r.pins, r.head = payloads, pins, receipt
	return r
}
