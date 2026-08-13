package selfdevprotocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

const (
	RestoreScopeVMLocal        = "vm_local"
	RestoreScopeFrontend       = "computer_surface_frontend"
	RestoreScopePlatform       = "platform"
	RestoreScopeCycle          = "cycle"
	FrontendDerivationRelease  = "release"
	FrontendDerivationExplicit = "explicit"
)

var restoreOutOfScopeDatabases = map[string]struct{}{
	RestoreScopePlatform: {},
	RestoreScopeCycle:    {},
}

// VMLocalContentWitness is the verification commitment for the event-derived
// VM-local Dolt projection. DoltHead is an audit receipt, never the restore address.
type VMLocalContentWitness struct {
	Database           string            `json:"database"`
	ContentRoot        string            `json:"content_root"`
	Schema             map[string]string `json:"schema"`
	Tables             map[string]string `json:"tables"`
	DoltHead           string            `json:"dolt_head"`
	DerivabilityDigest string            `json:"derivability_digest"`
}

// FrontendIdentity names the served computer-surface SPA. A checkpoint without
// a derivable identity is refused.
type FrontendIdentity struct {
	Digest        string `json:"digest"`
	Derivation    string `json:"derivation"`
	ReleaseDigest string `json:"release_digest,omitempty"`
}

// RestoreRequest is the owner-product restore operand. Platform and cycle
// stores cannot appear in the operand; the witness is the same scope.
type RestoreRequest struct {
	ComputerID       string   `json:"computer_id"`
	CheckpointDigest string   `json:"checkpoint_digest"`
	OperandScopes    []string `json:"operand_scopes"`
}

func (w VMLocalContentWitness) Validate() error {
	database := strings.TrimSpace(w.Database)
	if database == "" {
		return fmt.Errorf("self-development checkpoint: VM-local content witness is required")
	}
	if _, banned := restoreOutOfScopeDatabases[database]; banned {
		return fmt.Errorf("self-development checkpoint: %s store is out of restore scope", database)
	}
	if !computerevent.IsSHA256(w.ContentRoot) || !computerevent.IsSHA256(w.DoltHead) || !computerevent.IsSHA256(w.DerivabilityDigest) {
		return fmt.Errorf("self-development checkpoint: VM-local content witness digests are required")
	}
	if len(w.Schema) == 0 || len(w.Tables) == 0 || len(w.Schema) != len(w.Tables) {
		return fmt.Errorf("self-development checkpoint: VM-local schema and table hashes are required")
	}
	for table, schemaHash := range w.Schema {
		table = strings.TrimSpace(table)
		if table == "" || !computerevent.IsSHA256(schemaHash) {
			return fmt.Errorf("self-development checkpoint: VM-local schema hash is incomplete")
		}
		tableHash, ok := w.Tables[table]
		if !ok || !computerevent.IsSHA256(tableHash) {
			return fmt.Errorf("self-development checkpoint: VM-local table hash is incomplete")
		}
	}
	for table := range w.Tables {
		if _, ok := w.Schema[table]; !ok {
			return fmt.Errorf("self-development checkpoint: VM-local table hash is incomplete")
		}
	}
	return nil
}

