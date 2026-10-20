package gateway

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestSourceAPILiveProviders calls each wave 1 endpoint once against the
// real providers with the keys in the environment. It runs only with
// CHOIR_SOURCE_API_LIVE=1 (it spends a little of each budget, one
// NewsAPI.ai token) and prints statuses, never keys or bodies.
func TestSourceAPILiveProviders(t *testing.T) {
	if os.Getenv("CHOIR_SOURCE_API_LIVE") != "1" {
		t.Skip("set CHOIR_SOURCE_API_LIVE=1 with the provider keys in the environment")
	}
	c := NewSourceAPIClientFromEnv()
	calls := []string{
		`{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`,
		`{"provider":"fred","endpoint":"series_observations","params":{"series_id":"UNRATE","limit":2,"sort_order":"desc"}}`,
		`{"provider":"fred","endpoint":"series_search","params":{"search_text":"Taiwan exports","limit":2}}`,
		`{"provider":"congress","endpoint":"bills","params":{"congress":119,"limit":2}}`,
		`{"provider":"congress","endpoint":"bill","params":{"congress":119,"type":"hr","number":1}}`,
		`{"provider":"congress","endpoint":"bill_actions","params":{"congress":119,"type":"hr","number":1,"limit":2}}`,
		`{"provider":"courtlistener","endpoint":"search","params":{"q":"export controls semiconductors","type":"r","order_by":"dateFiled desc"}}`,
		`{"provider":"youtube","endpoint":"videos","params":{"id":"jNQXAC9IVRw"}}`,
		`{"provider":"youtube","endpoint":"channels","params":{"for_handle":"@NASA"}}`,
		`{"provider":"newsapi_ai","endpoint":"usage","params":{}}`,
		`{"provider":"newsapi_ai","endpoint":"articles","params":{"keyword":"台灣","lang":"zho","count":2}}`,
	}
	docketID := ""
	for _, call := range calls {
		var req SourceAPIRequest
		if err := json.Unmarshal([]byte(call), &req); err != nil {
			t.Fatal(err)
		}
		out, err := c.Do(req)
		if err != nil {
			t.Errorf("%s.%s: %v", req.Provider, req.Endpoint, err)
			continue
		}
		shape := "body"
		if len(out.Body) == 0 {
			shape = "text"
		}
		var top map[string]json.RawMessage
		_ = json.Unmarshal(out.Body, &top)
		var keys []string
		for k := range top {
			keys = append(keys, k)
		}
		t.Logf("%s.%s: upstream %d, %s %d bytes, fields %s %s", req.Provider, req.Endpoint, out.UpstreamStatus, shape, len(out.Body)+len(out.Text), strings.Join(keys, ","), out.Error)
		if out.UpstreamStatus != 200 {
			t.Errorf("%s.%s: upstream %d", req.Provider, req.Endpoint, out.UpstreamStatus)
		}
		if req.Provider == "courtlistener" && req.Endpoint == "search" {
			var s struct {
				Results []struct {
					DocketID json.Number `json:"docket_id"`
				} `json:"results"`
			}
			_ = json.Unmarshal(out.Body, &s)
			if len(s.Results) > 0 {
				docketID = s.Results[0].DocketID.String()
			}
		}
	}
	if docketID == "" {
		t.Fatal("courtlistener search returned no docket id")
	}
	out, err := c.Do(SourceAPIRequest{Provider: "courtlistener", Endpoint: "docket", Params: map[string]json.RawMessage{"id": json.RawMessage(docketID)}})
	if err != nil || out.UpstreamStatus != 200 {
		t.Fatalf("courtlistener.docket %s: %v %+v", docketID, err, out)
	}
	t.Logf("courtlistener.docket: upstream 200, %d bytes", len(out.Body))
}
