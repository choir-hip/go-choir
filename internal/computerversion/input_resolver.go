package computerversion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrInputNotFound = errors.New("computer input resolver: immutable input not found")

type CodeArtifact struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	URI    string `json:"uri"`
}

type CodeClosure struct {
	Ref          CodeRef        `json:"code_ref"`
	SourceCommit string         `json:"source_commit"`
	Artifacts    []CodeArtifact `json:"artifacts"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ArtifactProgramEntry struct {
	Sequence          uint64 `json:"sequence"`
	Kind              string `json:"kind"`
	ContentSHA256     string `json:"content_sha256"`
	ArtifactURI       string `json:"artifact_uri"`
	PreviousEntryHash string `json:"previous_entry_hash,omitempty"`
	EntryHash         string `json:"entry_hash"`
}

type ArtifactProgram struct {
	Ref       ArtifactProgramRef     `json:"artifact_program_ref"`
	Entries   []ArtifactProgramEntry `json:"entries"`
	CreatedAt time.Time              `json:"created_at"`
}

type ImmutableInputResolver interface {
	ResolveCode(context.Context, CodeRef) (CodeClosure, error)
	ResolveArtifactProgram(context.Context, ArtifactProgramRef) (ArtifactProgram, error)
}

type ImmutableInputCatalog interface {
	ImmutableInputResolver
	PinCode(context.Context, CodeClosure) (CodeClosure, error)
	PinArtifactProgram(context.Context, ArtifactProgram) (ArtifactProgram, error)
}

func NewCodeClosure(sourceCommit string, artifacts []CodeArtifact, createdAt time.Time) (CodeClosure, error) {
	closure := CodeClosure{SourceCommit: strings.ToLower(strings.TrimSpace(sourceCommit)), Artifacts: append([]CodeArtifact(nil), artifacts...), CreatedAt: createdAt.UTC()}
	for i := range closure.Artifacts {
		closure.Artifacts[i].Name = strings.TrimSpace(closure.Artifacts[i].Name)
		closure.Artifacts[i].SHA256 = strings.ToLower(strings.TrimSpace(closure.Artifacts[i].SHA256))
		closure.Artifacts[i].URI = strings.TrimSpace(closure.Artifacts[i].URI)
	}
	if closure.CreatedAt.IsZero() {
		return CodeClosure{}, fmt.Errorf("computer input resolver: code closure creation time is required")
	}
	sort.Slice(closure.Artifacts, func(i, j int) bool { return closure.Artifacts[i].Name < closure.Artifacts[j].Name })
	if err := validateCodeClosurePayload(closure); err != nil {
		return CodeClosure{}, err
	}
	payload, err := codeClosurePayloadJSON(closure)
	if err != nil {
		return CodeClosure{}, err
	}
	closure.Ref = CodeRef("code:sha256:" + immutableInputSHA256Hex(payload))
	return closure, nil
}

func NewArtifactProgram(entries []ArtifactProgramEntry, createdAt time.Time) (ArtifactProgram, error) {
	program := ArtifactProgram{Entries: append([]ArtifactProgramEntry(nil), entries...), CreatedAt: createdAt.UTC()}
	if program.CreatedAt.IsZero() {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: artifact program creation time is required")
	}
	previous := ""
	for i := range program.Entries {
		entry := &program.Entries[i]
		entry.Sequence = uint64(i + 1)
		entry.Kind = strings.TrimSpace(entry.Kind)
		entry.ContentSHA256 = strings.ToLower(strings.TrimSpace(entry.ContentSHA256))
		entry.ArtifactURI = strings.TrimSpace(entry.ArtifactURI)
		entry.PreviousEntryHash = previous
		if entry.Kind == "" || !validSHA256(entry.ContentSHA256) || !validContentAddressedURI(entry.ArtifactURI, entry.ContentSHA256) {
			return ArtifactProgram{}, fmt.Errorf("computer input resolver: invalid artifact program entry %d", i+1)
		}
		hash, err := artifactProgramEntryHash(*entry)
		if err != nil {
			return ArtifactProgram{}, err
		}
		entry.EntryHash = hash
		previous = hash
	}
	if len(program.Entries) == 0 {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: artifact program requires at least one entry")
	}
	payload, err := artifactProgramPayloadJSON(program)
	if err != nil {
		return ArtifactProgram{}, err
	}
	program.Ref = ArtifactProgramRef("artifact-program:sha256:" + immutableInputSHA256Hex(payload))
	return program, nil
}

func (c CodeClosure) Verify() error {
	if !c.Ref.Valid() {
		return fmt.Errorf("computer input resolver: code ref is required")
	}
	if err := validateCodeClosurePayload(c); err != nil {
		return err
	}
	payload, err := codeClosurePayloadJSON(c)
	if err != nil {
		return err
	}
	want := CodeRef("code:sha256:" + immutableInputSHA256Hex(payload))
	if c.Ref != want {
		return fmt.Errorf("computer input resolver: code closure hash mismatch")
	}
	return nil
}

func (p ArtifactProgram) Verify() error {
	if !p.Ref.Valid() || len(p.Entries) == 0 || p.CreatedAt.IsZero() {
		return fmt.Errorf("computer input resolver: incomplete artifact program")
	}
	previous := ""
	for i, entry := range p.Entries {
		if entry.Sequence != uint64(i+1) || entry.PreviousEntryHash != previous || entry.Kind == "" || !validSHA256(entry.ContentSHA256) || !validContentAddressedURI(entry.ArtifactURI, entry.ContentSHA256) {
			return fmt.Errorf("computer input resolver: invalid artifact program entry %d", i+1)
		}
		want, err := artifactProgramEntryHash(entry)
		if err != nil {
			return err
		}
		if entry.EntryHash != want {
			return fmt.Errorf("computer input resolver: artifact program entry %d hash mismatch", i+1)
		}
		previous = entry.EntryHash
	}
	payload, err := artifactProgramPayloadJSON(p)
	if err != nil {
		return err
	}
	want := ArtifactProgramRef("artifact-program:sha256:" + immutableInputSHA256Hex(payload))
	if p.Ref != want {
		return fmt.Errorf("computer input resolver: artifact program hash mismatch")
	}
	return nil
}

func validateCodeClosurePayload(c CodeClosure) error {
	if !validSourceCommit(c.SourceCommit) || c.CreatedAt.IsZero() || len(c.Artifacts) == 0 {
		return fmt.Errorf("computer input resolver: incomplete code closure")
	}
	last := ""
	for _, artifact := range c.Artifacts {
		if artifact.Name == "" || artifact.Name <= last || !validSHA256(strings.ToLower(artifact.SHA256)) || !validContentAddressedURI(artifact.URI, artifact.SHA256) {
			return fmt.Errorf("computer input resolver: invalid or unsorted code artifact %q", artifact.Name)
		}
		last = artifact.Name
	}
	return nil
}

func codeClosurePayloadJSON(c CodeClosure) ([]byte, error) {
	return json.Marshal(struct {
		SourceCommit string         `json:"source_commit"`
		Artifacts    []CodeArtifact `json:"artifacts"`
		CreatedAt    time.Time      `json:"created_at"`
	}{SourceCommit: c.SourceCommit, Artifacts: c.Artifacts, CreatedAt: c.CreatedAt.UTC()})
}

func artifactProgramPayloadJSON(p ArtifactProgram) ([]byte, error) {
	return json.Marshal(struct {
		Entries   []ArtifactProgramEntry `json:"entries"`
		CreatedAt time.Time              `json:"created_at"`
	}{Entries: p.Entries, CreatedAt: p.CreatedAt.UTC()})
}

func artifactProgramEntryHash(entry ArtifactProgramEntry) (string, error) {
	payload, err := json.Marshal(struct {
		Sequence          uint64 `json:"sequence"`
		Kind              string `json:"kind"`
		ContentSHA256     string `json:"content_sha256"`
		ArtifactURI       string `json:"artifact_uri"`
		PreviousEntryHash string `json:"previous_entry_hash,omitempty"`
	}{entry.Sequence, entry.Kind, entry.ContentSHA256, entry.ArtifactURI, entry.PreviousEntryHash})
	if err != nil {
		return "", fmt.Errorf("computer input resolver: encode artifact entry: %w", err)
	}
	return immutableInputSHA256Hex(payload), nil
}

func validSourceCommit(value string) bool {
	value = strings.TrimSpace(value)
	return (len(value) == 40 || len(value) == 64) && validHex(value)
}

func contentAddressedURI(scheme, digest, locator string) string {
	return (&url.URL{Scheme: scheme, Host: strings.ToLower(strings.TrimSpace(digest)), Path: "/" + strings.TrimLeft(strings.TrimSpace(locator), "/")}).String()
}

func validContentAddressedURI(raw, digest string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	switch parsed.Scheme {
	case "artifact+sha256", "nix-store+sha256":
	default:
		return false
	}
	if !strings.EqualFold(parsed.Host, strings.TrimSpace(digest)) || !validSHA256(strings.ToLower(parsed.Host)) {
		return false
	}
	if parsed.Path == "" || parsed.Path == "/" || !filepath.IsAbs(parsed.Path) {
		return false
	}
	return parsed.Scheme != "nix-store+sha256" || strings.HasPrefix(filepath.Clean(parsed.Path), "/nix/store/")
}

func validHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func immutableInputSHA256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

const immutableInputSchema = `
CREATE TABLE IF NOT EXISTS computer_version_code_closures (
  code_ref VARCHAR(96) NOT NULL PRIMARY KEY,
  source_commit VARCHAR(128) NOT NULL,
  closure_json LONGTEXT NOT NULL,
  created_at DATETIME(6) NOT NULL
);
CREATE TABLE IF NOT EXISTS computer_version_artifact_programs (
  artifact_program_ref VARCHAR(128) NOT NULL PRIMARY KEY,
  program_json LONGTEXT NOT NULL,
  created_at DATETIME(6) NOT NULL
);`

type ArtifactContentVerifier interface {
	VerifyArtifact(context.Context, string, string) error
}

type LocalArtifactContentVerifier struct {
	artifactRoot string
}

func NewLocalArtifactContentVerifier(artifactRoot string) *LocalArtifactContentVerifier {
	return &LocalArtifactContentVerifier{artifactRoot: filepath.Clean(strings.TrimSpace(artifactRoot))}
}

func (v *LocalArtifactContentVerifier) VerifyArtifact(ctx context.Context, rawURI, digest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validContentAddressedURI(rawURI, digest) {
		return fmt.Errorf("computer input resolver: artifact URI is not bound to content digest")
	}
	parsed, _ := url.Parse(rawURI)
	artifactPath := filepath.Clean(parsed.Path)
	if parsed.Scheme == "artifact+sha256" {
		if v == nil || v.artifactRoot == "" || v.artifactRoot == "." {
			return fmt.Errorf("computer input resolver: artifact root is required for artifact+sha256 URI")
		}
		relative := strings.TrimPrefix(artifactPath, string(filepath.Separator))
		artifactPath = filepath.Join(v.artifactRoot, relative)
		rootPrefix := v.artifactRoot + string(filepath.Separator)
		if artifactPath != v.artifactRoot && !strings.HasPrefix(artifactPath, rootPrefix) {
			return fmt.Errorf("computer input resolver: artifact URI escapes artifact root")
		}
	}
	file, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("computer input resolver: open immutable artifact: %w", err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("computer input resolver: stat immutable artifact: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("computer input resolver: immutable artifact must be a regular file")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("computer input resolver: hash immutable artifact: %w", err)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, digest) {
		return fmt.Errorf("computer input resolver: immutable artifact content hash mismatch")
	}
	return nil
}

type SQLInputCatalog struct {
	db       *sql.DB
	verifier ArtifactContentVerifier
}

func NewSQLInputCatalog(db *sql.DB, verifiers ...ArtifactContentVerifier) *SQLInputCatalog {
	catalog := &SQLInputCatalog{db: db}
	if len(verifiers) > 0 {
		catalog.verifier = verifiers[0]
	}
	return catalog
}

func (c *SQLInputCatalog) EnsureSchema(ctx context.Context) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("computer input resolver: SQL database is required")
	}
	if _, err := c.db.ExecContext(ctx, immutableInputSchema); err != nil {
		return fmt.Errorf("computer input resolver: ensure schema: %w", err)
	}
	return nil
}

func (c *SQLInputCatalog) PinCode(ctx context.Context, closure CodeClosure) (CodeClosure, error) {
	if c == nil || c.db == nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: SQL database is required")
	}
	if err := closure.Verify(); err != nil {
		return CodeClosure{}, err
	}
	if c.verifier == nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: artifact content verifier is required")
	}
	for _, artifact := range closure.Artifacts {
		if err := c.verifier.VerifyArtifact(ctx, artifact.URI, artifact.SHA256); err != nil {
			return CodeClosure{}, fmt.Errorf("computer input resolver: verify code artifact %q: %w", artifact.Name, err)
		}
	}
	encoded, err := json.Marshal(closure)
	if err != nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: encode code closure: %w", err)
	}
	if _, err := c.db.ExecContext(ctx, `INSERT INTO computer_version_code_closures (code_ref, source_commit, closure_json, created_at)
		VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE code_ref = code_ref`, closure.Ref, closure.SourceCommit, encoded, closure.CreatedAt); err != nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: pin code closure: %w", err)
	}
	resolved, err := c.ResolveCode(ctx, closure.Ref)
	if err != nil {
		return CodeClosure{}, err
	}
	resolvedJSON, _ := json.Marshal(resolved)
	if string(resolvedJSON) != string(encoded) {
		return CodeClosure{}, fmt.Errorf("computer input resolver: code ref collision")
	}
	return resolved, nil
}

func (c *SQLInputCatalog) PinArtifactProgram(ctx context.Context, program ArtifactProgram) (ArtifactProgram, error) {
	if c == nil || c.db == nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: SQL database is required")
	}
	if err := program.Verify(); err != nil {
		return ArtifactProgram{}, err
	}
	if c.verifier == nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: artifact content verifier is required")
	}
	for _, entry := range program.Entries {
		if err := c.verifier.VerifyArtifact(ctx, entry.ArtifactURI, entry.ContentSHA256); err != nil {
			return ArtifactProgram{}, fmt.Errorf("computer input resolver: verify artifact program entry %d: %w", entry.Sequence, err)
		}
	}
	encoded, err := json.Marshal(program)
	if err != nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: encode artifact program: %w", err)
	}
	if _, err := c.db.ExecContext(ctx, `INSERT INTO computer_version_artifact_programs (artifact_program_ref, program_json, created_at)
		VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE artifact_program_ref = artifact_program_ref`, program.Ref, encoded, program.CreatedAt); err != nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: pin artifact program: %w", err)
	}
	resolved, err := c.ResolveArtifactProgram(ctx, program.Ref)
	if err != nil {
		return ArtifactProgram{}, err
	}
	resolvedJSON, _ := json.Marshal(resolved)
	if string(resolvedJSON) != string(encoded) {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: artifact program ref collision")
	}
	return resolved, nil
}

type inputQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (c *SQLInputCatalog) ResolveCode(ctx context.Context, ref CodeRef) (CodeClosure, error) {
	if c == nil || c.db == nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: SQL database is required")
	}
	return resolveCodeSQL(ctx, c.db, ref, false)
}

func (c *SQLInputCatalog) ResolveArtifactProgram(ctx context.Context, ref ArtifactProgramRef) (ArtifactProgram, error) {
	if c == nil || c.db == nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: SQL database is required")
	}
	return resolveArtifactProgramSQL(ctx, c.db, ref, false)
}

// VerifySQLInputsInTransition re-resolves and locks both immutable inputs inside
// the same SQL transaction that advances D-ROUTE. The locks close the catalog
// resolution/CAS race; a transition cannot commit against a concurrently changed
// or deleted input declaration.
func VerifySQLInputsInTransition(ctx context.Context, tx *sql.Tx, version ComputerVersion) error {
	if tx == nil || !version.Valid() {
		return fmt.Errorf("computer input resolver: SQL transaction and ComputerVersion are required")
	}
	if _, err := resolveCodeSQL(ctx, tx, version.CodeRef, true); err != nil {
		return fmt.Errorf("resolve CodeRef in route transaction: %w", err)
	}
	if _, err := resolveArtifactProgramSQL(ctx, tx, version.ArtifactProgramRef, true); err != nil {
		return fmt.Errorf("resolve ArtifactProgramRef in route transaction: %w", err)
	}
	return nil
}

func resolveCodeSQL(ctx context.Context, queryer inputQueryRower, ref CodeRef, forUpdate bool) (CodeClosure, error) {
	if queryer == nil || !ref.Valid() {
		return CodeClosure{}, fmt.Errorf("computer input resolver: valid code ref and SQL queryer are required")
	}
	query := `SELECT closure_json FROM computer_version_code_closures WHERE code_ref = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var encoded []byte
	if err := queryer.QueryRowContext(ctx, query, ref).Scan(&encoded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CodeClosure{}, ErrInputNotFound
		}
		return CodeClosure{}, fmt.Errorf("computer input resolver: resolve code: %w", err)
	}
	var closure CodeClosure
	if err := json.Unmarshal(encoded, &closure); err != nil {
		return CodeClosure{}, fmt.Errorf("computer input resolver: decode code closure: %w", err)
	}
	if closure.Ref != ref {
		return CodeClosure{}, fmt.Errorf("computer input resolver: resolved code ref mismatch")
	}
	if err := closure.Verify(); err != nil {
		return CodeClosure{}, err
	}
	return closure, nil
}

func resolveArtifactProgramSQL(ctx context.Context, queryer inputQueryRower, ref ArtifactProgramRef, forUpdate bool) (ArtifactProgram, error) {
	if queryer == nil || !ref.Valid() {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: valid artifact program ref and SQL queryer are required")
	}
	query := `SELECT program_json FROM computer_version_artifact_programs WHERE artifact_program_ref = ?`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var encoded []byte
	if err := queryer.QueryRowContext(ctx, query, ref).Scan(&encoded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArtifactProgram{}, ErrInputNotFound
		}
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: resolve artifact program: %w", err)
	}
	var program ArtifactProgram
	if err := json.Unmarshal(encoded, &program); err != nil {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: decode artifact program: %w", err)
	}
	if program.Ref != ref {
		return ArtifactProgram{}, fmt.Errorf("computer input resolver: resolved artifact program ref mismatch")
	}
	if err := program.Verify(); err != nil {
		return ArtifactProgram{}, err
	}
	return program, nil
}
