package offlineslice

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// S1Story is a candidate story offered for an arrival.
type S1Story struct {
	ID      string
	Title   string
	Summary string
}

// S1Item is one arrival and the stories nearest to it as of its arrival.
type S1Item struct {
	ID      string
	Title   string
	Text    string
	Stories []S1Story
}

// S1Map says, for one item of a job, which question it is, which handle the
// capture has in the state, and which story each option stands for.
type S1Map struct {
	ItemID   string            `json:"item_id"`
	Handle   string            `json:"handle"`
	Question string            `json:"question"`
	Options  map[string]string `json:"options"` // option key -> story id
}

// S1Job is one System 1 request: a shared state and a question for each item.
type S1Job struct {
	ID        string                     `json:"id"`
	State     json.RawMessage            `json:"state"`
	Questions map[string]json.RawMessage `json:"questions"`
	Items     []S1Map                    `json:"items"`
}

const s1Task = "Each capture is a news item. Each story is a story already published. For each capture, decide whether one of the stories reports the same affair."

type s1Build struct {
	captures  map[string]any
	stories   map[string]any
	storyKey  map[string]string // story id -> handle in this job
	questions map[string]json.RawMessage
	items     []S1Map
}

func newS1Build() *s1Build {
	return &s1Build{captures: map[string]any{}, stories: map[string]any{}, storyKey: map[string]string{}, questions: map[string]json.RawMessage{}}
}

func (b *s1Build) state() json.RawMessage {
	st, _ := json.Marshal(map[string]any{"task": s1Task, "captures": b.captures, "stories": b.stories})
	return st
}

// add puts an item into the build and returns an undo.
func (b *s1Build) add(it S1Item) (undo func()) {
	n := len(b.items) + 1
	handle := fmt.Sprintf("c%d", n)
	b.captures[handle] = map[string]string{"title": it.Title, "text": it.Text}
	var newKeys []string
	opts := map[string]string{}
	crit := map[string]string{}
	for _, s := range it.Stories {
		key, ok := b.storyKey[s.ID]
		if !ok {
			key = fmt.Sprintf("s%d", len(b.storyKey)+1)
			b.storyKey[s.ID] = key
			b.stories[key] = map[string]string{"title": s.Title, "summary": s.Summary}
			newKeys = append(newKeys, s.ID)
		}
		opts[key] = s.ID
		crit[key] = fmt.Sprintf("`state.stories.%s` reports the same affair as `state.captures.%s`: the same happening, or a direct development of it. Sharing actors or a topic is not enough.", key, handle)
	}
	crit["other"] = "The capture reports an affair that none of the listed stories covers."
	crit["idk"] = "The capture has no readable news content, or it cannot be told what it reports."
	qname := "q_" + handle
	q, _ := json.Marshal(map[string]any{
		"type":         "choice",
		"instructions": fmt.Sprintf("Which of the stories offered reports the same affair as `state.captures.%s`?", handle),
		"criteria":     crit,
	})
	b.questions[qname] = q
	b.items = append(b.items, S1Map{ItemID: it.ID, Handle: handle, Question: qname, Options: opts})
	return func() {
		delete(b.captures, handle)
		delete(b.questions, qname)
		b.items = b.items[:len(b.items)-1]
		for _, id := range newKeys {
			key := b.storyKey[id]
			delete(b.stories, key)
			delete(b.storyKey, id)
		}
	}
}

// BuildS1Jobs packs items, in order, into as few requests as the vendor's
// limits (internal/jevbudget) allow; maxPerCall > 0 also caps the items in a
// request (used only for the small-batch comparison). Every item is in exactly
// one job. An item that alone crosses a limit is an error.
func BuildS1Jobs(items []S1Item, maxPerCall int) ([]S1Job, error) {
	var jobs []S1Job
	b := newS1Build()
	flush := func() {
		if len(b.items) == 0 {
			return
		}
		jobs = append(jobs, S1Job{ID: fmt.Sprintf("j%04d", len(jobs)+1), State: b.state(), Questions: b.questions, Items: b.items})
		b = newS1Build()
	}
	for _, it := range items {
		if maxPerCall > 0 && len(b.items) >= maxPerCall {
			flush()
		}
		undo := b.add(it)
		if !jevbudget.Fits(b.state(), b.questions) {
			undo()
			if len(b.items) == 0 {
				return nil, fmt.Errorf("item %s alone crosses %s", it.ID, jevbudget.Limit(b.state(), b.questions))
			}
			flush()
			b.add(it)
			if !jevbudget.Fits(b.state(), b.questions) {
				return nil, fmt.Errorf("item %s alone crosses %s", it.ID, jevbudget.Limit(b.state(), b.questions))
			}
		}
	}
	flush()
	return jobs, nil
}

// S1Answer is one item's answer with the option mapped back to a story.
type S1Answer struct {
	Verdict    string             `json:"verdict"` // story | other | idk | missing
	Story      string             `json:"story,omitempty"`
	Confidence float64            `json:"confidence"`
	StoryProb  map[string]float64 `json:"story_prob,omitempty"` // story id -> probability
	OtherProb  float64            `json:"other_prob"`
	IdkProb    float64            `json:"idk_prob"`
}

// S1Result is a parsed response.
type S1Result struct {
	Answers      map[string]S1Answer
	InputTokens  int
	OutputTokens int
	Cost         float64
}

// ParseS1 reads a response body against its job.
func ParseS1(j S1Job, body []byte) (S1Result, error) {
	var r struct {
		Answers map[string]struct {
			Choice        string             `json:"choice"`
			Confidence    float64            `json:"confidence"`
			Probabilities map[string]float64 `json:"probabilities"`
		} `json:"answers"`
		Usage struct {
			In   int     `json:"input_tokens"`
			Out  int     `json:"output_tokens"`
			Cost float64 `json:"cost"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return S1Result{}, err
	}
	res := S1Result{Answers: map[string]S1Answer{}, InputTokens: r.Usage.In, OutputTokens: r.Usage.Out, Cost: r.Usage.Cost}
	for _, m := range j.Items {
		a, ok := r.Answers[m.Question]
		if !ok || a.Choice == "" {
			res.Answers[m.ItemID] = S1Answer{Verdict: "missing"}
			continue
		}
		ans := S1Answer{Confidence: a.Confidence, OtherProb: a.Probabilities["other"], IdkProb: a.Probabilities["idk"], StoryProb: map[string]float64{}}
		for opt, sid := range m.Options {
			if p, ok := a.Probabilities[opt]; ok {
				ans.StoryProb[sid] = p
			}
		}
		switch c := strings.TrimSpace(a.Choice); {
		case c == "other":
			ans.Verdict = "other"
		case c == "idk":
			ans.Verdict = "idk"
		default:
			sid, ok := m.Options[c]
			if !ok {
				ans.Verdict = "missing"
			} else {
				ans.Verdict, ans.Story = "story", sid
			}
		}
		res.Answers[m.ItemID] = ans
	}
	return res, nil
}
