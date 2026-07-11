package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
	inventorySchema = "runtime-dissolution-inventory/v1"
	runtimeImport  = "github.com/yusefmosiah/go-choir/internal/runtime"
)

type Inventory struct {
	Schema               string `yaml:"schema"`
	CanonicalParent      string `yaml:"canonical_parent"`
	DispatchNonce        string `yaml:"dispatch_nonce"`
	Transition           string `yaml:"transition"`
	Counts               Counts `yaml:"counts"`
	Files                []Entry `yaml:"files"`
	Exports              []Entry `yaml:"exports"`
	Routes               []Entry `yaml:"routes"`
	Tools                 []Entry `yaml:"tools"`
	ProductionImporters  []Entry `yaml:"production_importers"`
	Wrappers              []Entry `yaml:"wrappers"`
	CompatibilityMarkers []Entry `yaml:"compatibility_markers"`
	StateWriters          []Entry `yaml:"state_writers"`
	Citers                []Entry `yaml:"citers"`
}

type Counts struct {
	GoFiles              int `yaml:"go_files"`
	ProductionFiles      int `yaml:"production_files"`
	TestFiles            int `yaml:"test_files"`
	ProductionLOC        int `yaml:"production_loc"`
	TestLOC              int `yaml:"test_loc"`
	Exports              int `yaml:"exports"`
	Routes               int `yaml:"routes"`
	Tools                int `yaml:"tools"`
	ProductionImporters  int `yaml:"production_importers"`
	Wrappers              int `yaml:"wrappers"`
	CompatibilityMarkers int `yaml:"compatibility_markers"`
	StateWriters          int `yaml:"state_writers"`
	Citers                int `yaml:"citers"`
}

type Entry struct {
	ID          string `yaml:"id"`
	Disposition string `yaml:"disposition"`
	LOC         int    `yaml:"loc,omitempty"`
}

var compatibilityRE = regexp.MustCompile(`(?i)\b(deprecated|compatib(?:ility|le)|legacy|old runtime|new runtime)\b`)
var writerVerbRE = regexp.MustCompile(`^(Create|Update|Set|Append|Record|Complete|Publish|Promote|Save|Put|Delete|Mark|Transition)`)
var writerObjectRE = regexp.MustCompile(`(?i)(Run|Wire|Promotion|ComputerVersion|AppChangePackage|CandidatePackage)`)

func scanRepository(root string) (Inventory, error) {
	inv := Inventory{
		Schema: inventorySchema,
		CanonicalParent: "f72a141ef0f97fbec6521831dc3f5836b9526631",
		DispatchNonce: "s0-runtime-inventory-ratchet-01-nonce-01",
		Transition: "s0-runtime-inventory-ratchet-dispatch-intent-01",
	}
	files, err := repositoryFiles(root)
	if err != nil {
		return Inventory{}, err
	}
	citerOrdinals := map[string]int{}
	if err := scanGo(root, files, citerOrdinals, &inv); err != nil {
		return Inventory{}, err
	}
	if err := scanTextCiters(root, files, citerOrdinals, &inv); err != nil {
		return Inventory{}, err
	}
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

func scanGo(root string, files []string, citerOrdinals map[string]int, inv *Inventory) error {
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
		inRuntime := rel == "internal/runtime" || strings.HasPrefix(rel, "internal/runtime/")
		if inRuntime {
			loc := countLines(src)
			kind := "production"
			if isTest {
				kind = "test"
			}
			inv.Files = append(inv.Files, Entry{ID: rel + " [" + kind + "]", Disposition: domainDisposition(rel), LOC: loc})
			scanRuntimeAST(rel, file, fset, isTest, inv)
		}
		aliases := runtimeAliases(file)
		if !inRuntime && !isTest && len(aliases) > 0 {
			inv.ProductionImporters = append(inv.ProductionImporters, Entry{ID: rel, Disposition: "delete"})
			scanWrappers(rel, file, aliases, inv)
		}
		scanGoCommentCiters(rel, file, citerOrdinals, inv)
	}
	return nil
}

func scanRuntimeAST(rel string, file *ast.File, fset *token.FileSet, isTest bool, inv *Inventory) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(d.Name.Name) {
				kind := "func"
				if d.Recv != nil && len(d.Recv.List) > 0 {
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
		return
	}
	ordinals := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok && exprString(lit.Type) == "Tool" {
			if name, ok := toolName(lit); ok {
				id := rel + ":Tool:" + name
				inv.Tools = append(inv.Tools, Entry{ID: uniqueID(id, ordinals), Disposition: domainDisposition(rel)})
			}
		}
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
		// Direct Tool literals are inventoried above. Register calls that receive
		// constructor results are represented by the returned Tool declaration.
		callee := sel.Sel.Name
		if writerVerbRE.MatchString(callee) && writerObjectRE.MatchString(callee) {
			id := rel + ":" + enclosingFunction(file, call.Pos()) + ":" + callee
			inv.StateWriters = append(inv.StateWriters, Entry{ID: uniqueID(id, ordinals), Disposition: domainDisposition(rel)})
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
}

func scanWrappers(rel string, file *ast.File, aliases map[string]bool, inv *Inventory) {
	ordinals := map[string]int{}
	ast.Inspect(file, func(n ast.Node) bool {
		var typ ast.Expr
		var label string
		switch x := n.(type) {
		case *ast.Field:
			typ = x.Type
			label = "field"
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

func runtimeAliases(file *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || (path != runtimeImport && !strings.HasPrefix(path, runtimeImport+"/")) {
			continue
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" && name != "." {
			aliases[name] = true
		}
	}
	return aliases
}

func runtimeSurfaceType(expr ast.Expr, aliases map[string]bool) string {
	switch x := expr.(type) {
	case *ast.StarExpr:
		return runtimeSurfaceType(x.X, aliases)
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		if ok && aliases[id.Name] && (x.Sel.Name == "Runtime" || x.Sel.Name == "APIHandler") {
			return id.Name + "." + x.Sel.Name
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
		if ok && key.Name == "Name" {
			return stringLiteral(kv.Value)
		}
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
		{"candidate_package", []string{"candidate_package"}},
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
	if strings.HasPrefix(rel, "docs/evidence/") {
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
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " "))
	if len(s) > 240 {
		s = s[:240]
	}
	return s
}

func slashRel(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

func sortInventory(inv *Inventory) {
	lists := []*[]Entry{&inv.Files, &inv.Exports, &inv.Routes, &inv.Tools, &inv.ProductionImporters, &inv.Wrappers, &inv.CompatibilityMarkers, &inv.StateWriters, &inv.Citers}
	for _, list := range lists {
		sort.Slice(*list, func(i, j int) bool { return (*list)[i].ID < (*list)[j].ID })
	}
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
	c.Routes = len(inv.Routes)
	c.Tools = len(inv.Tools)
	c.ProductionImporters = len(inv.ProductionImporters)
	c.Wrappers = len(inv.Wrappers)
	c.CompatibilityMarkers = len(inv.CompatibilityMarkers)
	c.StateWriters = len(inv.StateWriters)
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
