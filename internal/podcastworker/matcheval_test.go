package podcastworker

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
)

// CHOIR_PODCAST_JUDGED names the operator's judged candidates files (comma
// separated). They hold third-party transcript text and are not in the
// repository, so this test skips without them; the committed fixtures in
// internal/podcastmatch carry the assertion that always runs.
func TestJudgedFilesPrecisionAtTheShippedThresholds(t *testing.T) {
	list := os.Getenv("CHOIR_PODCAST_JUDGED")
	if list == "" {
		t.Skip("CHOIR_PODCAST_JUDGED is not set")
	}
	rows, err := EvalJudged(context.Background(), strings.Split(list, ","),
		map[string]podcastmatch.Params{"strict": podcastmatch.StrictParams, "default": podcastmatch.DefaultParams, "loose": podcastmatch.LooseParams},
		[]string{"strict", "default", "loose"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + FormatEval(rows))
	for _, r := range rows[:2] {
		if r.Precision() < 0.9 {
			t.Errorf("%s precision %.3f below 0.9 on the judged files", r.Name, r.Precision())
		}
	}
}

func TestSentenceSegmentsCoverTheSpanOnSentenceEdges(t *testing.T) {
	segs := sentenceSegments("One short. Then a much longer second sentence here! Last?", 100, 130)
	if len(segs) != 3 || segs[0].Start != 100 || segs[2].End < 129.9 || segs[2].End > 130 {
		t.Fatalf("%+v", segs)
	}
	for _, s := range segs {
		if !podcastmatch.SentenceEnd(s.Text) {
			t.Errorf("%q", s.Text)
		}
	}
}
