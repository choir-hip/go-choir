// Command kernelgate holds the three structural gates of the Go kernel
// (modular architecture plan section 5.1, R1). The scripts
// check-kernel-imports, check-kernel-names and check-kernel-responsibilities
// call it.
//
//	kernelgate imports [DIR]           kernel files import only the standard library and the kernel
//	kernelgate names [DIR] [REPO]      kernel files name no desk and no app id
//	kernelgate responsibilities [DIR]  the exported surface equals DIR/RESPONSIBILITIES
//
// DIR defaults to internal/kernel; REPO to the working directory.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

const kernelPath = "github.com/yusefmosiah/go-choir/internal/kernel"

// legacyNames are desk and app literals that predate the registries.
var legacyNames = []string{
	"texture", "mail", "email", "calendar", "slides", "files", "image", "reader",
	"library", "browser", "podcast", "pdf", "epub", "video", "audio", "content",
	"super-console", "conductor", "management", "research", "engineering",
	"autopaper", "autoputer", "autoradio",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kernelgate imports|names|responsibilities [DIR] [REPO]")
		os.Exit(2)
	}
	dir := "internal/kernel"
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}
	var errs []string
	switch os.Args[1] {
	case "imports":
		errs = checkImports(dir)
	case "names":
		repo := "."
		if len(os.Args) > 3 {
			repo = os.Args[3]
		}
		errs = checkNames(dir, repo)
	case "responsibilities":
		errs = checkResponsibilities(dir)
	default:
		fmt.Fprintln(os.Stderr, "unknown gate", os.Args[1])
		os.Exit(2)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "kernel-gate FAIL:", e)
		}
		os.Exit(1)
	}
	fmt.Println("kernel-gate", os.Args[1]+": ok")
}

func goFiles(dir string, includeTests bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		out = append(out, p)
		return nil
	})
	sort.Strings(out)
	if err == nil && len(out) == 0 {
		err = fmt.Errorf("no Go files under %s", dir)
	}
	return out, err
}

func checkImports(dir string) []string {
	files, err := goFiles(dir, true)
	if err != nil {
		return []string{err.Error()}
	}
	var errs []string
	fset := token.NewFileSet()
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for _, imp := range af.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			first := strings.SplitN(p, "/", 2)[0]
			if strings.Contains(first, ".") && !strings.HasPrefix(p, kernelPath) {
				errs = append(errs, fmt.Sprintf("%s imports %q: the kernel imports only the standard library and itself", f, p))
			}
		}
	}
	return errs
}

func nameList(repo string) []string {
	set := map[string]bool{}
	for _, n := range deskmanifest.Names() {
		set[n] = true
	}
	for _, n := range legacyNames {
		set[n] = true
	}
	// App ids: the package manifests.
	nameRe := regexp.MustCompile(`"name"\s*:\s*"([^"]+)"`)
	manifests, _ := filepath.Glob(filepath.Join(repo, "frontend", "src", "packages", "*", "choir-package.json"))
	for _, m := range manifests {
		if b, err := os.ReadFile(m); err == nil {
			if sub := nameRe.FindSubmatch(b); sub != nil {
				set[string(sub[1])] = true
			}
		}
		set[filepath.Base(filepath.Dir(m))] = true
	}
	var out []string
	for n := range set {
		if n != "" {
			out = append(out, strings.ToLower(n))
		}
	}
	sort.Strings(out)
	return out
}

func checkNames(dir, repo string) []string {
	files, err := goFiles(dir, true)
	if err != nil {
		return []string{err.Error()}
	}
	names := nameList(repo)
	var errs []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			low := strings.ToLower(line)
			for _, n := range names {
				if hasWord(low, n) {
					errs = append(errs, fmt.Sprintf("%s:%d names %q: the kernel names no desk and no app", f, i+1, n))
				}
			}
		}
	}
	return errs
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// hasWord reports whether name occurs in line delimited by non-identifier bytes.
func hasWord(line, name string) bool {
	for from := 0; ; {
		i := strings.Index(line[from:], name)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(name)
		if (i == 0 || !isWordByte(line[i-1])) && (end == len(line) || !isWordByte(line[end])) {
			return true
		}
		from = i + 1
	}
}

// surface lists the exported top-level symbols of the non-test files: funcs,
// types, consts, vars, and methods on exported types as Type.Method.
func surface(dir string) ([]string, error) {
	files, err := goFiles(dir, false)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range af.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if d.Recv == nil {
					set[d.Name.Name] = true
					continue
				}
				if t := recvType(d.Recv.List[0].Type); ast.IsExported(t) {
					set[t+"."+d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							set[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								set[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	var out []string
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func recvType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvType(t.X)
	case *ast.IndexExpr:
		return recvType(t.X)
	case *ast.IndexListExpr:
		return recvType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func checkResponsibilities(dir string) []string {
	have, err := surface(dir)
	if err != nil {
		return []string{err.Error()}
	}
	b, err := os.ReadFile(filepath.Join(dir, "RESPONSIBILITIES"))
	if err != nil {
		return []string{err.Error()}
	}
	listed := map[string]string{}
	var errs []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		resp, syms, ok := strings.Cut(line, ":")
		if !ok {
			errs = append(errs, "RESPONSIBILITIES line without a responsibility: "+line)
			continue
		}
		for _, s := range strings.Fields(syms) {
			if prev, dup := listed[s]; dup {
				errs = append(errs, fmt.Sprintf("%s is listed under both %s and %s", s, prev, strings.TrimSpace(resp)))
			}
			listed[s] = strings.TrimSpace(resp)
		}
	}
	haveSet := map[string]bool{}
	for _, s := range have {
		haveSet[s] = true
		if _, ok := listed[s]; !ok {
			errs = append(errs, fmt.Sprintf("exported symbol %s is in no responsibility: add it to RESPONSIBILITIES deliberately", s))
		}
	}
	for s := range listed {
		if !haveSet[s] {
			errs = append(errs, fmt.Sprintf("RESPONSIBILITIES lists %s, which the kernel no longer exports", s))
		}
	}
	sort.Strings(errs)
	return errs
}
