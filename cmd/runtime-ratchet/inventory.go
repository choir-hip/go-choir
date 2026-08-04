package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	inventorySchema      = "runtime-dissolution-inventory/v6"
	retiredRuntimeImport = "github.com/yusefmosiah/go-choir/internal/runtime"
	agentCoreImport      = "github.com/yusefmosiah/go-choir/internal/agentcore"
	textureOwnerImport   = "github.com/yusefmosiah/go-choir/internal/textureowner"
	toolRegistryImport   = "github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func isDissolutionImport(path string) bool {
	return path == agentCoreImport || strings.HasPrefix(path, agentCoreImport+"/") ||
		path == textureOwnerImport || strings.HasPrefix(path, textureOwnerImport+"/")
}

type Inventory struct {
	Schema               string  `yaml:"schema"`
	CanonicalParent      string  `yaml:"canonical_parent"`
	DispatchNonce        string  `yaml:"dispatch_nonce"`
	Transition           string  `yaml:"transition"`
	Counts               Counts  `yaml:"counts"`
	Files                []Entry `yaml:"files"`
	Exports              []Entry `yaml:"exports"`
	UnusedExportDebt     []Entry `yaml:"initial_unused_export_debt"`
	Routes               []Entry `yaml:"routes"`
	Tools                []Entry `yaml:"tools"`
	ProductionImporters  []Entry `yaml:"production_importers"`
	Wrappers             []Entry `yaml:"wrappers"`
	CompatibilityMarkers []Entry `yaml:"compatibility_markers"`
	StoreCalls           []Entry `yaml:"store_calls"`
	InterfaceCandidates  []Entry `yaml:"interface_candidates"`
	LegacyStateWriters   []Entry `yaml:"state_writers,omitempty"`
	LegacyStoreReads     []Entry `yaml:"declared_store_reads,omitempty"`
	Citers               []Entry `yaml:"citers"`
}

type Counts struct {
	GoFiles                 int `yaml:"go_files"`
	ProductionFiles         int `yaml:"production_files"`
	TestFiles               int `yaml:"test_files"`
	ProductionLOC           int `yaml:"production_loc"`
	TestLOC                 int `yaml:"test_loc"`
	Exports                 int `yaml:"exports"`
	ExportCallerEdges       int `yaml:"export_caller_edges"`
	InitialUnusedExportDebt int `yaml:"initial_unused_export_debt"`
	Routes                  int `yaml:"routes"`
	Tools                   int `yaml:"tools"`
	ProductionImporters     int `yaml:"production_importers"`
	Wrappers                int `yaml:"wrappers"`
	CompatibilityMarkers    int `yaml:"compatibility_markers"`
	StoreCalls              int `yaml:"store_calls"`
	InterfaceCandidates     int `yaml:"interface_candidates"`
	LegacyStateWriters      int `yaml:"state_writers,omitempty"`
	LegacyStoreReads        int `yaml:"declared_store_reads,omitempty"`
	Citers                  int `yaml:"citers"`
}

type Entry struct {
	ID                string   `yaml:"id"`
	Disposition       string   `yaml:"disposition"`
	LOC               int      `yaml:"loc,omitempty"`
	ProductionCallers []string `yaml:"production_callers,omitempty"`
}

// SupervisionMutationCallerCategory is the closed source-level disposition
// vocabulary for a production Texture/lifecycle semantic write.
type SupervisionMutationCallerCategory string

const (
	SupervisionCanonicalService               SupervisionMutationCallerCategory = "canonical_service"
	SupervisionReducerPrivateProjection       SupervisionMutationCallerCategory = "reducer_private_projection"
	SupervisionDerivedCompatibilityProjection SupervisionMutationCallerCategory = "derived_compatibility_projection"
	SupervisionMigrationOnly                  SupervisionMutationCallerCategory = "migration_only"
	SupervisionUnrelatedLedger                SupervisionMutationCallerCategory = "unrelated_ledger"
	SupervisionDeterministicRefusal           SupervisionMutationCallerCategory = "deterministic_refusal"
	SupervisionUnauthorizedWriter             SupervisionMutationCallerCategory = "unauthorized_semantic_writer"
	SupervisionIndependentLifecycleEvent      SupervisionMutationCallerCategory = "independent_lifecycle_event_writer"
)

// SupervisionMutationCaller identifies one non-test source caller of a
// Texture/lifecycle semantic write. It deliberately records source identity,
// rather than attempting to infer runtime data flow, so that a new alternate
// writer is visible at review time.
type SupervisionMutationCaller struct {
	ID       string
	Category SupervisionMutationCallerCategory
}

var supervisionSemanticMutationMethods = map[string]bool{
	"ApplyLifecycleUpdateWithSourceGraph":        true,
	"AppendChannelMessage":                       true,
	"ArchiveLifecycleArtifact":                   true,
	"ArchiveTextureDocumentAuthority":            true,
	"CancelLifecycleTrajectory":                  true,
	"CommitLifecycleArtifactHead":                true,
	"CreateDocument":                             true,
	"CreateEvidence":                             true,
	"CreateRevision":                             true,
	"CreateTextureDecision":                      true,
	"CreateTrajectoryIfAbsent":                   true,
	"CreateWorkItem":                             true,
	"ProjectTerminalLifecycleRun":                true,
	"QueueLifecycleUpdate":                       true,
	"RecordLifecycleRefs":                        true,
	"ReconcileLifecycleSettlementForTerminalRun": true,
	"ReplaceLifecycleActivation":                 true,
	"SettleLifecycleTrajectory":                  true,
	"StartLifecycle":                             true,
	"UpdateDocument":                             true,
	"UpdateLifecycleDocumentTitleAuthority":      true,
	"UpdateTextureDocumentTitleAuthority":        true,
	"UpdateWorkItemDetails":                      true,
	"UpdateWorkItemStatus":                       true,
	"UpsertDocumentAlias":                        true,
	"allocateTextureManifestPath":                true,
}

