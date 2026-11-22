package offlineslice

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

func items(n int, textRunes int) []S1Item {
	var out []S1Item
	for i := 0; i < n; i++ {
		it := S1Item{ID: fmt.Sprintf("cap%d", i), Title: fmt.Sprintf("title %d", i), Text: strings.Repeat("word ", textRunes/5)}
		for k := 0; k < 3; k++ {
			it.Stories = append(it.Stories, S1Story{ID: fmt.Sprintf("st%d", (i+k)%7), Title: fmt.Sprintf("story %d", (i+k)%7), Summary: "summary"})
		}
		out = append(out, it)
	}
	return out
}

func TestS1JobsFitTheVendorLimitsAndCoverEveryItemOnce(t *testing.T) {
	jobs, err := BuildS1Jobs(items(400, 1800), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) < 2 {
		t.Fatalf("400 long items fit one call: %d jobs", len(jobs))
	}
	seen := map[string]int{}
	for _, j := range jobs {
		var st json.RawMessage = j.State
		if !jevbudget.Fits(st, j.Questions) {
			t.Fatalf("job %s crosses %s", j.ID, jevbudget.Limit(st, j.Questions))
		}
		for _, m := range j.Items {
			seen[m.ItemID]++
		}
		if len(j.Questions) != len(j.Items) {
			t.Fatalf("questions %d items %d", len(j.Questions), len(j.Items))
		}
	}
	if len(seen) != 400 {
		t.Fatalf("covered %d", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("%s in %d jobs", id, n)
		}
	}
}

func TestS1MaxPerCallBinds(t *testing.T) {
	jobs, err := BuildS1Jobs(items(30, 100), 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || len(jobs[0].Items) != 12 || len(jobs[2].Items) != 6 {
		t.Fatalf("jobs %d", len(jobs))
	}
}

func TestS1StoryPoolIsSharedWithinACall(t *testing.T) {
	jobs, _ := BuildS1Jobs(items(7, 100), 0)
	if len(jobs) != 1 {
		t.Fatal("one call")
	}
	var st struct {
		Stories map[string]json.RawMessage `json:"stories"`
	}
	if err := json.Unmarshal(jobs[0].State, &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Stories) != 7 {
		t.Fatalf("pool %d, want the 7 distinct stories", len(st.Stories))
	}
}

func TestS1AnswersMapBackToStoryIDs(t *testing.T) {
	jobs, _ := BuildS1Jobs(items(2, 100), 0)
	j := jobs[0]
	m := j.Items[0]
	// the model picks the second option of the first item
	var pick string
	for opt, sid := range m.Options {
		if sid == "st1" {
			pick = opt
		}
	}
	if pick == "" {
		t.Fatalf("options %v", m.Options)
	}
	body := fmt.Sprintf(`{"answers":{%q:{"type":"choice","choice":%q,"confidence":0.8,"probabilities":{%q:0.9,"other":0.1}}},"usage":{"input_tokens":10,"output_tokens":2,"cost":0.001}}`, m.Question, pick, pick)
	res, err := ParseS1(j, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.Answers[m.ItemID].Story != "st1" || res.Answers[m.ItemID].Verdict != "story" || res.Answers[m.ItemID].Confidence != 0.8 {
		t.Fatalf("%+v", res.Answers[m.ItemID])
	}
	// the other item has no answer: recorded as missing, not as "other"
	if a, ok := res.Answers[j.Items[1].ItemID]; ok && a.Verdict != "missing" {
		t.Fatalf("missing answer read as %+v", a)
	}
	if res.Cost != 0.001 || res.InputTokens != 10 {
		t.Fatalf("usage %+v", res)
	}
	// other and idk
	body = fmt.Sprintf(`{"answers":{%q:{"choice":"other","confidence":0.5},%q:{"choice":"idk","confidence":0.1}}}`, m.Question, j.Items[1].Question)
	res, _ = ParseS1(j, []byte(body))
	if res.Answers[m.ItemID].Verdict != "other" || res.Answers[j.Items[1].ItemID].Verdict != "idk" {
		t.Fatalf("%+v", res.Answers)
	}
}

func TestS1TheOracleIsNotInTheRequest(t *testing.T) {
	jobs, _ := BuildS1Jobs(items(3, 100), 0)
	b, _ := json.Marshal(jobs[0])
	for _, bad := range []string{"fed", "cited", "oracle"} {
		if strings.Contains(strings.ToLower(string(b)), bad) {
			t.Fatalf("request mentions %q", bad)
		}
	}
}
