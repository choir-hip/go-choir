package types

import (
	"fmt"
	"time"
)

const (
	// InstallReviewDueWakeKind is the owed review's re-entry wake: the clock
	// of an owner or update install, minted by the owed record's own commit.
	InstallReviewDueWakeKind = "install_review_due"
	// InstallReviewOwedNoticeAfter is how long after the record, or the last
	// notice, a re-entry may send the next notice; InstallReviewOwedDeadline
	// is when the obligation closes from the record.
	InstallReviewOwedNoticeAfter = 3 * time.Minute
	InstallReviewOwedDeadline    = 45 * time.Minute

	InstallReviewStageTick     = "tick"
	InstallReviewStageDeadline = "deadline"
)

// InstallReviewDueWake is an install_review_due wake's content. Its identity
// is derived from the content, so every stage and notice is its own wake.
type InstallReviewDueWake struct {
	ReleaseID     string `json:"release_id"`
	SubjectDigest string `json:"subject_digest"`
	Attempt       int    `json:"attempt"`
	Stage         string `json:"stage"`
	// Notice is the number of notices already sent when a tick was minted.
	Notice int `json:"notice"`
}

// InstallReviewDueWakes lists the re-entry wakes a committed owed record or
// notice of an install-path review owes, each with its not-before time: the
// record owes a tick after InstallReviewOwedNoticeAfter and the deadline;
// notice n below the cap owes the next tick. The store mints them in the
// record's own transaction (actorWakeOutboxFromObject).
func InstallReviewDueWakes(rec CommitmentRecord, committedAt time.Time) ([]InstallReviewDueWake, []time.Time) {
	if !IsInstallReviewOwedRecord(rec) || rec.SubjectRefs["path"] != InstallReviewPathInstall {
		return nil, nil
	}
	var attempt, notice int
	_, _ = fmt.Sscan(rec.SubjectRefs["attempt"], &attempt)
	_, _ = fmt.Sscan(rec.SubjectRefs["notice"], &notice)
	base := InstallReviewDueWake{ReleaseID: rec.SubjectRefs["release_id"], SubjectDigest: rec.SubjectRefs["subject_digest"], Attempt: attempt}
	var wakes []InstallReviewDueWake
	var at []time.Time
	add := func(stage string, n int, when time.Time) {
		w := base
		w.Stage, w.Notice = stage, n
		wakes, at = append(wakes, w), append(at, when)
	}
	if notice == 0 {
		add(InstallReviewStageTick, 0, committedAt.Add(InstallReviewOwedNoticeAfter))
		add(InstallReviewStageDeadline, 0, committedAt.Add(InstallReviewOwedDeadline))
	} else if notice < InstallReviewOwedMaxNotices {
		add(InstallReviewStageTick, notice, committedAt.Add(InstallReviewOwedNoticeAfter))
	}
	return wakes, at
}
