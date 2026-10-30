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
// VM-local SQLite projection (witness v3, witness_v3.go). ContentRoot commits
// to SchemaHash and every table digest; it is the restore address of the
// projection's content.
type VMLocalContentWitness struct {
	Database    string `json:"database"`
	ContentRoot string `json:"content_root"`
	// SchemaHash is the SHA-256 over sqlite_master (sql text, sorted by name).
	SchemaHash string `json:"schema_hash"`
	// Tables is the per-table SHA-256 over the rows in primary-key order.
	Tables             map[string]string `json:"tables"`
	DerivabilityDigest string            `json:"derivability_digest"`
	// WitnessVersion is 3. A reader refuses every other version
	// (RefuseWitnessVersion): 1 and 2 were Dolt witnesses.
	WitnessVersion int `json:"witness_version"`
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
	if err := RefuseWitnessVersion(w.version()); err != nil {
		return err
	}
	return w.validateV3()
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
	if err := ValidateTapeCompleteness(checkpoint.Request.TapeCompleteness, checkpoint.Request.CompleteFromHead); err != nil {
		return err
	}
	return nil
}

// WitnessFromObservationSets seals the v3 witness of a live store that a
// replay reproduced exactly: it refuses unless the two sets are equivalent and
// carry the same root.
func WitnessFromObservationSets(live, replay computerversion.ObservationSet, result computerversion.EquivalenceResult) (VMLocalContentWitness, error) {
	notDerivable := fmt.Errorf("self-development checkpoint: behavior-bearing VM-local rows are not event- or receipt-derivable")
	if !result.Equivalent() {
		return VMLocalContentWitness{}, notDerivable
	}
	liveWitness, err := witnessFromObservations(live)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	replayWitness, err := witnessFromObservations(replay)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	if liveWitness.Database != replayWitness.Database || liveWitness.ContentRoot != replayWitness.ContentRoot || liveWitness.SchemaHash != replayWitness.SchemaHash || len(liveWitness.Tables) != len(replayWitness.Tables) {
		return VMLocalContentWitness{}, notDerivable
	}
	for table, digest := range liveWitness.Tables {
		if replayWitness.Tables[table] != digest {
			return VMLocalContentWitness{}, notDerivable
		}
	}
	sealed, err := SealWitness(liveWitness)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	if err := sealed.Validate(); err != nil {
		return VMLocalContentWitness{}, err
	}
	return sealed, nil
}

// WitnessFromObservationSet seals a v3 witness from one extraction's
// observations (computerversion.SQLiteStateExtractor.Extract).
func WitnessFromObservationSet(set computerversion.ObservationSet) (VMLocalContentWitness, error) {
	witness, err := witnessFromObservations(set)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	sealed, err := SealWitness(witness)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	if err := sealed.Validate(); err != nil {
		return VMLocalContentWitness{}, err
	}
	return sealed, nil
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
	witness := VMLocalContentWitness{WitnessVersion: WitnessVersionSQLite, Tables: map[string]string{}}
	for _, observation := range set.Observations {
		database, kind, table, ok := parseStateObservationKey(observation.Key)
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
		switch kind {
		case "content_root":
			witness.ContentRoot = observationDigest(observation.Value)
		case "schema":
			witness.SchemaHash = observationDigest(observation.Value)
		case "table":
			witness.Tables[table] = observationDigest(observation.Value)
		}
	}
	if witness.Database == "" {
		return VMLocalContentWitness{}, fmt.Errorf("self-development checkpoint: VM-local content witness is required")
	}
	return witness, nil
}

// parseStateObservationKey reads the keys SQLiteStateExtractor writes:
// sqlite:D:schema, sqlite:D:content_root, sqlite:D:table:T. The per-table
// schema keys (sqlite:D:schema:T) are diagnostics and are skipped.
func parseStateObservationKey(key string) (database, kind, table string, ok bool) {
	const prefix = "sqlite:"
	if !strings.HasPrefix(key, prefix) {
		return "", "", "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	database, rest, found := strings.Cut(rest, ":")
	if !found || database == "" || rest == "" {
		return "", "", "", false
	}
	switch {
	case rest == "schema" || rest == "content_root":
		return database, rest, "", true
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

func observationDigest(value string) string {
	stripped := stripDigestPrefix(value)
	if computerevent.IsSHA256(stripped) {
		return stripped
	}
	return computerevent.DigestBytes([]byte(strings.TrimSpace(value)))
}
