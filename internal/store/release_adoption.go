package store

import (
	"context"
	"encoding/json"
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
	AdoptionID           string `json:"adoption_id"`
	OwnerID              string `json:"owner_id"`
	ComputerID           string `json:"computer_id"`
	ReleaseID            string `json:"release_id"`
	PublisherComputerID  string `json:"publisher_computer_id"`
	PublisherOperationID string `json:"publisher_operation_id"`
	PatchSHA256          string `json:"patch_sha256"`
	ReleaseBaseCommit    string `json:"release_base_commit"`
	ReleaseTreeHash      string `json:"release_tree_hash"`
	AdopterBaseCommit    string `json:"adopter_base_commit"`
	AdopterTreeHash      string `json:"adopter_tree_hash"`
	SameBase             bool   `json:"same_base,omitempty"`
	TestCommand          string `json:"test_command,omitempty"`
	// StagedDir is never stored or served: a package install's staging is
	// <updater root>/adoptions/<install id> (stagedInstallDir), so the
	// updater path does not reach the install views.
	StagedDir    string    `json:"-"`
	TrajectoryID string    `json:"trajectory_id"`
	DocID        string    `json:"doc_id,omitempty"`
	SubmissionID string    `json:"submission_id,omitempty"`
	RequestedAt  time.Time `json:"requested_at"`
	// Package install (docs/design/package-install-2026-11-04.md): Kind is
	// "package" for an install or uninstall of a package release (empty for
	// a P8 software release), Package its name, Action install or
	// uninstall, and Attempt counts the tries of one release and action on
	// this computer (a crash-closed try does not block the next).
	// ReleaseTreeHash is then the release's tree at the package root.
	Kind    string `json:"kind,omitempty"`
	Package string `json:"package,omitempty"`
	Action  string `json:"action,omitempty"`
	Attempt int    `json:"attempt,omitempty"`
	// Runtime install (S12, docs/design/s12-runtime-install-build-2026-11-04.md):
	// Mode is "runtime" for an install or uninstall that changed the
	// computer's installed package set, not its source; such a record is
	// written only after the change is done, and it has no trajectory.
	// ModuleSHA256 names the built module in the computer's module store,
	// RequiresSDK the shell SDK range it needs, and Manifest the package's
	// manifest bytes, from which the shell lists the app.
	Mode         string          `json:"mode,omitempty"`
	ModuleSHA256 string          `json:"module_sha256,omitempty"`
	ModuleABI    string          `json:"module_abi,omitempty"`
	RequiresSDK  string          `json:"requires_sdk,omitempty"`
	Manifest     json.RawMessage `json:"manifest,omitempty"`
}

// releaseAdoptionPage is the first page of an adoption listing.
var releaseAdoptionPage = 200

// ReleaseAdoptionModeRuntime marks a runtime install record (S12).
const ReleaseAdoptionModeRuntime = "runtime"

func releaseAdoptionKey(computerID, adoptionID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(adoptionID)
}

// PutReleaseAdoption records an adoption. It never replaces one: an adoption
// is written once (a repeat request reads the first).
func (s *Store) PutReleaseAdoption(ctx context.Context, adoption ReleaseAdoption) error {
	if strings.TrimSpace(adoption.OwnerID) == "" || strings.TrimSpace(adoption.ComputerID) == "" || strings.TrimSpace(adoption.AdoptionID) == "" ||
		strings.TrimSpace(adoption.ReleaseID) == "" {
		return fmt.Errorf("release adoption: owner, computer, adoption and release are required")
	}
	if adoption.Mode != ReleaseAdoptionModeRuntime && strings.TrimSpace(adoption.TrajectoryID) == "" {
		return fmt.Errorf("release adoption: a trajectory is required")
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
// It returns every one: the installed set is a fold of the whole history,
// so a window would drop a package that is still installed.
func (s *Store) ListReleaseAdoptions(ctx context.Context, ownerID, computerID string) ([]ReleaseAdoption, error) {
	return s.listReleaseAdoptions(ctx, ownerID, []objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}})
}

func (s *Store) listReleaseAdoptions(ctx context.Context, ownerID string, matches []objectgraph.JSONFieldMatch) ([]ReleaseAdoption, error) {
	// The list has no cursor, so the limit grows until a page comes back
	// short: the whole set, however long.
	limit := releaseAdoptionPage
	var objs []objectgraph.Object
	for {
		page, err := s.ogListByOwnerAndBody(ctx, ogKindReleaseAdoption, strings.TrimSpace(ownerID), matches, limit)
		if err != nil {
			return nil, err
		}
		objs = page
		if len(page) < limit {
			break
		}
		limit *= 4
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
