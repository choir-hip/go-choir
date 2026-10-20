package capsule

import "fmt"

// receiptSuggestMinShared is how many characters after the kind prefix an
// issued ref must share with an unknown one to be suggested: a third of the
// 64-digit digest, long enough that a guess is a typo, not a coincidence.
const receiptSuggestMinShared = 20

// unknownReceiptError refuses a receipt ref this executor did not issue. It
// names the ref, so a desk that retyped it can see the mistake, and the issued
// ref of the same kind sharing the longest prefix, when that prefix is long.
func unknownReceiptError(ref string, issued []string) error {
	best, bestShared := "", 0
	for _, cand := range issued {
		kind := receiptKindPrefix(cand)
		if kind == "" || kind != receiptKindPrefix(ref) {
			continue
		}
		shared := 0
		for shared < len(ref) && shared < len(cand) && ref[shared] == cand[shared] {
			shared++
		}
		if n := shared - len(kind); n >= receiptSuggestMinShared && shared > bestShared {
			best, bestShared = cand, shared
		}
	}
	if best != "" {
		return fmt.Errorf("executor receipt unavailable: %s is not a receipt this computer issued; did you mean %s? Copy receipt_ref exactly from the cell result", ref, best)
	}
	return fmt.Errorf("executor receipt unavailable: %s is not a receipt this computer issued; copy receipt_ref exactly from the cell result", ref)
}

func receiptKindPrefix(ref string) string {
	for _, p := range []string{"capsule-go-eval:sha256:", "capsule-exec:sha256:"} {
		if len(ref) >= len(p) && ref[:len(p)] == p {
			return p
		}
	}
	return ""
}
