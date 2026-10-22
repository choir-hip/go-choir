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
// receipt is the only authority for which computer needs a payload or an
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

// parseReceipt decodes a legacy pin receipt. ok is false for anything that
// is not a PinReceipt issued by corpusd naming a valid artifact digest.
func parseReceipt(raw []byte) (e edge, ok bool) {
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil || receipt.ReceiptKind != "PinReceipt" || receipt.Issuer != "corpusd" {
		return e, false
	}
	computer, _ := receipt.KindFields["computer_id"].(string)
	artifact, _ := receipt.KindFields["artifact_digest"].(string)
	namespace, _ := receipt.KindFields["pin_namespace"].(string)
	mediaType, _ := receipt.KindFields["media_type"].(string)
	lengthNum, _ := receipt.KindFields["length"].(json.Number)
	length, lengthErr := lengthNum.Int64()
	if !tapestore.ValidDigest(artifact) || lengthErr != nil {
		return e, false
	}
	if _, err := hex.Decode(e.artifact[:], []byte(artifact)); err != nil {
		return e, false
	}
	e.computer, e.length = computer, length
	switch {
	case namespace == eventNamespace:
		e.ns, e.eligible = tapestore.NSEvent, true
	case namespace == payloadNamespace:
		e.ns, e.eligible = tapestore.NSPayload, mediaType == computerevent.ProjectionBatchMediaType
	default:
		return e, false
	}
	if !tapestore.ValidComputerID(computer) || length <= 0 || length > tapestore.MaxRecordBytes {
		e.eligible = false
	}
	return e, true
}