func scanSupervisionMutationCallers(root string, files []string) ([]SupervisionMutationCaller, error) {
	fset := token.NewFileSet()
	callers := []SupervisionMutationCaller{}
	for _, path := range files {
		if filepath.Ext(path) != ".go" {
			continue
		}
		rel := slashRel(root, path)
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s for supervision mutation authority: %w", rel, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method := calledMethod(call)
			if method == "" {
				return true
			}
			category, tracked := classifySupervisionMutationCaller(rel, file, call, method)
			if !tracked {
				return true
			}
			position := fset.Position(call.Pos())
			id := rel + ":" + enclosingFunction(file, call.Pos()) + ":" + method + ":" + strconv.Itoa(position.Line)
			callers = append(callers, SupervisionMutationCaller{ID: id, Category: category})
			return true
		})
	}
	sort.Slice(callers, func(i, j int) bool { return callers[i].ID < callers[j].ID })
	return callers, nil
}

func calledMethod(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	default:
		return ""
	}
}

func classifySupervisionMutationCaller(rel string, file *ast.File, call *ast.CallExpr, method string) (SupervisionMutationCallerCategory, bool) {
	if method == "lifecycleObject" && len(call.Args) > 0 && exprString(call.Args[0]) == "ogKindLifecycleEvent" {
		if strings.HasPrefix(rel, "internal/store/supervision_projection") {
			return SupervisionReducerPrivateProjection, true
		}
		return SupervisionIndependentLifecycleEvent, true
	}
	if method == "AppendNewSupervisionTransaction" {
		if strings.HasPrefix(rel, "internal/agentcore/") || rel == "internal/computerevent/appender.go" {
			return SupervisionCanonicalService, true
		}
		return SupervisionUnauthorizedWriter, true
	}
	if method == "AppendSupervisionTransaction" {
		if strings.HasPrefix(rel, "internal/agentcore/") || strings.HasPrefix(rel, "internal/textureowner/") || rel == "internal/computerevent/appender.go" {
			return SupervisionCanonicalService, true
		}
		return SupervisionUnauthorizedWriter, true
	}
	if !supervisionSemanticMutationMethods[method] {
		return "", false
	}
	if strings.HasPrefix(rel, "internal/store/supervision_projection") {
		return SupervisionReducerPrivateProjection, true
	}
	if rel == "internal/agentcore/wire_publication.go" && wirePublicationLedgerMutationMethods[method] {
		return SupervisionUnrelatedLedger, true
	}
	if isDerivedCompatibilityProjection(rel, file, call, method) {
		return SupervisionDerivedCompatibilityProjection, true
	}
	if isLegacyProjectionImportBuilder(rel, file, call.Pos()) {
		return SupervisionMigrationOnly, true
	}
	if functionRefusesSupervisionAuthority(file, call.Pos()) {
		return SupervisionDeterministicRefusal, true
	}
	if strings.HasPrefix(rel, "internal/agentcore/") || strings.HasPrefix(rel, "internal/textureowner/") || strings.HasPrefix(rel, "internal/store/") {
		return SupervisionUnauthorizedWriter, true
	}
	return SupervisionUnrelatedLedger, true
}

var wirePublicationLedgerMutationMethods = map[string]bool{
	"CreateWorkItem":        true,
	"UpdateWorkItemDetails": true,
	"UpdateWorkItemStatus":  true,
}

func isDerivedCompatibilityProjection(rel string, file *ast.File, call *ast.CallExpr, method string) bool {
	if rel != "internal/textureowner/texture_import.go" || !isEventDerivedTextureManifest(file, call.Pos()) {
		return false
	}
	if method == "allocateTextureManifestPath" {
		return true
	}
	return method == "UpsertDocumentAlias" &&
		len(call.Args) == 5 &&
		exprString(call.Args[2]) == "sourcePath" &&
		exprString(call.Args[3]) == "doc.DocID"
}

func isEventDerivedTextureManifest(file *ast.File, pos token.Pos) bool {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name != "ensureTextureManifest" ||
			function.Pos() > pos || pos > function.End() ||
			function.Type.Params == nil || len(function.Type.Params.List) != 4 {
			continue
		}
		document := function.Type.Params.List[3]
		if len(document.Names) == 1 && document.Names[0].Name == "doc" && exprString(document.Type) == "types.Document" {
			return true
		}
	}
	return false
}

func isLegacyProjectionImportBuilder(rel string, file *ast.File, pos token.Pos) bool {
	return rel == "internal/store/projection_import.go" &&
		enclosingFunction(file, pos) == "BuildProjectionImportV1"
}

func functionRefusesSupervisionAuthority(file *ast.File, pos token.Pos) bool {
	var scope *ast.BlockStmt
	ast.Inspect(file, func(node ast.Node) bool {
		switch function := node.(type) {
		case *ast.FuncDecl:
			if function.Body != nil && function.Pos() <= pos && pos <= function.End() {
				scope = function.Body
			}
		case *ast.FuncLit:
			if function.Body != nil && function.Pos() <= pos && pos <= function.End() {
				scope = function.Body
			}
		}
		return true
	})
	return scope != nil && refusalGuardDominatesMutation(scope.List, pos)
}

