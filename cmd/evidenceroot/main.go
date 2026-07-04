package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	embedded "github.com/dolthub/driver"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/api"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const defaultName = "candidate-evidence-root"

type config struct {
	root               string
	name               string
	codeRef            string
	artifactProgramRef string
}

type output struct {
	Manifest       computerversion.CandidateEvidenceRootManifest `json:"manifest"`
	Observation    computerversion.ObservationSet                `json:"observation"`
	SelfCheck      computerversion.EquivalenceResult             `json:"self_check"`
	SeededMismatch computerversion.EquivalenceResult             `json:"seeded_mismatch"`
	BaseFixture    baseFixtureOutput                             `json:"base_fixture"`
}

type configuredServer struct {
	server      *server.Server
	authStore   *auth.Store
	persistent  *api.PersistentHandler
	journalPath string
	blobRoot    string
	authDBPath  string
}

type baseFixtureOutput struct {
	JournalPath string        `json:"journal_path"`
	BlobRoot    string        `json:"blob_root"`
	AuthDBPath  string        `json:"auth_db_path"`
	ItemID      model.ItemID  `json:"item_id"`
	BlobRef     model.BlobRef `json:"blob_ref"`
	SHA256      string        `json:"sha256"`
}

type fixtureBlobResponse struct {
	BlobRef model.BlobRef `json:"blob_ref"`
	SHA256  string        `json:"sha256"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	out, err := provision(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(stderr, "evidenceroot: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "evidenceroot: encode output: %v\n", err)
		return 1
	}
	return 0
}

func parseConfig(args []string, stderr io.Writer) (config, error) {
	fs := flag.NewFlagSet("evidenceroot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := config{name: defaultName}
	fs.StringVar(&cfg.root, "root", "", "empty local directory where candidate evidence root will be provisioned")
	fs.StringVar(&cfg.name, "name", defaultName, "observation/manifest name")
	fs.StringVar(&cfg.codeRef, "code-ref", "", "candidate ComputerVersion code ref")
	fs.StringVar(&cfg.artifactProgramRef, "artifact-program-ref", "", "candidate ComputerVersion artifact program ref")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.root) == "" {
		return config{}, fmt.Errorf("evidenceroot: --root is required")
	}
	if strings.TrimSpace(cfg.codeRef) == "" {
		return config{}, fmt.Errorf("evidenceroot: --code-ref is required")
	}
	if strings.TrimSpace(cfg.artifactProgramRef) == "" {
		return config{}, fmt.Errorf("evidenceroot: --artifact-program-ref is required")
	}
	if strings.TrimSpace(cfg.name) == "" {
		return config{}, fmt.Errorf("evidenceroot: --name is required")
	}
	return cfg, nil
}

func provision(ctx context.Context, cfg config) (output, error) {
	root, err := prepareRoot(cfg.root)
	if err != nil {
		return output{}, err
	}
	journalPath := filepath.Join(root, "base.sqlite")
	blobRoot := filepath.Join(root, "blobs")
	authDBPath := filepath.Join(root, "auth.sqlite")
	configured, err := openConfiguredServer(journalPath, blobRoot, authDBPath)
	if err != nil {
		return output{}, err
	}
	defer configured.Close()
	baseFixture, err := configured.seedFixture(ctx)
	if err != nil {
		return output{}, err
	}
	vmDir := filepath.Join(root, "vm")
	persistentDir := filepath.Join(vmDir, "persist")
	if err := os.MkdirAll(persistentDir, 0o755); err != nil {
		return output{}, fmt.Errorf("create vm persistent dir: %w", err)
	}
	dataImage := filepath.Join(vmDir, "data.img")
	kernelImage := filepath.Join(vmDir, "vmlinux")
	rootfs := filepath.Join(vmDir, "rootfs.ext4")
	if err := os.WriteFile(dataImage, []byte("candidate evidence root data image\n"), 0o644); err != nil {
		return output{}, fmt.Errorf("write data image fixture: %w", err)
	}
	if err := os.WriteFile(kernelImage, []byte("candidate evidence root kernel artifact\n"), 0o644); err != nil {
		return output{}, fmt.Errorf("write kernel fixture: %w", err)
	}
	if err := os.WriteFile(rootfs, []byte("candidate evidence root rootfs artifact\n"), 0o644); err != nil {
		return output{}, fmt.Errorf("write rootfs fixture: %w", err)
	}

	version := computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(cfg.codeRef), ArtifactProgramRef: computerversion.ArtifactProgramRef(cfg.artifactProgramRef)}
	objectGraph := objectGraphSnapshot(cfg, version)
	doltHead, err := provisionDoltObjectGraph(ctx, root, objectGraph)
	if err != nil {
		return output{}, err
	}
	manifest := computerversion.CandidateEvidenceRootManifest{
		ID:                    cfg.name,
		RootPath:              root,
		Source:                computerversion.EvidenceRootSourceLocalCandidate,
		AuthorizedForSampling: true,
		ContainsProduction:    false,
		TouchesDeployedRoute:  false,
		EvidenceRefs:          []string{cfg.name + ":manifest", cfg.name + ":observation"},
		Fixture: computerversion.ProductFixtureRoot{
			Version: version,
			Base: computerversion.BaseCurrentStatePaths{
				JournalPath: journalPath,
				BlobRoot:    blobRoot,
			},
			VM: computerversion.VMManagerScopedPath{
				VMID:            cfg.name + "-vm",
				PersistentDir:   persistentDir,
				DataImagePath:   dataImage,
				KernelImagePath: kernelImage,
				RootfsPath:      rootfs,
				ComputerKind:    "candidate",
				OwnerID:         cfg.name + "-owner",
				DesktopID:       cfg.name + "-desktop",
				CandidateID:     cfg.name + "-candidate",
				Epoch:           1,
			},
			Promotion: computerversion.PromotionCertificate{
				ID:            cfg.name + "-promotion",
				RouteSlot:     "desktop:" + cfg.name,
				Active:        computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(cfg.codeRef + "-active"), ArtifactProgramRef: computerversion.ArtifactProgramRef(cfg.artifactProgramRef + "-active")},
				Base:          computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(cfg.codeRef + "-active"), ArtifactProgramRef: computerversion.ArtifactProgramRef(cfg.artifactProgramRef + "-active")},
				Candidate:     version,
				OwnerApproved: true,
				HealthWindow:  computerversion.PromotionHealthConfirmed,
				Ledgers: []computerversion.PromotionLedgerCertificate{
					{Name: "data", State: computerversion.PromotionLedgerApplied},
					{Name: "index", State: computerversion.PromotionLedgerApplied},
					{Name: "source", State: computerversion.PromotionLedgerApplied},
				},
				RollbackRef: cfg.name + "-rollback",
				EvidenceRef: cfg.name + ":observation",
			},
			ObjectGraph: objectGraph,
			DoltHead:    doltHead,
		},
	}
	fixture, err := manifest.ProductFixtureRoot()
	if err != nil {
		return output{}, err
	}
	observation, err := fixture.ObservationSet(ctx, cfg.name)
	if err != nil {
		return output{}, err
	}
	selfCheck := computerversion.EquivalenceChecker{}.CheckObservationSets(observation, observation)
	mismatch := seededMismatch(observation)
	return output{Manifest: manifest, Observation: observation, SelfCheck: selfCheck, SeededMismatch: mismatch, BaseFixture: baseFixture}, nil
}

func objectGraphSnapshot(cfg config, version computerversion.ComputerVersion) *computerversion.ObjectGraphSnapshot {
	ownerID := cfg.name + "-owner"
	computerID := cfg.name + "-computer"
	agentBody := []byte("candidate evidence root objectgraph agent\n")
	agentMetadata := json.RawMessage(fmt.Sprintf(`{"computer_version":{"code_ref":%q,"artifact_program_ref":%q},"fixture":"candidate_evidence_root","role":"agent"}`, version.CodeRef, version.ArtifactProgramRef))
	agentKind := objectgraph.ObjectKind("choir.agent")
	agentHash := objectgraph.ContentHash(agentKind, agentBody, agentMetadata)
	agentID, _ := objectgraph.BuildCanonicalID(agentKind, ownerID, objectgraph.StableSuffixFromContent(agentHash))
	runBody := []byte("candidate evidence root objectgraph run\n")
	runMetadata := json.RawMessage(fmt.Sprintf(`{"computer_version":{"code_ref":%q,"artifact_program_ref":%q},"fixture":"candidate_evidence_root","role":"run"}`, version.CodeRef, version.ArtifactProgramRef))
	runKind := objectgraph.ObjectKind("choir.run")
	runHash := objectgraph.ContentHash(runKind, runBody, runMetadata)
	runID, _ := objectgraph.BuildCanonicalID(runKind, ownerID, objectgraph.StableSuffixFromKey(cfg.name+"-run"))
	edgeMetadata := json.RawMessage(`{"fixture":"candidate_evidence_root","relation":"records"}`)
	edgeID, _ := objectgraph.BuildEdgeID(agentID, runID, objectgraph.EdgeKind("records"), edgeMetadata)
	return &computerversion.ObjectGraphSnapshot{
		Objects: []objectgraph.Object{
			{CanonicalID: agentID, ObjectKind: agentKind, OwnerID: ownerID, ComputerID: computerID, VersionID: string(version.CodeRef), ContentHash: agentHash, Body: agentBody, Metadata: agentMetadata},
			{CanonicalID: runID, ObjectKind: runKind, OwnerID: ownerID, ComputerID: computerID, VersionID: string(version.CodeRef), ContentHash: runHash, Body: runBody, Metadata: runMetadata},
		},
		Edges: []objectgraph.Edge{{EdgeID: edgeID, FromID: agentID, ToID: runID, Kind: objectgraph.EdgeKind("records"), Metadata: edgeMetadata}},
	}
}

func provisionDoltObjectGraph(ctx context.Context, root string, snapshot *computerversion.ObjectGraphSnapshot) (*computerversion.DoltHeadSnapshot, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("dolt objectgraph: snapshot is required")
	}
	repoRoot := filepath.Join(root, "dolt-objectgraph")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create dolt objectgraph repo: %w", err)
	}
	const database = "objectgraph"
	db, connector, err := openDoltDatabase(repoRoot, database)
	if err != nil {
		return nil, err
	}
	defer connector.Close()
	defer db.Close()
	store := objectgraph.NewDoltStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	for _, obj := range snapshot.Objects {
		if err := store.PutObject(ctx, obj); err != nil {
			return nil, err
		}
	}
	for _, edge := range snapshot.Edges {
		if err := store.PutEdge(ctx, edge); err != nil {
			return nil, err
		}
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', 'candidate evidence root objectgraph')"); err != nil && !strings.Contains(err.Error(), "nothing to commit") {
		return nil, fmt.Errorf("dolt objectgraph commit: %w", err)
	}
	var commitHash string
	if err := db.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&commitHash); err != nil {
		return nil, fmt.Errorf("dolt objectgraph head: %w", err)
	}
	return &computerversion.DoltHeadSnapshot{
		RepoRoot:           repoRoot,
		Database:           database,
		CommitHash:         commitHash,
		ObjectGraph:        snapshot,
		Derivation:         "cmd/evidenceroot local embedded Dolt objectgraph fixture",
		ContainsProduction: false,
	}, nil
}

func openDoltDatabase(root, database string) (*sql.DB, interface{ Close() error }, error) {
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse dolt root dsn: %w", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open dolt root connector: %w", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + database); err != nil {
		_ = rootDB.Close()
		_ = rootConnector.Close()
		return nil, nil, fmt.Errorf("create dolt objectgraph database: %w", err)
	}
	if err := rootDB.Close(); err != nil {
		_ = rootConnector.Close()
		return nil, nil, fmt.Errorf("close dolt root db: %w", err)
	}
	if err := rootConnector.Close(); err != nil {
		return nil, nil, fmt.Errorf("close dolt root connector: %w", err)
	}

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true", root, database)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse dolt db dsn: %w", err)
	}
	connector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open dolt db connector: %w", err)
	}
	return sql.OpenDB(connector), connector, nil
}

func prepareRoot(root string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("create root: %w", err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", fmt.Errorf("read root: %w", err)
	}
	if len(entries) > 0 {
		return "", fmt.Errorf("root %s must be empty", abs)
	}
	return abs, nil
}

func openConfiguredServer(journalPath, blobRoot, authDBPath string) (*configuredServer, error) {
	authStore, err := auth.OpenStore(authDBPath)
	if err != nil {
		return nil, fmt.Errorf("open auth store: %w", err)
	}
	persistent, err := api.OpenPersistentHandler(api.PersistentHandlerConfig{JournalPath: journalPath, BlobRoot: blobRoot}, authStore)
	if err != nil {
		_ = authStore.Close()
		return nil, err
	}
	srv := server.NewServer("evidenceroot", "0")
	if err := api.RegisterPersistentRoutes(srv, persistent); err != nil {
		_ = persistent.Close()
		_ = authStore.Close()
		return nil, err
	}
	return &configuredServer{server: srv, authStore: authStore, persistent: persistent, journalPath: journalPath, blobRoot: blobRoot, authDBPath: authDBPath}, nil
}

func (s *configuredServer) seedFixture(ctx context.Context) (baseFixtureOutput, error) {
	user, err := s.authStore.CreateUser("evidenceroot-fixture-user", "evidenceroot-fixture@example.com")
	if err != nil {
		return baseFixtureOutput{}, fmt.Errorf("create fixture user: %w", err)
	}
	_, secret, err := s.authStore.CreateAPIKey(ctx, user.ID, "evidenceroot-fixture", []string{api.ScopeReadBase, api.ScopeWriteBase}, nil)
	if err != nil {
		return baseFixtureOutput{}, fmt.Errorf("create fixture api key: %w", err)
	}
	blobResp, err := s.putFixtureBlob(secret, []byte("evidenceroot fixture bytes"))
	if err != nil {
		return baseFixtureOutput{}, err
	}
	itemID := model.ItemID("base_item_evidenceroot_fixture")
	itemBody, err := json.Marshal(map[string]any{
		"item_id":        itemID,
		"event_type":     model.EventCreate,
		"kind":           model.KindFile,
		"name":           "evidenceroot-fixture.txt",
		"parent_item_id": "base_item_root",
		"version_id":     "base_ver_evidenceroot_fixture",
		"blob_ref":       blobResp.BlobRef,
		"content_hash":   blobResp.SHA256,
		"media_type":     "text/plain",
	})
	if err != nil {
		return baseFixtureOutput{}, fmt.Errorf("encode fixture item: %w", err)
	}
	if err := s.doFixtureRequest(http.MethodPost, "/api/base/items", secret, bytes.NewReader(itemBody), nil); err != nil {
		return baseFixtureOutput{}, err
	}
	return baseFixtureOutput{JournalPath: s.journalPath, BlobRoot: s.blobRoot, AuthDBPath: s.authDBPath, ItemID: itemID, BlobRef: blobResp.BlobRef, SHA256: blobResp.SHA256}, nil
}

func (s *configuredServer) putFixtureBlob(secret string, data []byte) (fixtureBlobResponse, error) {
	var resp fixtureBlobResponse
	if err := s.doFixtureRequest(http.MethodPost, "/api/base/blobs", secret, bytes.NewReader(data), &resp); err != nil {
		return fixtureBlobResponse{}, err
	}
	if resp.BlobRef == "" || resp.SHA256 == "" {
		return fixtureBlobResponse{}, fmt.Errorf("incomplete fixture blob response: %#v", resp)
	}
	return resp, nil
}

func (s *configuredServer) doFixtureRequest(method, path, secret string, body *bytes.Reader, out any) error {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+secret)
	if method == http.MethodPost && strings.HasSuffix(path, "/items") {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.server.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		return fmt.Errorf("%s %s status %d: %s", method, path, rr.Code, rr.Body.String())
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rr.Body.Bytes(), out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}

func (s *configuredServer) Close() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.persistent.Close(), s.authStore.Close())
}

func seededMismatch(observation computerversion.ObservationSet) computerversion.EquivalenceResult {
	tampered := observation
	tampered.Name = observation.Name + ":seeded-mismatch"
	tampered.Observations = append([]computerversion.Observation(nil), observation.Observations...)
	for i := range tampered.Observations {
		if tampered.Observations[i].Kind == computerversion.ObservationVMStateManifest {
			tampered.Observations[i].Value += ":tampered"
			break
		}
	}
	return computerversion.EquivalenceChecker{}.CheckObservationSets(observation, tampered)
}