func (f FrontendIdentity) Validate(releaseDigest string) error {
	if !computerevent.IsSHA256(f.Digest) {
		return fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	switch f.Derivation {
	case FrontendDerivationRelease:
		if !computerevent.IsSHA256(f.ReleaseDigest) || f.ReleaseDigest != strings.TrimSpace(releaseDigest) {
			return fmt.Errorf("self-development checkpoint: frontend identity is not derivable from the release")
		}
	case FrontendDerivationExplicit:
		if f.ReleaseDigest != "" && !computerevent.IsSHA256(f.ReleaseDigest) {
			return fmt.Errorf("self-development checkpoint: frontend identity release join is incomplete")
		}
	default:
		return fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	return nil
}

func RestoreFromRequest(request RestoreRequest, checkpoint Checkpoint) error {
	if strings.TrimSpace(request.ComputerID) == "" || request.ComputerID != checkpoint.Request.ComputerID {
		return fmt.Errorf("restore: computer binding does not match checkpoint")
	}
	if !computerevent.IsSHA256(request.CheckpointDigest) || request.CheckpointDigest != checkpoint.Digest {
		return fmt.Errorf("restore: checkpoint digest does not match")
	}
	if err := checkpoint.Request.VMLocalContentWitness.Validate(); err != nil {
		return fmt.Errorf("restore: checkpoint witness refused: %w", err)
	}
	if err := checkpoint.Request.FrontendIdentity.Validate(checkpoint.Request.ReleaseDigest); err != nil {
		return fmt.Errorf("restore: checkpoint frontend identity refused: %w", err)
	}
	haveVMLocal, haveFrontend := false, false
	for _, scope := range request.OperandScopes {
		switch strings.TrimSpace(scope) {
		case RestoreScopeVMLocal:
			haveVMLocal = true
		case RestoreScopeFrontend:
			haveFrontend = true
		case RestoreScopePlatform, RestoreScopeCycle:
			return fmt.Errorf("restore: %s store is out of restore scope", scope)
		case "":
			return fmt.Errorf("restore: operand scope is incomplete")
		default:
			return fmt.Errorf("restore: unknown operand scope %q", scope)
		}
	}
	if !haveVMLocal || !haveFrontend {
		return fmt.Errorf("restore: operand must include vm-local projection and computer-surface frontend")
	}
	return nil
}

func WitnessFromObservationSets(live, replay computerversion.ObservationSet, result computerversion.EquivalenceResult) (VMLocalContentWitness, error) {
	if !result.Equivalent() {
		return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: behavior-bearing VM-local rows are not event- or receipt-derivable")
	}
	liveWitness, err := witnessFromObservations(live)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	replayWitness, err := witnessFromObservations(replay)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	if liveWitness.Database != replayWitness.Database || liveWitness.ContentRoot != replayWitness.ContentRoot {
		return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: behavior-bearing VM-local rows are not event- or receipt-derivable")
	}
	if len(liveWitness.Schema) != len(replayWitness.Schema) || len(liveWitness.Tables) != len(replayWitness.Tables) {
		return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: behavior-bearing VM-local rows are not event- or receipt-derivable")
	}
	for table, schemaHash := range liveWitness.Schema {
		if replayWitness.Schema[table] != schemaHash || replayWitness.Tables[table] != liveWitness.Tables[table] {
			return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: behavior-bearing VM-local rows are not event- or receipt-derivable")
		}
	}
	proof, err := computerevent.CanonicalJSON(struct {
		Database    string            `json:"database"`
		ContentRoot string            `json:"content_root"`
		Schema      map[string]string `json:"schema"`
		Tables      map[string]string `json:"tables"`
	}{liveWitness.Database, liveWitness.ContentRoot, liveWitness.Schema, liveWitness.Tables})
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	liveWitness.DerivabilityDigest = computerevent.DigestBytes(proof)
	if err := liveWitness.Validate(); err != nil {
		return VMLocalContentWitness{}, err
	}
	return liveWitness, nil
}

type ReleaseFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func FrontendIdentityFromReleaseFiles(releaseDigest string, files []ReleaseFile) (FrontendIdentity, error) {
	if !computerevent.IsSHA256(strings.TrimSpace(releaseDigest)) {
		return FrontendIdentity{}, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	var surface []ReleaseFile
	for _, file := range files {
		path := strings.TrimPrefix(strings.TrimSpace(file.Path), "/")
		if path == "frontend" || strings.HasPrefix(path, "frontend/") {
			if !computerevent.IsSHA256(strings.TrimSpace(file.SHA256)) {
				return FrontendIdentity{}, fmt.Errorf("self-development checkpoint: served SPA is underivable")
			}
			surface = append(surface, ReleaseFile{Path: path, SHA256: strings.TrimSpace(file.SHA256)})
		}
	}
	if len(surface) == 0 {
		return FrontendIdentity{}, fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	sort.Slice(surface, func(i, j int) bool { return surface[i].Path < surface[j].Path })
	canonical, err := computerevent.CanonicalJSON(surface)
	if err != nil {
		return FrontendIdentity{}, err
	}
	return FrontendIdentity{
		Digest:        computerevent.DigestBytes(canonical),
		Derivation:    FrontendDerivationRelease,
		ReleaseDigest: strings.TrimSpace(releaseDigest),
	}, nil
}

func witnessFromObservations(set computerversion.ObservationSet) (VMLocalContentWitness, error) {
	witness := VMLocalContentWitness{Schema: map[string]string{}, Tables: map[string]string{}}
	for _, observation := range set.Observations {
		database, kind, table, ok := parseDoltObservationKey(observation.Key)
		if !ok {
			continue
		}
		if _, banned := restoreOutOfScopeDatabases[database]; banned {
			return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: %s store is out of restore scope", database)
		}
		if witness.Database == "" {
			witness.Database = database
		} else if witness.Database != database {
			return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: VM-local content witness spans multiple databases")
		}
		digest := stripDigestPrefix(observation.Value)
		switch kind {
		case "head":
			witness.DoltHead = digest
		case "content_root":
			witness.ContentRoot = digest
		case "schema":
			witness.Schema[table] = digest
		case "table":
			witness.Tables[table] = digest
		}
	}
	if witness.Database == "" {
		return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: VM-local content witness is required")
	}
	return witness, nil
}

func parseDoltObservationKey(key string) (database, kind, table string, ok bool) {
	const prefix = "dolt:"
	if !strings.HasPrefix(key, prefix) {
		return "", "", "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	database, rest, found := strings.Cut(rest, ":")
	if !found || database == "" || rest == "" {
		return "", "", "", false
	}
	switch {
	case rest == "head" || rest == "content_root":
		return database, rest, "", true
	case strings.HasPrefix(rest, "schema:"):
		table = strings.TrimPrefix(rest, "schema:")
		if table == "" {
			return "", "", "", false
		}
		return database, "schema", table, true
	case strings.HasPrefix(rest, "table:"):
		table = strings.TrimPrefix(rest, "table:")
		if table == "" {
			return "", "", "", false
		}
		return database, "table", table, true
	default:
		return "", "", "", false
	}
}

func stripDigestPrefix(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimPrefix(value, "sha256:")
}
