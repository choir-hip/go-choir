package taperepack

import (
	"encoding/hex"
	"encoding/json"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

const (
	eventNamespace   = "computer-event"
	payloadNamespace = "computer-event-payload"
	receiptNamespace = "pin-receipts"

	// maxReceiptBytes bounds a pin receipt read; receipts are about 1 KiB.
	maxReceiptBytes = 1 << 20
)

// edge is what a signed pin receipt says: computer pinned artifact. The
// receipt is the only authority (as a shape check: the signature is not
// verified here, the host has no platform key; the tape on the host disk is
// the trust boundary, review round 1 F4) for which computer needs a payload or an
// event in its segments; the artifact's own bytes can name any computer.
type edge struct {
	computer string
	ns       tapestore.NS // NSEvent or NSPayload
	artifact [32]byte
	length   int64
	// eligible follows the production pin path (internal/platform
	// event_artifacts.go): an event, or a payload whose media type is the
	// projection batch type, pinned for a computer that can name a segment
	// directory, with a length a record can hold.
	eligible bool
}

type receiptStatus int

const (
	// receiptForeign: valid JSON that is not a PinReceipt; it names nothing
	// and is ignored.
	receiptForeign receiptStatus = iota
	// receiptOK: a well formed pin receipt. Its edge may still be ineligible.
	receiptOK
	// receiptRejected: a PinReceipt that names an artifact but fails the
	// shape guard (issuer, signature set, version, namespace, length). It
	// cannot be trusted to attribute, but it must not remove a requirement:
	// the files it names are refused (review round 2, R3).
	receiptRejected
	// receiptUnnamable: something that claims to be a receipt but names no
	// artifact (not JSON, no valid digest). No file can be proven free of it.
	receiptUnnamable
)

// classifyReceipt decodes a pin receipt.
func classifyReceipt(raw []byte) (e edge, st receiptStatus) {
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return e, receiptUnnamable
	}
	if receipt.ReceiptKind != "PinReceipt" {
		return e, receiptForeign
	}
	computer, _ := receipt.KindFields["computer_id"].(string)
	artifact, _ := receipt.KindFields["artifact_digest"].(string)
	namespace, _ := receipt.KindFields["pin_namespace"].(string)
	mediaType, _ := receipt.KindFields["media_type"].(string)
	lengthNum, _ := receipt.KindFields["length"].(json.Number)
	length, lengthErr := lengthNum.Int64()
	if !tapestore.ValidDigest(artifact) {
		return e, receiptUnnamable
	}
	if _, err := hex.Decode(e.artifact[:], []byte(artifact)); err != nil {
		return e, receiptUnnamable
	}
	e.computer, e.length = computer, length
	if receipt.Issuer != "corpusd" || len(receipt.SignatureSet) == 0 || receipt.ReceiptVersion != computerevent.ReceiptVersionV1 || lengthErr != nil {
		return e, receiptRejected
	}
	switch {
	case namespace == eventNamespace:
		e.ns, e.eligible = tapestore.NSEvent, true
	case namespace == payloadNamespace:
		e.ns, e.eligible = tapestore.NSPayload, mediaType == computerevent.ProjectionBatchMediaType
	default:
		return e, receiptRejected
	}
	if !tapestore.ValidComputerID(computer) || length <= 0 || length > tapestore.MaxRecordBytes {
		e.eligible = false
	}
	return e, receiptOK
}

// parseReceipt decodes a legacy pin receipt. ok is false for anything that
// is not a well formed PinReceipt issued by corpusd naming a valid artifact.
func parseReceipt(raw []byte) (e edge, ok bool) {
	e, st := classifyReceipt(raw)
	return e, st == receiptOK
}

// hexRefs returns the distinct 64-character lowercase hex tokens in b: the
// digests an event body names (payload refs, parents). The sweep keeps every
// file the chain names this way, and its regexp matches inside a longer run
// of hex, so every 64-character window of a run is a reference here (more
// requirements, never fewer; review round 3, L5).
func hexRefs(b []byte) [][32]byte {
	var out [][32]byte
	seen := map[[32]byte]bool{}
	start := -1
	flush := func(end int) {
		if start >= 0 {
			for i := start; i+64 <= end; i++ {
				var d [32]byte
				if _, err := hex.Decode(d[:], b[i:i+64]); err == nil && !seen[d] {
					seen[d] = true
					out = append(out, d)
				}
			}
		}
		start = -1
	}
	for i, c := range b {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(b))
	return out
}
