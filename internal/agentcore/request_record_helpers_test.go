package agentcore

import "github.com/yusefmosiah/go-choir/internal/types"

// testRequest is a valid request-field set for tests that stage a request
// and are not about the request fields themselves.
func testRequest() *types.RequestFields {
	p, w := 0.8, 0.3
	return &types.RequestFields{PSuccess: &p, Weight: &w, Settlement: types.SettlementRecipient}
}
