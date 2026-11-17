package podcastworker

// Evaluate the matcher against judged candidate files (the operator's hand
// matches): a row with support above 0 is a positive, a row without is a
// negative. The files hold third-party transcript text, so they are read from
// where the operator keeps them and never from the repository.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
)

// EvalRow is the matcher's result at one threshold setting.
type EvalRow struct {
	Name                 string
	Positives, Negatives int
	TP, FP, FN, Unjudged int
	Rejected             map[podcastmatch.Rule]int
}

func (r EvalRow) Precision() float64 {
	if r.TP+r.FP == 0 {
		return 1
	}
	return float64(r.TP) / float64(r.TP+r.FP)
}

func (r EvalRow) Recall() float64 {
	if r.Positives == 0 {
		return 1
	}
	return float64(r.TP) / float64(r.Positives)
}

// EvalJudged runs the matcher on every passage of the files as its own tiny
// episode (its sentences timed in proportion to length across the clip's span)
// against every story of the files, and compares what it accepts with the
// judge's labels. The time rule is not exercised: the files carry no story
// update time. A pair the matcher accepts that no file offered as a candidate
// is counted as unjudged, not as a false positive.
func EvalJudged(ctx context.Context, paths []string, settings map[string]podcastmatch.Params, order []string) ([]EvalRow, error) {
	type pair struct{ story, ep string }
	label := map[pair]bool{}
	storyText := map[string][2]string{}
	epSegs := map[string]podcastmatch.Episode{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var f CandidatesFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, c := range f.Candidates {
			storyText[c.StoryRoute] = [2]string{c.StoryTitle, c.StoryDek}
			key := fmt.Sprintf("%s@%.1f", c.EpisodeID, c.Start)
			if _, ok := epSegs[key]; !ok {
				epSegs[key] = podcastmatch.Episode{Key: key, Segs: sentenceSegments(c.Passage, c.Start, c.End)}
			}
			p := pair{c.StoryRoute, key}
			label[p] = label[p] || c.Support > 0
		}
	}
	var stories []podcastmatch.Story
	for route, tt := range storyText {
		stories = append(stories, podcastmatch.Story{ID: route, Title: tt[0], Dek: tt[1]})
	}
	var eps []podcastmatch.Episode
	for _, e := range epSegs {
		if len(e.Segs) > 0 {
			eps = append(eps, e)
		}
	}
	pos, neg := 0, 0
	for _, l := range label {
		if l {
			pos++
		} else {
			neg++
		}
	}
	var rows []EvalRow
	for _, name := range order {
		res, err := podcastmatch.Match(ctx, stories, eps, settings[name])
		if err != nil {
			return nil, err
		}
		row := EvalRow{Name: name, Positives: pos, Negatives: neg, Rejected: res.Rejected}
		hit := map[pair]bool{}
		for _, a := range res.Accepted {
			p := pair{a.StoryID, a.EpisodeKey}
			l, offered := label[p]
			switch {
			case !offered:
				row.Unjudged++
			case l:
				row.TP++
				hit[p] = true
			default:
				row.FP++
			}
		}
		row.FN = pos - len(hit)
		rows = append(rows, row)
	}
	return rows, nil
}

// sentenceSegments cuts a passage into sentences timed across [start, end] in
// proportion to their length.
func sentenceSegments(passage string, start, end float64) []podcastmatch.Segment {
	var sents []string
	var cur strings.Builder
	words := strings.Fields(passage)
	for _, w := range words {
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
		if podcastmatch.SentenceEnd(w) {
			sents = append(sents, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		sents = append(sents, cur.String())
	}
	total := 0
	for _, s := range sents {
		total += len(s)
	}
	if total == 0 || end <= start {
		return nil
	}
	var segs []podcastmatch.Segment
	at := start
	for _, s := range sents {
		d := (end - start) * float64(len(s)) / float64(total)
		segs = append(segs, podcastmatch.Segment{Start: at, End: at + d - 0.01, Text: s})
		at += d
	}
	return segs
}

// FormatEval prints the precision and recall table.
func FormatEval(rows []EvalRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %9s %9s %4s %4s %4s %8s %9s %7s  rejected\n", "setting", "positives", "negatives", "TP", "FP", "FN", "unjudged", "precision", "recall")
	for _, r := range rows {
		var rej []string
		for _, rule := range podcastmatch.Rules {
			rej = append(rej, fmt.Sprintf("%s=%d", rule, r.Rejected[rule]))
		}
		fmt.Fprintf(&b, "%-8s %9d %9d %4d %4d %4d %8d %9.3f %7.3f  %s\n", r.Name, r.Positives, r.Negatives, r.TP, r.FP, r.FN, r.Unjudged, r.Precision(), r.Recall(), strings.Join(rej, " "))
	}
	return b.String()
}

var _ = time.Second
