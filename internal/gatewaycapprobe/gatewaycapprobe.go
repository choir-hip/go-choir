// Package gatewaycapprobe holds the fixed request shapes of the guest's
// gateway body-cap diagnostic (GET|POST /internal/diag/gateway-body-cap in
// internal/autoputer). It is a leaf package so the guest route that sends the
// requests and the gateway test that proves they cannot reach a provider
// build their bytes from the same code.
//
// Record: docs/problems/guest-proof-steps-have-no-route-2026-10-23.md.
//
// Provider safety rests on two shapes, neither of which a gateway can decode
// into a request, under any cap mode or limit:
//
//   - the search body is an unterminated JSON string (SearchBody), so the
//     decoder fails at EOF when the body is under the cap and the cap fails it
//     first (413) when it is over;
//   - the model leg declares a Content-Length and sends no body byte at all
//     (ModelDeclaredHead), so there is nothing to decode.
package gatewaycapprobe

import (
	"fmt"
	"io"
	"strings"
)

const (
	// SearchPath is the capped, non-provider-backed-until-decoded route of
	// step 11(a); it enforces its 1 MiB cap in every mode.
	SearchPath = "/provider/v1/search"
	// ModelPath is the model route of step 11(b).
	ModelPath = "/provider/v1/inference"

	// SearchBodySize is the fixed size of the search body: the default
	// 1 MiB search cap plus 4 KiB. Never caller-sized.
	SearchBodySize int64 = 1<<20 + 4<<10
	// ModelDeclaredLength is the Content-Length declared on the model leg:
	// 700 MiB, over the 576 MiB observe ceiling (step 11(b)). No byte of it
	// is sent.
	ModelDeclaredLength int64 = 700 << 20

	searchPrefix = `{"query":"`
)

type repeatA struct{}

func (repeatA) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// SearchBody returns exactly SearchBodySize bytes: `{"query":"` and then
// 'a' bytes, never closed. It cannot decode as a SearchRequest.
func SearchBody() io.Reader {
	return io.MultiReader(strings.NewReader(searchPrefix), io.LimitReader(repeatA{}, SearchBodySize-int64(len(searchPrefix))))
}

// ModelDeclaredHead is the request head of the model leg: a POST whose
// Content-Length is ModelDeclaredLength and of which no body byte follows.
// host is the gateway's host:port. The token appears only in the
// Authorization line, which the caller sends and never records.
func ModelDeclaredHead(host, token string) []byte {
	return []byte(fmt.Sprintf("POST %s HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\n"+
		"Content-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
		ModelPath, host, token, ModelDeclaredLength))
}
