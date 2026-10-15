package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `choir self-dev patch` reads a frozen candidate's source patch through the
// read-only source-patch route. Ways it could fail, written before the
// command:
//   - the bytes are corrupted or substituted in transit (proxy truncation, a
//     wrong bundle) and the caller applies them anyway: the client must hash
//     what it received and refuse a mismatch with the route's
//     source_patch_sha256, on both surfaces;
//   - the route answers a different operation than the one asked for;
//   - --out writes a file while the command still reports failure, or
//     reports success without writing; a failed verification leaves a file
//     behind that a later `git apply` would use;
//   - MCP inlines a multi-megabyte patch into model context, or a non-UTF-8
//     patch whose text encoding would change the bytes (and the sha);
//   - --out leaks into the MCP schema (a model must not pick local paths);
//   - the CLI and MCP build different requests or print different output
//     (registry_test.go self_dev_patch case).

const testSourcePatch = "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"

// sourcePatchReply is the route's answer for patch; a non-empty claimedSHA
// replaces the true sha256 (a mismatch).
func sourcePatchReply(operationID, computerID string, patch []byte, claimedSHA string) string {
	sum := sha256.Sum256(patch)
	if claimedSHA == "" {
		claimedSHA = hex.EncodeToString(sum[:])
	}
	raw, _ := json.Marshal(map[string]any{
		"operation_id": operationID, "computer_id": computerID, "state": "awaiting_approval",
		"bundle_digest": strings.Repeat("b", 64), "source_patch_sha256": claimedSHA,
		"source_patch_base_commit": strings.Repeat("1", 40), "patch_bytes": len(patch),
		"patch_base64": base64.StdEncoding.EncodeToString(patch),
	})
	return string(raw)
}

func TestSelfDevPatchVerifiesTheSHA256OnBothSurfaces(t *testing.T) {
	cliArgs := []string{"self-dev", "patch", "--computer", "c1", "--operation", "op-1"}
	mcpArgs := `{"computer":"c1","operation":"op-1"}`

	// A mismatch is refused on the CLI (exit 1, no file) and on MCP (isError).
	parityServer(t, sourcePatchReply("op-1", "c1", []byte(testSourcePatch), strings.Repeat("0", 64)))
	out := filepath.Join(t.TempDir(), "op-1.patch")
	var stdout, stderr bytes.Buffer
	if code := run(append(cliArgs, "--out", out), &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "sha256") {
		t.Fatalf("mismatch: cli exit %d stderr %q, want 1 naming the sha256", code, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("a refused patch left %s behind (%v)", out, err)
	}
	if result := toolCall(t, "self_dev_patch", mcpArgs); !result.IsError || !strings.Contains(result.Content[0].Text, "sha256") {
		t.Fatalf("mismatch: mcp %+v, want isError naming the sha256", result)
	}

	// The route answering another operation is refused.
	parityServer(t, sourcePatchReply("op-2", "c1", []byte(testSourcePatch), ""))
	if code := run(cliArgs, &stdout, &stderr); code != 1 {
		t.Fatalf("wrong operation: cli exit %d", code)
	}

	// A verified patch: --out writes exactly the bytes and the JSON names the
	// file instead of inlining the patch.
	parityServer(t, sourcePatchReply("op-1", "c1", []byte(testSourcePatch), ""))
	stdout.Reset()
	stderr.Reset()
	if code := run(append(cliArgs, "--out", out), &stdout, &stderr); code != 0 {
		t.Fatalf("verified: cli exit %d stderr %q", code, stderr.String())
	}
	written, err := os.ReadFile(out)
	if err != nil || string(written) != testSourcePatch {
		t.Fatalf("--out wrote %q (%v)", written, err)
	}
	var summary map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil || summary["out"] != out || summary["patch"] != nil ||
		summary["sha256_verified"] != true || summary["source_patch_base_commit"] != strings.Repeat("1", 40) {
		t.Fatalf("--out summary %s (%v)", stdout.String(), err)
	}

	// Inline (MCP, or the CLI without --out) refuses a patch over the cap.
	big := []byte(strings.Repeat("+x\n", selfDevPatchInlineMax/3+1))
	parityServer(t, sourcePatchReply("op-1", "c1", big, ""))
	if code := run(cliArgs, &stdout, &stderr); code != 1 {
		t.Fatalf("oversize inline: cli exit %d", code)
	}
	if result := toolCall(t, "self_dev_patch", mcpArgs); !result.IsError || !strings.Contains(result.Content[0].Text, "--out") {
		t.Fatalf("oversize inline: mcp %+v, want isError pointing at --out", result)
	}

	// --out is a CLI-only parameter.
	for _, cmd := range mcpCommands() {
		if cmd.toolName() == "self_dev_patch" {
			if _, ok := cmd.inputSchema()["properties"].(map[string]any)["out"]; ok {
				t.Fatal("--out leaked into the MCP schema")
			}
		}
	}
}
