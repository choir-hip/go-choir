package computerevent

import (
	"slices"
	"testing"
)

// DetectRefusalSecrets is the release-staging refusal predicate. It must
// refuse real credentials while admitting machine-generated text (vendored
// JS, minified bundles, compiled dumps) whose credential-shaped tokens are
// identifiers and literals — the same false-positive class as compiled
// binaries, documented in the TestStageGrantedReleaseAdmitsBinaryStringBlob
// regression note. Receipt: docs/problems/m11-freeze-secret-screen-and-ref-
// prefix-2026-09-28.md (pdf.worker-B1D2UnXD.mjs refused a real freeze).
func TestDetectRefusalSecretsAdmitsVendoredJavascript(t *testing.T) {
	cases := map[string]string{
		"pdf.js bitmask enum":        "  MULTILINE: 0x0001000,\n  PASSWORD: 0x0002000,\n  NOTOGGLETOOFF: 0x0004000,",
		"member access read":         "const tok = this.getToken()\ntoken = this.getToken()\n",
		"member call argument":       "password = this.#decodeUserPassword(passwordBytes)\n",
		"member field":               "password: this.data.password\n",
		"self assignment":            "password = password\npassword = utf8StringToString(password)\n",
		"field flag access":          "password = this.hasFieldFlag(AnnotationFieldFlag.PASSWORD)\n",
		"literal name self-assign":   "Password = ownerPassword\nPassword = cipher.encryptBlock(userPassword)\n",
		"identifier sub-slice":       "password = password.subarray(0\n",
		"camelCase identifier value": "password=secretValue\ntoken=accessToken\n",
		"hex-prefixed literal":       "secret=0xdeadbeef00\napi_key=0xABCDEF12\n",
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := DetectRefusalSecrets([]byte(payload)); len(findings) != 0 {
				t.Fatalf("vendored code refused as secret: %v", findings)
			}
		})
	}
}

func TestDetectRefusalSecretsRefusesRealCredentials(t *testing.T) {
	cases := map[string]struct {
		payload string
		kind    string
	}{
		"plain api key":      {payload: "api_key=abcdefghijklmnop", kind: "credential_assignment"},
		"quoted api key":     {payload: `api_key = "abcdefghijklmnop"`, kind: "credential_assignment"},
		"lowercase password": {payload: "password=hunter2abc", kind: "credential_assignment"},
		"private key header": {payload: "-----BEGIN RSA PRIVATE KEY-----", kind: "private_key_header"},
		"openai-style key":   {payload: "sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ1234", kind: "openai_key"},
		"github token":       {payload: "ghp_abcdefghijklmnop1234", kind: "github_token"},
		"google api key":     {payload: "AIzaSyABCDEFGHIJKLMNOPQRSTUVWXYZ1234", kind: "google_api_key"},
		"bearer literal":     {payload: "Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456", kind: "authorization_bearer"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			findings := DetectRefusalSecrets([]byte(tc.payload))
			if !slices.Contains(findings, tc.kind) {
				t.Fatalf("expected %s in %v", tc.kind, findings)
			}
		})
	}
}

// The redaction path keeps the permissive detector unchanged: over-matching
// costs encryption, not refusal. Refusal and redaction MUST stay split — a
// shared predicate is the exact defect the pdf.worker refusal receipt names.
func TestDetectPrivateSecretsStillRedactionGrade(t *testing.T) {
	payload := []byte("PASSWORD: 0x0002000")
	if findings := DetectPrivateSecrets(payload); !slices.Contains(findings, "credential_assignment") {
		t.Fatalf("redaction detector lost credential_assignment: %v", findings)
	}
}