func refusalGuardDominatesMutation(statements []ast.Stmt, pos token.Pos) bool {
	dominates := false
	for _, statement := range statements {
		if statement.End() < pos {
			dominates = dominates || isLegacyRefusalGuard(statement)
			continue
		}
		if statement.Pos() <= pos && pos <= statement.End() {
			return dominates || refusalGuardDominatesWithin(statement, pos)
		}
	}
	return false
}

func refusalGuardDominatesWithin(statement ast.Stmt, pos token.Pos) bool {
	switch statement := statement.(type) {
	case *ast.BlockStmt:
		return refusalGuardDominatesMutation(statement.List, pos)
	case *ast.IfStmt:
		if statement.Body != nil && statement.Body.Pos() <= pos && pos <= statement.Body.End() {
			return refusalGuardDominatesMutation(statement.Body.List, pos)
		}
		if statement.Else != nil && statement.Else.Pos() <= pos && pos <= statement.Else.End() {
			return refusalGuardDominatesWithin(statement.Else, pos)
		}
	case *ast.ForStmt:
		if statement.Body != nil && statement.Body.Pos() <= pos && pos <= statement.Body.End() {
			return refusalGuardDominatesMutation(statement.Body.List, pos)
		}
	case *ast.RangeStmt:
		if statement.Body != nil && statement.Body.Pos() <= pos && pos <= statement.Body.End() {
			return refusalGuardDominatesMutation(statement.Body.List, pos)
		}
	case *ast.SwitchStmt:
		for _, clause := range statement.Body.List {
			if clause.Pos() <= pos && pos <= clause.End() {
				return refusalGuardDominatesWithin(clause, pos)
			}
		}
	case *ast.TypeSwitchStmt:
		for _, clause := range statement.Body.List {
			if clause.Pos() <= pos && pos <= clause.End() {
				return refusalGuardDominatesWithin(clause, pos)
			}
		}
	case *ast.SelectStmt:
		for _, clause := range statement.Body.List {
			if clause.Pos() <= pos && pos <= clause.End() {
				return refusalGuardDominatesWithin(clause, pos)
			}
		}
	case *ast.CaseClause:
		return refusalGuardDominatesMutation(statement.Body, pos)
	case *ast.CommClause:
		return refusalGuardDominatesMutation(statement.Body, pos)
	case *ast.LabeledStmt:
		return refusalGuardDominatesWithin(statement.Stmt, pos)
	}
	return false
}

func isLegacyRefusalGuard(statement ast.Stmt) bool {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || !blockAlwaysReturns(guard.Body) {
		return false
	}
	if assignment, ok := guard.Init.(*ast.AssignStmt); ok && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 {
		identifier, ok := assignment.Lhs[0].(*ast.Ident)
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		return ok && callIsLegacyRefusal(call) && isErrorCheckForIdentifier(guard.Cond, identifier.Name)
	}
	call, ok := errorCheckCall(guard.Cond)
	return ok && callIsLegacyRefusal(call)
}

func callIsLegacyRefusal(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return selector.Sel.Name == "refuseLegacySupervisionWrite" || selector.Sel.Name == "refuseLegacyTextureWriter"
}

func isErrorCheckForIdentifier(condition ast.Expr, identifier string) bool {
	binary, ok := condition.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}
	return (isIdentifier(binary.X, identifier) && isNil(binary.Y)) || (isNil(binary.X) && isIdentifier(binary.Y, identifier))
}

func errorCheckCall(condition ast.Expr) (*ast.CallExpr, bool) {
	binary, ok := condition.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return nil, false
	}
	if call, ok := binary.X.(*ast.CallExpr); ok && isNil(binary.Y) {
		return call, true
	}
	if call, ok := binary.Y.(*ast.CallExpr); ok && isNil(binary.X) {
		return call, true
	}
	return nil, false
}

func isIdentifier(expression ast.Expr, want string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == want
}

func isNil(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "nil"
}

func blockAlwaysReturns(block *ast.BlockStmt) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.List {
		switch statement := statement.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.BlockStmt:
			if blockAlwaysReturns(statement) {
				return true
			}
		case *ast.IfStmt:
			if statement.Else != nil && blockAlwaysReturns(statement.Body) && statementAlwaysReturns(statement.Else) {
				return true
			}
		}
	}
	return false
}

func statementAlwaysReturns(statement ast.Stmt) bool {
	switch statement := statement.(type) {
	case *ast.BlockStmt:
		return blockAlwaysReturns(statement)
	case *ast.ReturnStmt:
		return true
	case *ast.IfStmt:
		return statement.Else != nil && blockAlwaysReturns(statement.Body) && statementAlwaysReturns(statement.Else)
	default:
		return false
	}
}

func validateSupervisionMutationCallers(callers []SupervisionMutationCaller) error {
	var violations []string
	for _, caller := range callers {
		switch caller.Category {
		case SupervisionCanonicalService, SupervisionReducerPrivateProjection, SupervisionDerivedCompatibilityProjection, SupervisionMigrationOnly, SupervisionUnrelatedLedger, SupervisionDeterministicRefusal:
			continue
		default:
			violations = append(violations, caller.ID+": "+string(caller.Category))
		}
	}
	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("supervision mutation authority violations:\n  - %s", strings.Join(violations, "\n  - "))
}

var compatibilityRE = regexp.MustCompile(`(?i)\b(deprecated|compatib(?:ility|le)|legacy|old runtime|new runtime)\b`)

