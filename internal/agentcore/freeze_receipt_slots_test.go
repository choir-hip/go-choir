package agentcore

// Freeze's distinct-receipts refusal must name the slots that share a
// receipt (docs/archive/problems/selfdev-freeze-receipts-refused-2026-10-15.md).
// In drill selfdev-ab-2239 the desk passed the same receipt as
// build_recipe_ref and test_receipts[0] and was told only "distinct build,
// test, and dependency/toolchain execution receipts are required"; it
// never retried Freeze.
//
// Ways the diagnosis could fail, written before the fix:
//
//  1. It names no slot, or the wrong slots, for the drill's shape
//     (build == test[0], toolchain distinct).
//     -> TestFreezeReceiptSlotsNameTheDrillCollision
//  2. It reports one pair when all three slots share one receipt, so the
//     desk fixes one slot and is refused again.
//     -> TestFreezeReceiptSlotsNameEveryCollision
//  3. It refuses a shape the executor accepts: a duplicate inside one slot
//     while the three slots still hold three distinct receipts.
//     -> TestFreezeReceiptSlotsAcceptThreeDistinct
//  4. It drops the original refusal text, which operators and the s0
//     problem doc grep for.
//     -> TestFreezeReceiptSlotsNameTheDrillCollision (prefix)
//  5. It does not say what each slot should cite, so the desk guesses
//     again. -> TestFreezeReceiptSlotsNameTheDrillCollision (guidance)

import (
	"strings"
	"testing"
)

const (
	slotRefA = "capsule-go-eval:sha256:ae4ae002fd358ed110244a98cf49a9aff51951262f60e091d9d34c288123f4e9"
	slotRefB = "capsule-go-eval:sha256:ffc67f04192a81cd8f07455c0a971779a3c8d46eae285ad49e031d57df83ec9b"
	slotRefC = "capsule-go-eval:sha256:10cf753478a3d94e489cbec22383a1d59d14f54a25a814db7b3a9976c43c71fc"
)

func TestFreezeReceiptSlotsNameTheDrillCollision(t *testing.T) {
	err := freezeReceiptSlotsError(slotRefA, []string{slotRefA}, []string{slotRefB})
	if err == nil {
		t.Fatal("two distinct receipts for three slots were accepted")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "distinct build, test, and dependency/toolchain execution receipts are required") {
		t.Fatalf("refusal lost its original text: %q", msg)
	}
	if !strings.Contains(msg, "build_recipe_ref and test_receipts[0] cite the same receipt "+slotRefA) {
		t.Fatalf("refusal does not name the colliding slots and receipt: %q", msg)
	}
	if strings.Contains(msg, "dependency_toolchain_refs[0] and") || strings.Contains(msg, "and dependency_toolchain_refs[0]") {
		t.Fatalf("refusal blames the distinct toolchain slot: %q", msg)
	}
	for _, want := range []string{"three different cells", "build_recipe_ref:", "test_receipts:", "dependency_toolchain_refs:"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not say %q: %q", want, msg)
		}
	}
}

func TestFreezeReceiptSlotsNameEveryCollision(t *testing.T) {
	err := freezeReceiptSlotsError(slotRefA, []string{slotRefA}, []string{slotRefA})
	if err == nil {
		t.Fatal("one receipt for three slots was accepted")
	}
	if !strings.Contains(err.Error(), "build_recipe_ref, test_receipts[0] and dependency_toolchain_refs[0] cite the same receipt "+slotRefA) {
		t.Fatalf("refusal does not name all three slots: %q", err)
	}

	err = freezeReceiptSlotsError(slotRefA, []string{slotRefB, slotRefB}, []string{slotRefA})
	if err == nil {
		t.Fatal("two distinct receipts for three slots were accepted")
	}
	for _, want := range []string{
		"build_recipe_ref and dependency_toolchain_refs[0] cite the same receipt " + slotRefA,
		"test_receipts[0] and test_receipts[1] cite the same receipt " + slotRefB,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q: %q", want, err)
		}
	}
}

func TestFreezeReceiptSlotsAcceptThreeDistinct(t *testing.T) {
	for name, slots := range map[string][3][]string{
		"one each":              {{slotRefA}, {slotRefB}, {slotRefC}},
		"duplicate inside test": {{slotRefA}, {slotRefB, slotRefB}, {slotRefC}},
		"build also a test":     {{slotRefA}, {slotRefA, slotRefB}, {slotRefC}},
	} {
		if err := freezeReceiptSlotsError(slots[0][0], slots[1], slots[2]); err != nil {
			t.Errorf("%s: three distinct receipts refused: %v", name, err)
		}
	}
}
