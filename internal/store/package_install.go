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

// ogKindPackageInstall is one install or uninstall of a Library package
// release on this computer (docs/design/package-install-2026-11-04.md,
// docs/design/forced-modularity-build-2026-11-04.md M1). It is written once,
// after the change is done; the projection interceptor puts it on the
// computer's tape. Before MA2.4 the kind was named for the deleted release
// flow; records of that kind are read through legacyInstallKind
// (package_install_legacy.go) and converted to this kind.
const ogKindPackageInstall = objectgraph.ObjectKind("choir.package_install")

// PackageInstall is an install or uninstall of a package release and what the
// computer verified before doing it.
type PackageInstall struct {
	InstallID           string `json:"install_id"`
	OwnerID             string `json:"owner_id"`
	ComputerID          string `json:"computer_id"`
	ReleaseID           string `json:"release_id"`
	PublisherComputerID string `json:"publisher_computer_id"`
	// PublisherTrack is the owner id of the track a runtime install came from.
	PublisherTrack       string    `json:"publisher_track,omitempty"`
	PublisherOperationID string    `json:"publisher_operation_id"`
	PatchSHA256          string    `json:"patch_sha256"`
	ReleaseBaseCommit    string    `json:"release_base_commit"`
	ReleaseTreeHash      string    `json:"release_tree_hash"`
	TestCommand          string    `json:"test_command,omitempty"`
	TrajectoryID         string    `json:"trajectory_id"`
	DocID                string    `json:"doc_id,omitempty"`
	SubmissionID         string    `json:"submission_id,omitempty"`
	RequestedAt          time.Time `json:"requested_at"`
	// Package install (docs/design/package-install-2026-11-04.md): Kind is
	// "package" for an install or uninstall of a package release, Package
	// its name, Action install or uninstall, and Attempt counts the tries of
	// one release and action on this computer (a crash-closed try does not
	// block the next). ReleaseTreeHash is then the release's tree at the
	// package root.
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
	// Files are the data files the install carries (a theme pack's themes),
	// each in the computer's module store by digest. A package with no app (a
	// pack) has files or nothing and no ModuleSHA256.
	Files []PackageInstallFile `json:"files,omitempty"`
}

// PackageInstallFile is one data file of an install: its path in the package
// and its sha256.
type PackageInstallFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// packageInstallPage is the first page of an install listing.
var packageInstallPage = 200

// PackageInstallModeRuntime marks a runtime install record (S12).
const PackageInstallModeRuntime = "runtime"

func packageInstallKey(computerID, installID string) string {
	return strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(installID)
}

// PutPackageInstall records an install. It never replaces one: an install is
// written once (a repeat request reads the first).
func (s *Store) PutPackageInstall(ctx context.Context, install PackageInstall) error {
	if strings.TrimSpace(install.OwnerID) == "" || strings.TrimSpace(install.ComputerID) == "" || strings.TrimSpace(install.InstallID) == "" ||
		strings.TrimSpace(install.ReleaseID) == "" {
		return fmt.Errorf("package install: owner, computer, install and release are required")
	}
	if install.Mode != PackageInstallModeRuntime && strings.TrimSpace(install.TrajectoryID) == "" {
		return fmt.Errorf("package install: a trajectory is required")
	}
	if _, err := s.GetPackageInstall(ctx, install.OwnerID, install.ComputerID, install.InstallID); err == nil {
		return fmt.Errorf("package install %s already recorded", install.InstallID)
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if install.RequestedAt.IsZero() {
		install.RequestedAt = time.Now().UTC()
	}
	_, err := s.ogPut(ctx, ogKindPackageInstall, install.OwnerID, packageInstallKey(install.ComputerID, install.InstallID), install, packageInstallMetadata(install), install.RequestedAt)
	return err
}

func packageInstallMetadata(install PackageInstall) map[string]any {
	return map[string]any{
		"computer_id":   install.ComputerID,
		"install_id":    install.InstallID,
		"release_id":    install.ReleaseID,
		"trajectory_id": install.TrajectoryID,
		"requested_at":  install.RequestedAt.UTC().Format(time.RFC3339Nano),
	}
}

// GetPackageInstall returns one install, or ErrNotFound. A record still under
// the legacy kind is read too.
func (s *Store) GetPackageInstall(ctx context.Context, ownerID, computerID, installID string) (PackageInstall, error) {
	if s.og == nil {
		return PackageInstall{}, fmt.Errorf("store: object graph not initialized")
	}
	for _, kind := range []objectgraph.ObjectKind{ogKindPackageInstall, legacyInstallKind} {
		id, err := objectgraph.BuildCanonicalID(kind, strings.TrimSpace(ownerID), objectgraph.StableSuffixFromKey(packageInstallKey(computerID, installID)))
		if err != nil {
			return PackageInstall{}, err
		}
		obj, err := s.og.GetObject(ctx, id)
		if errors.Is(err, objectgraph.ErrNotFound) {
			continue
		}
		if err != nil {
			return PackageInstall{}, err
		}
		if obj.Tombstone {
			continue
		}
		return decodePackageInstall(obj)
	}
	return PackageInstall{}, ErrNotFound
}

// ListPackageInstalls returns the computer's installs, newest first. It
// returns every one: the installed set is a fold of the whole history, so a
// window would drop a package that is still installed. Records under the
// legacy kind are included; a record present under both kinds (a conversion
// that crashed between its two writes) is listed once, as the new kind.
func (s *Store) ListPackageInstalls(ctx context.Context, ownerID, computerID string) ([]PackageInstall, error) {
	matches := []objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}}
	seen := map[string]bool{}
	var out []PackageInstall
	for _, kind := range []objectgraph.ObjectKind{ogKindPackageInstall, legacyInstallKind} {
		objs, err := s.listPackageInstallObjects(ctx, kind, ownerID, matches)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			if obj.Tombstone {
				continue
			}
			install, err := decodePackageInstall(obj)
			if err != nil {
				return nil, err
			}
			if seen[install.InstallID] {
				continue
			}
			seen[install.InstallID] = true
			out = append(out, install)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

func (s *Store) listPackageInstallObjects(ctx context.Context, kind objectgraph.ObjectKind, ownerID string, matches []objectgraph.JSONFieldMatch) ([]objectgraph.Object, error) {
	// The list has no cursor, so the limit grows until a page comes back
	// short: the whole set, however long.
	limit := packageInstallPage
	for {
		page, err := s.ogListByOwnerAndBody(ctx, kind, strings.TrimSpace(ownerID), matches, limit)
		if err != nil {
			return nil, err
		}
		if len(page) < limit {
			return page, nil
		}
		limit *= 4
	}
}
