package runtimeprompts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// docs/design/delegate-one-round-build-2026-10-21.md section 5: the
// overlay's frontend build job is run, not just read, against a fake npm.
// Failure modes: it links the image's dependencies when package.json or
// the lockfile differs (2); it leaves node_modules or dist behind (3); it
// runs npm ci when the files match (4); a failed build leaves the tree
// dirty or exits 0 (7).
func TestFrontendBuildJobUsesTheImageDependenciesOnlyWhenThePackageFilesMatch(t *testing.T) {
	if out, err := exec.Command("cp", "--version").CombinedOutput(); err != nil || !strings.Contains(string(out), "GNU") {
		t.Skip("the job's cp flags are GNU coreutils")
	}
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	var script string
	for _, m := range shellScriptCalls.FindAllStringSubmatch(overlay, -1) {
		if m[1] == "Start" && strings.Contains(m[2], "npm run build") {
			script = m[2]
		}
	}
	if script == "" {
		t.Fatal("no choir.Start recipe runs the npm build")
	}
	type result struct {
		exit     int
		npmCalls string
		tree     []string
		release  []string
	}
	run := func(t *testing.T, depsMatch, setDeps, buildFails bool) result {
		t.Helper()
		root := t.TempDir()
		front := filepath.Join(root, "platform", "frontend")
		release := filepath.Join(root, "release") + "/"
		deps := filepath.Join(root, "store", "deps")
		bin := filepath.Join(root, "bin")
		for _, d := range []string{front, filepath.Join(deps, "node_modules", "vite"), bin} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		write := func(p, s string) {
			if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write(filepath.Join(front, "package.json"), `{"name":"f"}`)
		write(filepath.Join(front, "package-lock.json"), `{"lockfileVersion":3}`)
		write(filepath.Join(deps, "package.json"), `{"name":"f"}`)
		lock := `{"lockfileVersion":3}`
		if !depsMatch {
			lock = `{"lockfileVersion":3,"packages":{"x":{}}}`
		}
		write(filepath.Join(deps, "package-lock.json"), lock)
		write(filepath.Join(deps, "node_modules", "vite", "marker"), "image")
		calls := filepath.Join(root, "npm-calls")
		fail := "0"
		if buildFails {
			fail = "1"
		}
		// The fake npm: ci makes a node_modules directory; run build needs a
		// node_modules with vite in it, then writes dist (or fails).
		write(filepath.Join(bin, "npm"), "#!/bin/sh\necho \"$*\" >> "+calls+"\n"+
			"case \"$1\" in\n"+
			"ci) mkdir -p node_modules/vite && echo registry > node_modules/vite/marker ;;\n"+
			"run) test -f node_modules/vite/marker || exit 3; cat node_modules/vite/marker >> "+calls+"; [ "+fail+" = 1 ] && exit 1; mkdir -p dist && echo app > dist/index.html ;;\n"+
			"esac\n")
		if err := os.Chmod(filepath.Join(bin, "npm"), 0o755); err != nil {
			t.Fatal(err)
		}
		local := strings.ReplaceAll(script, "/workspace/platform/frontend", front)
		local = strings.ReplaceAll(local, "/"+capsule.ReleaseTreePrefix+"frontend/", release)
		local = strings.ReplaceAll(local, "/"+strings.TrimSuffix(capsule.ReleaseTreePrefix, "/")+"/frontend", strings.TrimSuffix(release, "/"))
		cmd := exec.Command("sh", "-c", local)
		cmd.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "HOME=" + root}
		if setDeps {
			cmd.Env = append(cmd.Env, "CHOIR_FRONTEND_DEPS="+deps)
		}
		out, err := cmd.CombinedOutput()
		r := result{}
		if ee, ok := err.(*exec.ExitError); ok {
			r.exit = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run job: %v\n%s", err, out)
		}
		b, _ := os.ReadFile(calls)
		r.npmCalls = string(b)
		entries, _ := os.ReadDir(front)
		for _, e := range entries {
			r.tree = append(r.tree, e.Name())
		}
		rel, _ := os.ReadDir(release)
		for _, e := range rel {
			r.release = append(r.release, e.Name())
		}
		if _, err := os.Stat(filepath.Join(deps, "node_modules", "vite", "marker")); err != nil {
			t.Fatalf("the job removed the image's dependencies: %v", err)
		}
		return r
	}
	clean := "package-lock.json,package.json"

	matched := run(t, true, true, false)
	if matched.exit != 0 || strings.Contains(matched.npmCalls, "ci") || !strings.Contains(matched.npmCalls, "run build") ||
		!strings.Contains(matched.npmCalls, "image") || strings.Join(matched.tree, ",") != clean || strings.Join(matched.release, ",") != "index.html" {
		t.Fatalf("matching package files: %+v", matched)
	}
	for name, r := range map[string]result{"lockfile differs": run(t, false, true, false), "no image dependencies": run(t, true, false, false)} {
		if r.exit != 0 || !strings.Contains(r.npmCalls, "ci --no-audit --no-fund") || !strings.Contains(r.npmCalls, "registry") ||
			strings.Join(r.tree, ",") != clean {
			t.Errorf("%s: want npm ci and a clean tree: %+v", name, r)
		}
	}
	failed := run(t, true, true, true)
	if failed.exit == 0 || strings.Join(failed.tree, ",") != clean || len(failed.release) != 0 {
		t.Fatalf("failed build: want a non-zero exit, a clean tree, no release: %+v", failed)
	}
}
