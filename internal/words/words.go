// Package words holds the embedded word banks: a dictionary for validating
// guesses and difficulty-tiered pools for secret words.
package words

import (
	"bufio"
	"embed"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

//go:embed data/words-*.txt
var dataFS embed.FS

// Difficulty is a secret-word tier. Higher is rarer.
type Difficulty int

const (
	Easy   Difficulty = 1
	Medium Difficulty = 2
	Hard   Difficulty = 3
)

func (d Difficulty) Valid() bool { return d >= Easy && d <= Hard }

func (d Difficulty) String() string {
	switch d {
	case Easy:
		return "Easy"
	case Medium:
		return "Medium"
	case Hard:
		return "Hard"
	}
	return fmt.Sprintf("Difficulty(%d)", int(d))
}

// Tier values below Easy, as written by cmd/wordgen.
const (
	guessOnly    Difficulty = 0  // a real word, but not a secret candidate
	commonProper Difficulty = -1 // "april": a valid guess, never a secret
	properName   Difficulty = -2 // "dakar": not accepted, known only for messages
)

// Bank answers word questions for every supported length.
type Bank struct {
	tiers map[int]map[string]Difficulty // length -> word -> tier
	pools map[int]map[Difficulty][]string
}

// Lengths are the supported word lengths.
var Lengths = []int{4, 5, 6}

// Load parses the embedded word files.
func Load() (*Bank, error) {
	b := &Bank{
		tiers: map[int]map[string]Difficulty{},
		pools: map[int]map[Difficulty][]string{},
	}
	for _, n := range Lengths {
		f, err := dataFS.Open(fmt.Sprintf("data/words-%d.txt", n))
		if err != nil {
			return nil, err
		}
		tiers := map[string]Difficulty{}
		pools := map[Difficulty][]string{}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			word, tierStr, ok := strings.Cut(sc.Text(), "\t")
			if !ok {
				continue
			}
			t, err := strconv.Atoi(tierStr)
			if err != nil {
				f.Close()
				return nil, fmt.Errorf("words-%d.txt: bad tier for %q", n, word)
			}
			tiers[word] = Difficulty(t)
			if Difficulty(t) >= Easy {
				pools[Difficulty(t)] = append(pools[Difficulty(t)], word)
			}
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return nil, err
		}
		b.tiers[n] = tiers
		b.pools[n] = pools
	}
	return b, nil
}

// IsWord reports whether w is an accepted guess.
func (b *Bank) IsWord(w string) bool {
	t, ok := b.tiers[len(w)][w]
	return ok && t >= commonProper
}

// IsProperNoun reports whether w is known only as a proper noun ("april",
// "dakar"). Words that are also ordinary words ("march") are not.
func (b *Bank) IsProperNoun(w string) bool {
	return b.tiers[len(w)][w] < guessOnly
}

// SecretTier is the easiest difficulty at which w may be a secret, or 0 if
// it can't be one (plurals, rare words, proper nouns).
func (b *Bank) SecretTier(w string) Difficulty {
	return max(b.tiers[len(w)][w], guessOnly)
}

// SecretAllowed reports whether w may be a secret in a game at difficulty d.
// A typed secret must be at d or easier, so players can't stump each other
// with obscure words.
func (b *Bank) SecretAllowed(w string, d Difficulty) bool {
	t := b.SecretTier(w)
	return t >= Easy && t <= d
}

// RandomSecret picks a secret from exactly tier d, so a "hard" game actually
// gets a hard word. When unique is set, words with repeated letters are skipped.
func (b *Bank) RandomSecret(length int, d Difficulty, unique bool) (string, error) {
	pool := b.pools[length][d]
	if len(pool) == 0 {
		return "", fmt.Errorf("no words for length %d difficulty %d", length, d)
	}
	// Rejection sampling: most words qualify, so this converges fast.
	for range 1000 {
		w := pool[rand.IntN(len(pool))]
		if !unique || HasUniqueLetters(w) {
			return w, nil
		}
	}
	return "", fmt.Errorf("no unique-letter words for length %d difficulty %d", length, d)
}

// HasUniqueLetters reports whether no letter appears twice in w.
func HasUniqueLetters(w string) bool {
	var seen [26]bool
	for i := range len(w) {
		c := w[i] - 'a'
		if seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}