func scanRepository(root string) (Inventory, error) {
	inv := Inventory{
		Schema:          inventorySchema,
		CanonicalParent: "0f905ffcfeba3db85f0958382d9beb68f013a498",
		DispatchNonce:   "s0-runtime-inventory-ratchet-01-nonce-01",
		Transition:      "s0-runtime-inventory-ratchet-dispatch-intent-01",
	}
	files, err := repositoryFiles(root)
	if err != nil {
		return Inventory{}, err
	}
	citerOrdinals := map[string]int{}
	typePackages := map[string]bool{}
	if err := scanGo(root, files, citerOrdinals, typePackages, &inv); err != nil {
		return Inventory{}, err
	}
	if err := scanTextCiters(root, files, citerOrdinals, &inv); err != nil {
		return Inventory{}, err
	}
	exportUses, storeCalls, interfaceCandidates, err := scanTypeAwareInventory(root, typePackages)
	if err != nil {
		return Inventory{}, err
	}
	attachProductionCallers(&inv, exportUses)
	inv.StoreCalls = storeCalls
	inv.InterfaceCandidates = interfaceCandidates
	seedUnusedExportDebt(&inv)
	sortInventory(&inv)
	setCounts(&inv)
	return inv, nil
}

func repositoryFiles(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		cmd := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("list non-ignored repository files: %w", err)
		}
		parts := bytes.Split(output, []byte{0})
		files := make([]string, 0, len(parts))
		for _, part := range parts {
			if len(part) == 0 {
				continue
			}
			path := filepath.Join(root, filepath.FromSlash(string(part)))
			info, statErr := os.Stat(path)
			if statErr == nil && !info.IsDir() {
				files = append(files, path)
			}
		}
		sort.Strings(files)
		return files, nil
	}

	ignoredDirectories := map[string]bool{
		".git": true, ".cache": true, "build": true, "coverage": true,
		"dist": true, "node_modules": true, "vendor": true,
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (ignoredDirectories[entry.Name()] || strings.HasPrefix(entry.Name(), ".runtime-ratchet-")) {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, err
}

func scanGo(root string, files []string, citerOrdinals map[string]int, typePackages map[string]bool, inv *Inventory) error {
	fset := token.NewFileSet()
	for _, path := range files {
		if filepath.Ext(path) != ".go" {
			continue
		}
		rel := slashRel(root, path)
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		isTest := strings.HasSuffix(rel, "_test.go")
		if !isTest {
			if err := scanProductionTools(rel, file, inv); err != nil {
				return err
			}
		}
		inOwnershipPackage := rel == "internal/agentcore" || strings.HasPrefix(rel, "internal/agentcore/") ||
			rel == "internal/textureowner" || strings.HasPrefix(rel, "internal/textureowner/")
		if inOwnershipPackage {
			loc := countLines(src)
			kind := "production"
			if isTest {
				kind = "test"
			}
			inv.Files = append(inv.Files, Entry{ID: rel + " [" + kind + "]", Disposition: domainDisposition(rel), LOC: loc})
			if err := scanRuntimeAST(rel, file, fset, isTest, inv); err != nil {
				return err
			}
		}
		if !isTest {
			typePackages[filepath.Dir(rel)] = true
		}
		retiredImports := retiredRuntimeImports(file)
		if !isTest && len(retiredImports) > 0 {
			inv.ProductionImporters = append(inv.ProductionImporters, Entry{ID: rel, Disposition: "delete"})
		}
		if !isTest {
			scanWrappers(rel, file, dissolutionImports(file), inv)
		}
		scanGoCommentCiters(rel, file, citerOrdinals, inv)
	}
	return nil
}

func scanRuntimeAST(rel string, file *ast.File, fset *token.FileSet, isTest bool, inv *Inventory) error {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(d.Name.Name) {
				kind := "func"
				if d.Recv != nil && len(d.Recv.List) > 0 {
					if !exportedReceiverType(d.Recv.List[0].Type) {
						continue
					}
					kind = "method(" + exprString(d.Recv.List[0].Type) + ")"
				}
				inv.Exports = append(inv.Exports, Entry{ID: rel + ":" + kind + ":" + d.Name.Name, Disposition: domainDisposition(rel)})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if ast.IsExported(s.Name.Name) {
						inv.Exports = append(inv.Exports, Entry{ID: rel + ":type:" + s.Name.Name, Disposition: domainDisposition(rel)})
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if ast.IsExported(name.Name) {
							inv.Exports = append(inv.Exports, Entry{ID: rel + ":" + strings.ToLower(d.Tok.String()) + ":" + name.Name, Disposition: domainDisposition(rel)})
						}
					}
				}
			}
		}
	}

	if isTest {
		return nil
	}
	ordinals := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if (sel.Sel.Name == "HandleFunc" || sel.Sel.Name == "Handle") && len(call.Args) >= 2 {
			if route, ok := stringLiteral(call.Args[0]); ok {
				id := rel + ":" + sel.Sel.Name + ":" + route + ":" + exprString(call.Args[1])
				inv.Routes = append(inv.Routes, Entry{ID: uniqueID(id, ordinals), Disposition: domainDisposition(rel)})
			}
		}
		return true
	})
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
			if compatibilityRE.MatchString(text) {
				id := rel + ":" + strconv.Itoa(fset.Position(c.Pos()).Line) + ":" + oneLine(text)
				inv.CompatibilityMarkers = append(inv.CompatibilityMarkers, Entry{ID: id, Disposition: "delete"})
			}
		}
	}
	return nil
}
func exportedReceiverType(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.Ident:
		return ast.IsExported(typed.Name)
	case *ast.StarExpr:
		return exportedReceiverType(typed.X)
	case *ast.IndexExpr:
		return exportedReceiverType(typed.X)
	case *ast.IndexListExpr:
		return exportedReceiverType(typed.X)
	default:
		return false
	}
}

