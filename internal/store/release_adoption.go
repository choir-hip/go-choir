package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ogKindReleaseAdoption is one owner-requested source adoption of a
// published release on this computer (P8,
// docs/archive/reports/p6-p8-release-adopt-2026-10-15.md). It is written once, after
// every verification passed and the request reached the computer's desks;
// the projection interceptor puts it on the computer's tape. Its status is
// derived from the self-development operation on its trajectory, never
// stored twice.
const ogKindReleaseAdoption = objectgraph.ObjectKind("choir.release_adoption")

// ReleaseAdoption is what the owner asked this computer to adopt and what
// the computer verified before asking its desks.
type ReleaseAdoption struct {
	AdoptionID           string    `json:"adoption_id"`
	OwnerID              string    `json:"owner_id"`
	ComputerID           string    `json:"computer_id"`
	ReleaseID            string    `json:"release_id"`
	PublisherComputerID  string    `json:"publisher_computer_id"`
	PublisherOperationID string    `json:"publisher_operation_id"`
	PatchSHA256          string    `json:"patch_sha256"`
	ReleaseBaseCommit    string    `json:"release_base_commit"`
	ReleaseTreeHash      string    `json:"release_tree_hash"`
	AdopterBaseCommit    string    `json:"adopter_base_commit"`
	AdopterTreeHash      string    `json:"adopter_tree_hash"`
	SameBase             bool      `json:"same_base"`
	TestCommand          string    `json:"test_command,omitempty"`
	StagedDir            string    `json:"staged_dir"`
	TrajectoryID         string    `json:"trajectory_id"`
	DocID                string    `json:"doc_id,omitempty"`
	SubmissionID         string    `json:"submission_id,omitempty"`
	RequestedAt          time.Time `json:"requested_at"`
}

func releaseAdoptionKey(computerID, adoptionID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(adoptionID)
}

// PutReleaseAdoption records an adoption. It never replaces one: an adoption
// is written once (a repeat request reads the first).
func (s *Store) PutReleaseAdoption(ctx context.Context, adoption ReleaseAdoption) error {
	if strings.TrimSpace(adoption.OwnerID) == "" || strings.TrimSpace(adoption.ComputerID) == "" || strings.TrimSpace(adoption.AdoptionID) == "" ||
		strings.TrimSpace(adoption.ReleaseID) == "" || strings.TrimSpace(adoption.TrajectoryID) == "" {
		return fmt.Errorf("release adoption: owner, computer, adoption, release and trajectory are required")
	}
	if _, err := s.GetReleaseAdoption(ctx, adoption.OwnerID, adoption.ComputerID, adoption.AdoptionID); err == nil {
		return fmt.Errorf("release adoption %s already recorded", adoption.AdoptionID)
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if adoption.RequestedAt.IsZero() {
		adoption.RequestedAt = time.Now().UTC()
	}
	metadata := map[string]any{
		"computer_id":   adoption.ComputerID,
		"adoption_id":   adoption.AdoptionID,
		"release_id":    adoption.ReleaseID,
		"trajectory_id": adoption.TrajectoryID,
		"requested_at":  adoption.RequestedAt.UTC().Format(time.RFC3339Nano),
	}
	_, err := s.ogPut(ctx, ogKindReleaseAdoption, adoption.OwnerID, releaseAdoptionKey(adoption.ComputerID, adoption.AdoptionID), adoption, metadata, adoption.RequestedAt)
	return err
}

// GetReleaseAdoption returns one adoption, or ErrNotFound.
func (s *Store) GetReleaseAdoption(ctx context.Context, ownerID, computerID, adoptionID string) (ReleaseAdoption, error) {
	if s.og == nil {
		return ReleaseAdoption{}, fmt.Errorf("store: object graph not initialized")
	}
	id, err := objectgraph.BuildCanonicalID(ogKindReleaseAdoption, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(releaseAdoptionKey(computerID, adoptionID)))
	if err != nil {
		return ReleaseAdoption{}, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return ReleaseAdoption{}, ErrNotFound
	}
	if err != nil {
		return ReleaseAdoption{}, err
	}
	var adoption ReleaseAdoption
	if err := ogDecode(obj, &adoption); err != nil {
		return ReleaseAdoption{}, err
	}
	return adoption, nil
}

// ListReleaseAdoptions returns the computer's adoptions, newest first.
func (s *Store) ListReleaseAdoptions(ctx context.Context, ownerID, computerID string) ([]ReleaseAdoption, error) {
	return s.listReleaseAdoptions(ctx, ownerID, []objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}})
}

// ReleaseAdoptionForTrajectory returns the adoption whose request opened the
// trajectory, or ErrNotFound.
func (s *Store) ReleaseAdoptionForTrajectory(ctx context.Context, ownerID, computerID, trajectoryID string) (ReleaseAdoption, error) {
	if strings.TrimSpace(trajectoryID) == "" {
		return ReleaseAdoption{}, ErrNotFound
	}
	list, err := s.listReleaseAdoptions(ctx, ownerID, []objectgraph.JSONFieldMatch{
		{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)},
		{JSONPath: "$.trajectory_id", Value: strings.TrimSpace(trajectoryID)},
	})
	if err != nil {
		return ReleaseAdoption{}, err
	}
	if len(list) == 0 {
		return ReleaseAdoption{}, ErrNotFound
	}
	return list[0], nil
}

func (s *Store) listReleaseAdoptions(ctx context.Context, ownerID string, matches []objectgraph.JSONFieldMatch) ([]ReleaseAdoption, error) {
	objs, err := s.ogListByOwnerAndBody(ctx, ogKindReleaseAdoption, strings.TrimSpace(ownerID), matches, 200)
	if err != nil {
		return nil, err
	}
	out := make([]ReleaseAdoption, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone {
			continue
		}
		var adoption ReleaseAdoption
		if err := ogDecode(obj, &adoption); err != nil {
			return nil, err
		}
		out = append(out, adoption)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}
