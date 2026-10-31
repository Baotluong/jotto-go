package game

import (
	"slices"
	"testing"
)

func TestCommonLetters(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"crane", "slate", 2}, // a, e
		{"apple", "paper", 4},
		{"hello", "world", 2}, // l, o
		{"abcde", "fghij", 0},
		{"stare", "tears", 5}, // anagram
		{"llama", "label", 3}, // l, l, a
	}
	for _, c := range cases {
		if got := CommonLetters(c.a, c.b); got != c.want {
			t.Errorf("CommonLetters(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := CommonLetters(c.b, c.a); got != c.want {
			t.Errorf("CommonLetters not symmetric for %q, %q", c.a, c.b)
		}
	}
}

func TestWordleMarks(t *testing.T) {
	H, P, M := Hit, Present, Miss
	cases := []struct {
		guess, secret string
		want          []Mark
	}{
		{"crane", "crane", []Mark{H, H, H, H, H}},
		{"stare", "tears", []Mark{P, P, H, H, P}},
		{"llama", "label", []Mark{H, P, P, M, M}},
		// Only one L in the secret: the exact match claims it, the other L misses.
		{"llama", "plant", []Mark{M, H, H, M, M}},
		// The secret's only E is claimed by the exact match, so earlier E's miss.
		{"geese", "those", []Mark{M, M, M, H, H}},
		{"eerie", "crane", []Mark{M, M, P, M, H}},
	}
	for _, c := range cases {
		if got := WordleMarks(c.guess, c.secret); !slices.Equal(got, c.want) {
			t.Errorf("WordleMarks(%q, %q) = %v, want %v", c.guess, c.secret, got, c.want)
		}
	}
}

func TestScoreModes(t *testing.T) {
	r := Score("stare", "tears", Classic)
	if r.Common != 5 || r.Exact != 0 || r.Marks != nil || r.Solved {
		t.Errorf("classic: %+v", r)
	}
	r = Score("crate", "crane", Positional)
	if r.Common != 4 || r.Exact != 4 || r.Marks != nil {
		t.Errorf("positional: %+v", r)
	}
	r = Score("crane", "crane", Wordle)
	if !r.Solved || r.Exact != 5 || len(r.Marks) != 5 {
		t.Errorf("wordle: %+v", r)
	}
}