func scanProductionTools(rel string, file *ast.File, inv *Inventory) error {
	aliases := toolRegistryAliases(file)
	if len(aliases) == 0 {
		return nil
	}
	ordinals := map[string]int{}
	var scanErr error
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isToolRegistryCompositeType(lit.Type, aliases) {
			return true
		}
		name, ok := toolName(lit)
		if !ok {
			scanErr = fmt.Errorf("%s: toolregistry.Tool Name must be a string literal or file-local string constant", rel)
			return false
		}
		id := rel + ":Tool:" + name
		inv.Tools = append(inv.Tools, Entry{ID: uniqueID(id, ordinals), Disposition: domainDisposition(rel)})
		return true
	})
	return scanErr
}

func toolRegistryAliases(file *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != toolRegistryImport {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" {
			aliases[name] = true
		}
	}
	return aliases
}

func isToolRegistryCompositeType(expr ast.Expr, aliases map[string]bool) bool {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && aliases[id.Name] && x.Sel.Name == "Tool"
	case *ast.Ident:
		return aliases["."] && x.Name == "Tool" && x.Obj == nil
	default:
		return false
	}
}

// scanWrappers inventories owner types that are re-exported through aliases or
// promoted through anonymous embedding. Named fields and function parameters are
// direct composition, not wrappers, and must remain outside this detector.
func scanWrappers(rel string, file *ast.File, aliases map[string]string, inv *Inventory) {
	ordinals := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		var typ ast.Expr
		var label string
		switch x := n.(type) {
		case *ast.Field:
			if len(x.Names) == 0 {
				typ = x.Type
				label = "embedded"
			}
		case *ast.TypeSpec:
			if x.Assign.IsValid() {
				typ = x.Type
				label = "alias:" + x.Name.Name
			}
		}
		if typ == nil {
			return true
		}
		if target := runtimeSurfaceType(typ, aliases); target != "" {
			id := rel + ":" + label + ":" + target
			inv.Wrappers = append(inv.Wrappers, Entry{ID: uniqueID(id, ordinals), Disposition: "delete"})
		}
		return true
	})
}

// retiredRuntimeImports is intentionally independent from owner-wrapper
// discovery: any return of the extinct import path remains a production error.
func retiredRuntimeImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || (path != retiredRuntimeImport && !strings.HasPrefix(path, retiredRuntimeImport+"/")) {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = path
	}
	return imports
}

func dissolutionImports(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !isDissolutionImport(path) {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" {
			imports[name] = path
		}
	}
	return imports
}
func scanTypeAwareInventory(root string, packageDirs map[string]bool) (map[string]map[string]bool, []Entry, []Entry, error) {
	patterns := make([]string, 0, len(packageDirs))
	for dir := range packageDirs {
		if dir == "." {
			patterns = append(patterns, ".")
		} else {
			patterns = append(patterns, "./"+filepath.ToSlash(dir))
		}
	}
	sort.Strings(patterns)
	uses := map[string]map[string]bool{}
	calls := map[string]Entry{}
	candidates := map[string]Entry{}
	environments := [][]string{
		nil,
		append(os.Environ(), "GOOS=linux", "CGO_ENABLED=0"),
	}
	for _, environment := range environments {
		graph, err := listGoPackages(root, environment, patterns, false)
		if err != nil {
			return nil, nil, nil, err
		}
		var selectedPatterns []string
		for _, listedPackage := range graph {
			if !isLocalProductionPackage(root, listedPackage) || !dependsOnRuntime(listedPackage) {
				continue
			}
			relative, relErr := filepath.Rel(root, listedPackage.Dir)
			if relErr != nil || !packageDirs[filepath.ToSlash(relative)] {
				continue
			}
			selectedPatterns = append(selectedPatterns, "./"+filepath.ToSlash(relative))
		}
		sort.Strings(selectedPatterns)
		listed, err := listGoPackages(root, environment, selectedPatterns, true)
		if err != nil {
			return nil, nil, nil, err
		}
		exports := make(map[string]string, len(listed))
		for _, listedPackage := range listed {
			if listedPackage.Export != "" {
				exports[listedPackage.ImportPath] = listedPackage.Export
			}
		}
		for _, listedPackage := range listed {
			if !isLocalProductionPackage(root, listedPackage) || !dependsOnRuntime(listedPackage) {
				continue
			}
			relative, relErr := filepath.Rel(root, listedPackage.Dir)
			if relErr != nil || !packageDirs[filepath.ToSlash(relative)] {
				continue
			}
			if listedPackage.Error != nil {
				return nil, nil, nil, fmt.Errorf(
					"type-check production package %s: %s",
					listedPackage.ImportPath, listedPackage.Error.Err,
				)
			}
			if err := collectTypedUses(root, environment, listedPackage, exports, uses, calls, candidates); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	storeCalls := make([]Entry, 0, len(calls))
	for _, call := range calls {
		storeCalls = append(storeCalls, call)
	}
	sort.Slice(storeCalls, func(i, j int) bool { return storeCalls[i].ID < storeCalls[j].ID })
	interfaceCandidates := make([]Entry, 0, len(candidates))
	for _, candidate := range candidates {
		interfaceCandidates = append(interfaceCandidates, candidate)
	}
	sort.Slice(interfaceCandidates, func(i, j int) bool {
		return interfaceCandidates[i].ID < interfaceCandidates[j].ID
	})
	return uses, storeCalls, interfaceCandidates, nil
}

type goListPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	CgoFiles   []string
	Deps       []string
	Export     string
	Error      *struct {
		Err string
	}
}

func listGoPackages(root string, environment, patterns []string, withExports bool) ([]goListPackage, error) {
	args := []string{"list", "-e", "-deps", "-json"}
	if withExports {
		args = append(args, "-export")
	}
	args = append(args, patterns...)
	command := exec.Command("go", args...)
	command.Dir = root
	command.Env = environment
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list production packages: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var listed []goListPackage
	for {
		var listedPackage goListPackage
		if err := decoder.Decode(&listedPackage); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go list package graph: %w", err)
		}
		listed = append(listed, listedPackage)
	}
	return listed, nil
}

