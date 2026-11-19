package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"sync"
	"sync/atomic"
)

// Fake is a deterministic embedder for tests: a vector is a function of the
// input's words, so equal text gives equal vectors and texts that share words
// are near. It needs no model.
type Fake struct {
	// Down makes every call fail (a server that is not there).
	Down atomic.Bool
	// Calls counts Embed calls and Inputs the texts in them.
	Calls  atomic.Int64
	Inputs atomic.Int64
	// Override, when set, returns the vector of an input before the hash does.
	Override func(input string) []float32
	mu       sync.Mutex
	seen     []string
}

// Seen returns every input embedded so far, in order.
func (f *Fake) Seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func (f *Fake) Health(context.Context) error {
	if f.Down.Load() {
		return errors.New("fake embedder down")
	}
	return nil
}

func (f *Fake) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	if f.Down.Load() {
		return nil, errors.New("fake embedder down")
	}
	f.Calls.Add(1)
	f.Inputs.Add(int64(len(inputs)))
	out := make([][]float32, len(inputs))
	f.mu.Lock()
	f.seen = append(f.seen, inputs...)
	f.mu.Unlock()
	for i, in := range inputs {
		if f.Override != nil {
			if v := f.Override(in); v != nil {
				out[i] = v
				continue
			}
		}
		out[i] = FakeVector(in)
	}
	return out, nil
}

// FakeVector is the vector Fake gives an input: the sum of one pseudo-random
// unit-ish vector per word after the prompt's "|" marks, normalised.
func FakeVector(input string) []float32 {
	text := input
	if i := strings.Index(input, "| "); i >= 0 {
		text = input[i+2:]
	}
	text = strings.TrimPrefix(text, "text: ")
	text = strings.TrimPrefix(text, "query: ")
	acc := make([]float64, Dim)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		seed := sha256.Sum256([]byte(w))
		for i := 0; i < Dim; i++ {
			block := sha256.Sum256(append(seed[:], byte(i), byte(i>>8)))
			acc[i] += float64(int32(binary.BigEndian.Uint32(block[:4]))) / math.MaxInt32
		}
	}
	var sum float64
	for _, x := range acc {
		sum += x * x
	}
	v := make([]float32, Dim)
	if sum == 0 {
		v[0] = 1
		return v
	}
	n := math.Sqrt(sum)
	for i, x := range acc {
		v[i] = float32(x / n)
	}
	return v
}
