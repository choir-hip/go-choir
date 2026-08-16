package selfdevprotocol

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateTapeCompletenessIncompleteOmitsBoundary(t *testing.T) {
	if err := ValidateTapeCompleteness("", ""); err != nil {
		t.Fatalf("incomplete tape refused: %v", err)
	}
	if err := ValidateTapeCompleteness("incomplete", ""); err != nil {
		t.Fatalf("explicit incomplete tape refused: %v", err)
	}
	if err := ValidateTapeCompleteness("", strings.Repeat("a", 64)); err == nil {
		t.Fatal("incomplete tape accepted a complete_from_head")
	}
}

func TestValidateTapeCompletenessCompleteFromHeadRequiresDigest(t *testing.T) {
	head := strings.Repeat("b", 64)
	if err := ValidateTapeCompleteness(TapeCompletenessCompleteFromHead, head); err != nil {
		t.Fatalf("complete_from_head refused: %v", err)
	}
	if err := ValidateTapeCompleteness(TapeCompletenessCompleteFromHead, ""); err == nil {
		t.Fatal("complete-tape checkpoint omitted complete_from_head")
	}
	if !errors.Is(ValidateTapeCompleteness(TapeCompletenessCompleteFromHead, "not-a-digest"), ErrCompleteFromHeadRequired) {
		t.Fatal("non-digest complete_from_head was not ErrCompleteFromHeadRequired")
	}
}

func TestValidateTapeCompletenessRefusesNewEpoch(t *testing.T) {
	if !errors.Is(ValidateTapeCompleteness(RecoveryDomainNewEpoch, ""), ErrNewEpochRefused) {
		t.Fatal("new_epoch was not refused")
	}
}

func TestAdmitRestoreSequenceFailsClosedBeforeCompleteness(t *testing.T) {
	if err := AdmitRestoreSequence(26, 27, false); err != nil {
		t.Fatalf("undeclared completeness refused sequence 26: %v", err)
	}
	if !errors.Is(AdmitRestoreSequence(26, 27, true), ErrIncompleteTapeRestore) {
		t.Fatal("pre-completeness full-computer restore was admitted")
	}
	if err := AdmitRestoreSequence(27, 27, true); err != nil {
		t.Fatalf("completeness head refused: %v", err)
	}
	if err := AdmitRestoreSequence(40, 27, true); err != nil {
		t.Fatalf("post-completeness head refused: %v", err)
	}
}

func TestCheckpointFromRequestRefusesNewEpoch(t *testing.T) {
	request := testCheckpointRequest(t)
	request.TapeCompleteness = RecoveryDomainNewEpoch
	if _, _, err := CheckpointFromRequest(request); !errors.Is(err, ErrNewEpochRefused) {
		t.Fatalf("checkpoint new_epoch error=%v", err)
	}
}

func TestCheckpointFromRequestKeepsIncompleteTapeDigestStable(t *testing.T) {
	request := testCheckpointRequest(t)
	first, _, err := CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	request.TapeCompleteness = ""
	request.CompleteFromHead = ""
	second, _, err := CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("empty tape_completeness changed checkpoint digest")
	}
}

func TestRestoreFromRequestAdmitsIncompleteTapeAndCompleteFromHead(t *testing.T) {
	checkpoint, _, err := CheckpointFromRequest(testCheckpointRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	valid := RestoreRequest{
		ComputerID: checkpoint.Request.ComputerID, CheckpointDigest: checkpoint.Digest,
		OperandScopes: []string{RestoreScopeVMLocal, RestoreScopeFrontend},
	}
	if err := RestoreFromRequest(valid, checkpoint); err != nil {
		t.Fatalf("incomplete-tape restore refused: %v", err)
	}

	complete := testCheckpointRequest(t)
	complete.TapeCompleteness = TapeCompletenessCompleteFromHead
	complete.CompleteFromHead = strings.Repeat("c", 64)
	completeCheckpoint, _, err := CheckpointFromRequest(complete)
	if err != nil {
		t.Fatal(err)
	}
	completeRestore := RestoreRequest{
		ComputerID: completeCheckpoint.Request.ComputerID, CheckpointDigest: completeCheckpoint.Digest,
		OperandScopes: []string{RestoreScopeVMLocal, RestoreScopeFrontend},
	}
	if err := RestoreFromRequest(completeRestore, completeCheckpoint); err != nil {
		t.Fatalf("complete_from_head restore refused: %v", err)
	}
}