func isLocalProductionPackage(root string, listed goListPackage) bool {
	if len(listed.GoFiles)+len(listed.CgoFiles) == 0 || listed.Dir == "" {
		return false
	}
	relative, err := filepath.Rel(root, listed.Dir)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func dependsOnRuntime(listed goListPackage) bool {
	if isDissolutionImport(listed.ImportPath) {
		return true
	}
	for _, dependency := range listed.Deps {
		if isDissolutionImport(dependency) {
			return true
		}
	}
	return false
}
func collectTypedUses(root string, environment []string, listed goListPackage, exports map[string]string, uses map[string]map[string]bool, calls, candidates map[string]Entry) error {
	fset := token.NewFileSet()
	names := append(append([]string{}, listed.GoFiles...), listed.CgoFiles...)
	files := make([]*ast.File, 0, len(names))
	for _, name := range names {
		path := filepath.Join(listed.Dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse production package %s: %w", listed.ImportPath, err)
		}
		files = append(files, file)
	}
	lookup := func(importPath string) (io.ReadCloser, error) {
		exportPath, ok := exports[importPath]
		if !ok || exportPath == "" {
			resolve := func(candidateEnvironment []string) (string, error) {
				command := exec.Command("go", "list", "-export", "-f={{.Export}}", importPath)
				command.Dir = root
				command.Env = candidateEnvironment
				output, err := command.Output()
				if err != nil {
					return "", err
				}
				return strings.TrimSpace(string(output)), nil
			}
			var err error
			exportPath, err = resolve(environment)
			if (err != nil || exportPath == "") && environment != nil {
				exportPath, err = resolve(nil)
			}
			if err != nil {
				return nil, fmt.Errorf("resolve export data for %s: %w", importPath, err)
			}
			if exportPath == "" {
				return nil, fmt.Errorf("missing export data for %s", importPath)
			}
			exports[importPath] = exportPath
		}
		return os.Open(exportPath)
	}
	typeInfo := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	gcImporter := importer.ForCompiler(fset, "gc", lookup)
	configuration := &types.Config{
		GoVersion: "go1.25",
		Importer:  gcImporter,
	}
	if _, err := configuration.Check(listed.ImportPath, fset, files, typeInfo); err != nil {
		return fmt.Errorf("type-check production package %s: %w", listed.ImportPath, err)
	}
	storeMethods, err := importedStoreMethods(gcImporter)
	if err != nil {
		return fmt.Errorf("load internal/store.Store methods: %w", err)
	}
	for identifier, object := range typeInfo.Uses {
		if object == nil || object.Pkg() == nil || !object.Exported() {
			continue
		}
		importPath := object.Pkg().Path()
		if !isDissolutionImport(importPath) {
			continue
		}
		position := fset.Position(identifier.Pos())
		key := typeObjectKey(object)
		if uses[key] == nil {
			uses[key] = map[string]bool{}
		}
		uses[key][slashRel(root, position.Filename)] = true
	}
	if isDissolutionImport(listed.ImportPath) {
		for _, file := range files {
			ordinals := map[string]int{}
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				selection := typeInfo.Selections[selector]
				if selection == nil {
					return true
				}
				function, ok := selection.Obj().(*types.Func)
				if !ok {
					return true
				}
				key, storeBacked := storeSelectionKey(selection, function, storeMethods)
				if !storeBacked {
					return true
				}
				position := fset.Position(selector.Pos())
				relative := slashRel(root, position.Filename)
				base := relative + ":" + enclosingFunction(file, selector.Pos()) + ":" + key
				id := uniqueID(base, ordinals)
				entry := Entry{ID: id}
				if strings.HasPrefix(key, "internal/store.interface:") {
					candidates[id] = entry
				} else {
					calls[id] = entry
				}
				return true
			})
		}
	}
	return nil
}

func typeObjectKey(object types.Object) string {
	if function, ok := object.(*types.Func); ok {
		signature, _ := function.Type().(*types.Signature)
		if signature != nil && signature.Recv() != nil {
			receiver := types.Unalias(signature.Recv().Type())
			if pointer, ok := receiver.(*types.Pointer); ok {
				receiver = types.Unalias(pointer.Elem())
			}
			if named, ok := receiver.(*types.Named); ok && named.Obj().Pkg() != nil {
				return named.Obj().Pkg().Path() + ".method:" + named.Obj().Name() + "." + object.Name()
			}
		}
	}
	return object.Pkg().Path() + ".object:" + object.Name()
}

func importedStoreMethods(typeImporter types.Importer) (map[string]*types.Func, error) {
	storePackage, err := typeImporter.Import("github.com/yusefmosiah/go-choir/internal/store")
	if err != nil {
		return map[string]*types.Func{}, nil
	}
	storeObject := storePackage.Scope().Lookup("Store")
	if storeObject == nil {
		return nil, errors.New("internal/store.Store type is missing")
	}
	storeType, ok := types.Unalias(storeObject.Type()).(*types.Named)
	if !ok {
		return nil, errors.New("internal/store.Store is not a named type")
	}
	methods := map[string]*types.Func{}
	methodSet := types.NewMethodSet(types.NewPointer(storeType))
	for index := range methodSet.Len() {
		method, ok := methodSet.At(index).Obj().(*types.Func)
		if ok {
			methods[method.Name()] = method
		}
	}
	return methods, nil
}

