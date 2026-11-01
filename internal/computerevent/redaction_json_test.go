package computerevent

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

// The paper's sequence 5647041 (2026-10-31): credential_assignment matched
// across a JSON key and its value, the splice ate the quotes and a brace, and
// the sealed projection batch no longer parsed. Every boot after replayed it
// and died. A JSON payload must stay JSON after redaction, and a secret in a
// string value must still be gone.
func TestRedactionKeepsJSONPayloadsJSON(t *testing.T) {
	cipher := externalTestCipher(t, 0x23)
	cases := []struct {
		name    string
		payload string
		secret  string
	}{
		{"credential pair", `{"ops":[{"body":{"password":"hunter2abcdef","n":1}}]}`, "hunter2abcdef"},
		{"numeric value after a token key", `{"ops":[{"body":{"auth_token":1234567890123,"x":"(y)"}}]}`, "1234567890123"},
		{"negative and float credentials", `{"password":-87654321,"api_key":1.2345678}`, "87654321"},
		{"escaped newline inside a credential", `{"k":"password=abc\ndefgh"}`, "defgh"},
		{"escaped comma inside a credential", `{"k":"token=ab\u002ccdefgh"}`, "cdefgh"},
		{"handle-shaped credential value", `{"password":"secret-handle:v1:x:0123456789abcdef"}`, "0123456789abcdef"},
		{"object value after a secret key", `{"secret":{"inner":"vvvvvvvvvvv"},"k":")"}`, ""},
		{"bearer in a string", `{"msg":"Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456"}`, "abcdefghijklmnopqrstuvwxyz123456"},
		{"escaped quote after a value", `{"note":"token=abcdefgh\\\"rest\" then"}`, ""},
		{"key in an escaped inner document", `{"text":"{\"api_key\":\"sk-abcdefghijklmnopqrstu\"}"}`, "sk-abcdefghijklmnopqrstu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !json.Valid([]byte(tc.payload)) {
				t.Fatalf("test payload is not JSON: %s", tc.payload)
			}
			envelope, _, err := cipher.Encrypt(context.Background(), "computer-test", "event-json", ProjectionBatchMediaType, "private", []byte(tc.payload))
			if err != nil {
				t.Fatal(err)
			}
			plain, _, err := cipher.Decrypt(context.Background(), envelope, "computer-test", "event-json")
			if err != nil {
				t.Fatal(err)
			}
			if !json.Valid(plain) {
				t.Fatalf("redacted payload is not JSON: %s", plain)
			}
			if tc.secret != "" && bytes.Contains(plain, []byte(tc.secret)) {
				t.Fatalf("secret survived redaction: %s", plain)
			}
		})
	}
}
