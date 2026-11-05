package apppackage

import "testing"

// SurfaceParts partitions the behaviour surface: each covered input moves
// exactly its own part (and so the surface hash), and nothing else.
func TestSurfacePartsMoveWithTheirOwnInput(t *testing.T) {
	files := mapFiles{
		"prompts/desk.md": "be careful",
		"desk/main.go":    "package main\nimport \"fmt\"\nfunc main(){ fmt.Println(1) }\n",
		"desk/tool.go":    "package main\nfunc tool(){}\n",
	}
	build := func(t *testing.T, mut func(map[string]any), f mapFiles) (string, map[string]string) {
		t.Helper()
		m, err := ParseManifest(v2JSON(t, func(v map[string]any) {
			deskV(v)
			d := v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
			d["tools"] = []any{"desk/tool.go"}
			d["schedules"] = []any{map[string]any{"id": "tick", "every_seconds": 3600}}
			v["capabilities"] = []any{map[string]any{"grant": "desk.act"}}
			if mut != nil {
				mut(v)
			}
		}), OriginFirstParty)
		if err != nil {
			t.Fatal(err)
		}
		h, err := m.BehaviourSurface(f)
		if err != nil {
			t.Fatal(err)
		}
		parts, err := m.SurfaceParts(f)
		if err != nil {
			t.Fatal(err)
		}
		return h, parts
	}
	with := func(k, v string) mapFiles { f := cloneFiles(files); f[k] = v; return f }
	baseHash, base := build(t, nil, files)
	if h2, p2 := build(t, nil, files); h2 != baseHash || len(p2) != len(base) {
		t.Fatal("parts are not stable")
	}
	for _, p := range []string{PartPrompt, PartCode, PartSchedules, PartAccepts, PartKinds, PartGrants, PartApp, PartAssets} {
		if len(base[p]) != 64 {
			t.Fatalf("part %s missing: %v", p, base)
		}
	}
	desk := func(v map[string]any) map[string]any {
		return v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
	}
	cases := map[string]struct {
		mut   func(map[string]any)
		files mapFiles
		moved string
	}{
		"prompt":   {nil, with("prompts/desk.md", "be reckless"), PartPrompt},
		"code":     {nil, with("desk/main.go", "package main\nimport \"fmt\"\nfunc main(){ fmt.Println(2) }\n"), PartCode},
		"tool":     {nil, with("desk/tool.go", "package main\nfunc tool2(){}\n"), PartCode},
		"schedule": {func(v map[string]any) { desk(v)["schedules"].([]any)[0].(map[string]any)["every_seconds"] = 60 }, files, PartSchedules},
		"accepts":  {func(v map[string]any) { desk(v)["accepts"] = []any{"plan"} }, files, PartAccepts},
		"grants": {func(v map[string]any) {
			v["capabilities"] = []any{map[string]any{"grant": "desk.act"}, map[string]any{"grant": "desk.act.cancel"}}
		}, files, PartGrants},
		"asset": {nil, with("README.md", "hello"), PartAssets},
	}
	for name, tc := range cases {
		h, parts := build(t, tc.mut, tc.files)
		if h == baseHash {
			t.Errorf("%s: the surface hash did not move", name)
		}
		for k, v := range parts {
			moved := v != base[k]
			if moved != (k == tc.moved) {
				t.Errorf("%s: part %s moved=%v, want only %s", name, k, moved, tc.moved)
			}
		}
	}
}