func storeSelectionKey(_ *types.Selection, function *types.Func, storeMethods map[string]*types.Func) (string, bool) {
	if isConcreteStoreMethod(function) {
		return "internal/store.method:Store." + function.Name(), true
	}
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return "", false
	}
	declaredReceiver := signature.Recv().Type()
	receiver := types.Unalias(declaredReceiver)
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	namedReceiver, ok := receiver.(*types.Named)
	if !ok || namedReceiver.Obj().Pkg() == nil ||
		!isDissolutionImport(namedReceiver.Obj().Pkg().Path()) {
		return "", false
	}
	if _, ok := namedReceiver.Underlying().(*types.Interface); !ok {
		return "", false
	}
	storeMethod := storeMethods[function.Name()]
	if storeMethod == nil || !sameCallableSignature(function, storeMethod) {
		return "", false
	}
	receiverName := types.TypeString(declaredReceiver, func(pkg *types.Package) string {
		return pkg.Path()
	})
	receiverName = strings.TrimPrefix(receiverName, "github.com/yusefmosiah/go-choir/")
	return "internal/store.interface:" + receiverName + "." + function.Name(), true
}

func isConcreteStoreMethod(function *types.Func) bool {
	signature, _ := function.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return false
	}
	receiver := types.Unalias(signature.Recv().Type())
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	return ok && named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == "github.com/yusefmosiah/go-choir/internal/store" &&
		named.Obj().Name() == "Store"
}

func sameCallableSignature(left, right *types.Func) bool {
	leftSignature, leftOK := left.Type().(*types.Signature)
	rightSignature, rightOK := right.Type().(*types.Signature)
	if !leftOK || !rightOK || leftSignature.Variadic() != rightSignature.Variadic() {
		return false
	}
	return types.Identical(leftSignature.Params(), rightSignature.Params()) &&
		types.Identical(leftSignature.Results(), rightSignature.Results())
}

func attachProductionCallers(inv *Inventory, uses map[string]map[string]bool) {
	const modulePrefix = "github.com/yusefmosiah/go-choir/"
	for index := range inv.Exports {
		entry := &inv.Exports[index]
		firstColon := strings.Index(entry.ID, ":")
		lastColon := strings.LastIndex(entry.ID, ":")
		if firstColon < 0 || lastColon <= firstColon || strings.Contains(entry.ID, "_test.go:") {
			continue
		}
		sourcePath := entry.ID[:firstColon]
		kind := entry.ID[firstColon+1 : lastColon]
		symbol := entry.ID[lastColon+1:]
		importPath := modulePrefix + filepath.ToSlash(filepath.Dir(sourcePath))
		key := importPath + ".object:" + symbol
		if strings.HasPrefix(kind, "method(") && strings.HasSuffix(kind, ")") {
			receiver := strings.TrimSuffix(strings.TrimPrefix(kind, "method("), ")")
			receiver = strings.TrimPrefix(receiver, "*")
			if bracket := strings.Index(receiver, "["); bracket >= 0 {
				receiver = receiver[:bracket]
			}
			if dot := strings.LastIndex(receiver, "."); dot >= 0 {
				receiver = receiver[dot+1:]
			}
			key = importPath + ".method:" + receiver + "." + symbol
		}
		for caller := range uses[key] {
			entry.ProductionCallers = append(entry.ProductionCallers, caller)
		}
		sort.Strings(entry.ProductionCallers)
	}
}

func runtimeSurfaceType(expr ast.Expr, aliases map[string]string) string {
	switch x := expr.(type) {
	case *ast.StarExpr:
		return runtimeSurfaceType(x.X, aliases)
	case *ast.ParenExpr:
		return runtimeSurfaceType(x.X, aliases)
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		if !ok {
			return ""
		}
		_, imported := aliases[id.Name]
		if imported && (x.Sel.Name == "Runtime" || x.Sel.Name == "APIHandler" || x.Sel.Name == "Handler") {
			return id.Name + "." + x.Sel.Name
		}
	case *ast.Ident:
		if _, imported := aliases["."]; imported &&
			(x.Name == "Runtime" || x.Name == "APIHandler" || x.Name == "Handler") {
			return x.Name
		}
	}
	return ""
}

func toolName(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Name" {
			continue
		}
		if value, ok := stringLiteral(kv.Value); ok {
			return value, true
		}
		id, ok := kv.Value.(*ast.Ident)
		if !ok || id.Obj == nil || id.Obj.Kind != ast.Con {
			return "", false
		}
		spec, ok := id.Obj.Decl.(*ast.ValueSpec)
		if !ok {
			return "", false
		}
		for i, name := range spec.Names {
			if name.Obj != id.Obj || i >= len(spec.Values) {
				continue
			}
			return stringLiteral(spec.Values[i])
		}
		return "", false
	}
	return "", false
}

func scanGoCommentCiters(rel string, file *ast.File, citerOrdinals map[string]int, inv *Inventory) {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "internal/runtime") {
				addCiter(inv, rel, c.Text, citerOrdinals)
			}
		}
	}
}

func scanTextCiters(root string, files []string, citerOrdinals map[string]int, inv *Inventory) error {
	for _, path := range files {
		rel := slashRel(root, path)
		textSurface := isCiterSurface(rel)
		codeSurface := isCodeSurface(rel)
		if rel == "docs/runtime-dissolution-inventory.yaml" || filepath.Ext(rel) == ".go" || (!textSurface && !codeSurface) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			text := scanner.Text()
			if strings.Contains(text, "internal/runtime") && (textSurface || looksLikeComment(text)) {
				addCiter(inv, rel, text, citerOrdinals)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("scan citer surface %s: %w", rel, err)
		}
	}
	return nil
}

