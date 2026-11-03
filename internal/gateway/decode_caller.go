package gateway

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// withModelDecodeCaller names the computer as the call's image decode caller,
// so one computer holds at most its share of the decode slots (bunch 116, F2).
func withModelDecodeCaller(r *http.Request, computerID string) *http.Request {
	return r.WithContext(provider.WithImageDecodeCaller(r.Context(), computerID))
}
