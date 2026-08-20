package solitaire
// trivial bump 2 for CI non-race sampling

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
)

type Suit string

const (
	SuitSpades   Suit = "spades"
	SuitHearts   Suit = "hearts"
	SuitDiamonds Suit = "diamonds"
	SuitClubs    Suit = "clubs"
)

type Color string

const (
	ColorBlack Color = "black"
	ColorRed   Color = "red"
)

func (s Suit) Color() Color {
	switch s {
	case SuitHearts, SuitDiamonds:
		return ColorRed
	default:
		return ColorBlack
	}
}

type Card struct {
	Suit   Suit `json:"suit"`
	Rank   int  `json:"rank"` // 1 (Ace) .. 13 (King)
	FaceUp bool `json:"face_up"`
}

func (c Card) String() string {
	r := fmt.Sprintf("%d", c.Rank)
	switch c.Rank {
	case 1:
		r = "A"
	case 11:
		r = "J"
	case 12:
		r = "Q"
	case 13:
		r = "K"
	}
	return fmt.Sprintf("%s:%s", r, c.Suit)
}

func NewStandardDeck() []Card {
	suits := []Suit{SuitSpades, SuitHearts, SuitDiamonds, SuitClubs}
	deck := make([]Card, 0, 52)
	for _, s := range suits {
		for r := 1; r <= 13; r++ {
			deck = append(deck, Card{Suit: s, Rank: r, FaceUp: false})
		}
	}
	return deck
}

func ShuffleDeck(deck []Card, seed uint64) ([]Card, uint64) {
	out := append([]Card(nil), deck...)
	if seed == 0 {
		var buf [8]byte
		_, _ = crand.Read(buf[:])
		seed = binary.LittleEndian.Uint64(buf[:])
		if seed == 0 {
			seed = 1
		}
	}
	r := rand.New(rand.NewPCG(seed, seed^0x5555555555555555))
	r.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out, seed
}