func addCiter(inv *Inventory, rel, text string, ordinals map[string]int) {
	base := rel + ":" + oneLine(text)
	inv.Citers = append(inv.Citers, Entry{
		ID:          uniqueID(base, ordinals),
		Disposition: citerDisposition(rel),
	})
}

func isCiterSurface(rel string) bool {
	if rel == "AGENTS.md" || strings.HasPrefix(rel, "docs/") || strings.HasPrefix(rel, "specs/") || strings.HasPrefix(rel, "skills/") || strings.HasPrefix(rel, ".github/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".yaml", ".yml", ".json", ".toml":
		return true
	}
	return false
}

func isCodeSurface(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".c", ".cc", ".cpp", ".css", ".graphql", ".h", ".hpp", ".html", ".java",
		".js", ".jsx", ".kt", ".m", ".proto", ".py", ".rb", ".rs", ".sh", ".sql",
		".svelte", ".swift", ".ts", ".tsx", ".vue", ".xml":
		return true
	}
	return false
}

func looksLikeComment(line string) bool {
	pathIndex := strings.Index(line, "internal/runtime")
	if pathIndex < 0 {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(line), "*") {
		return true
	}
	for _, marker := range []string{"//", "#", "/*", "<!--", "--"} {
		if markerIndex := strings.Index(line, marker); markerIndex >= 0 && markerIndex < pathIndex {
			return true
		}
	}
	return false
}

func domainDisposition(path string) string {
	name := strings.ToLower(path)
	domains := []struct {
		domain string
		terms  []string
	}{
		{"promotion", []string{"promotion", "computer_version"}},
		{"wire", []string{"wire"}},
		{"texture", []string{"texture"}},
		{"browser", []string{"browser"}},
		{"desktop", []string{"desktop"}},
		{"content", []string{"content"}},
		{"media", []string{"media"}},
		{"podcast", []string{"podcast"}},
		{"research", []string{"research", "search_gateway"}},
		{"evidence", []string{"evidence"}},
		{"model", []string{"model", "prompt"}},
		{"tools", []string{"tool"}},
		{"api", []string{"/api.go", "/api_"}},
		{"lifecycle", []string{"/runtime.go", "runtime_", "run_", "channel_store"}},
	}
	for _, candidate := range domains {
		for _, term := range candidate.terms {
			if strings.Contains(name, term) {
				return candidate.domain
			}
		}
	}
	return "core"
}

func citerDisposition(rel string) string {
	if strings.HasPrefix(rel, "docs/evidence/") || strings.HasPrefix(rel, "docs/archive/") {
		return "historical_evidence"
	}
	return "block"
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func exprString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	case *ast.IndexExpr:
		return exprString(x.X) + "[" + exprString(x.Index) + "]"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func enclosingFunction(file *ast.File, pos token.Pos) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Pos() <= pos && pos <= fn.End() {
			return fn.Name.Name
		}
	}
	return "package"
}

func uniqueID(base string, seen map[string]int) string {
	seen[base]++
	if seen[base] == 1 {
		return base
	}
	return base + "#" + strconv.Itoa(seen[base])
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}

func oneLine(s string) string {
	normalized := strings.Join(strings.Fields(s), " ")
	runes := []rune(normalized)
	if len(runes) <= 240 {
		return normalized
	}
	digest := sha256.Sum256([]byte(normalized))
	return string(runes[:200]) + "… [sha256:" + fmt.Sprintf("%x", digest) + "]"
}

func slashRel(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

func sortInventory(inv *Inventory) {
	lists := []*[]Entry{&inv.Files, &inv.Exports, &inv.UnusedExportDebt, &inv.Routes, &inv.Tools, &inv.ProductionImporters, &inv.Wrappers, &inv.CompatibilityMarkers, &inv.StoreCalls, &inv.InterfaceCandidates, &inv.Citers}
	for _, list := range lists {
		sort.Slice(*list, func(i, j int) bool { return (*list)[i].ID < (*list)[j].ID })
	}
}
func seedUnusedExportDebt(inv *Inventory) {
	for _, export := range inv.Exports {
		if strings.Contains(export.ID, "_test.go:") || len(export.ProductionCallers) > 0 {
			continue
		}
		inv.UnusedExportDebt = append(inv.UnusedExportDebt, Entry{
			ID:          export.ID,
			Disposition: "delete",
		})
	}
	sort.Slice(inv.UnusedExportDebt, func(i, j int) bool {
		return inv.UnusedExportDebt[i].ID < inv.UnusedExportDebt[j].ID
	})
}

func setCounts(inv *Inventory) {
	var c Counts
	c.GoFiles = len(inv.Files)
	for _, item := range inv.Files {
		if strings.HasSuffix(item.ID, " [test]") {
			c.TestFiles++
			c.TestLOC += item.LOC
		} else {
			c.ProductionFiles++
			c.ProductionLOC += item.LOC
		}
	}
	c.Exports = len(inv.Exports)
	c.InitialUnusedExportDebt = len(inv.UnusedExportDebt)
	for _, item := range inv.Exports {
		c.ExportCallerEdges += len(item.ProductionCallers)
	}
	c.Routes = len(inv.Routes)
	c.Tools = len(inv.Tools)
	c.ProductionImporters = len(inv.ProductionImporters)
	c.Wrappers = len(inv.Wrappers)
	c.CompatibilityMarkers = len(inv.CompatibilityMarkers)
	c.StoreCalls = len(inv.StoreCalls)
	c.InterfaceCandidates = len(inv.InterfaceCandidates)
	c.Citers = len(inv.Citers)
	inv.Counts = c
}

func writeInventory(path string, inv Inventory) error {
	data, err := yaml.Marshal(inv)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
