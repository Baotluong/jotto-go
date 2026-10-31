// Command wordgen builds the embedded word banks from a SCOWL "final" directory.
//
// SCOWL splits English into size levels (10 = most common ... 95 = obscure).
// We map those levels onto three secret-word difficulty tiers and use a wider
// slice of SCOWL as the "is this a real word" dictionary for guesses.
//
// Output: one file per word length, internal/words/data/words-<n>.txt, each
// line "word<TAB>tier" where tier 1-3 means the word may be a secret at that
// difficulty, 0 means it is accepted as a guess only, -1 is a common proper
// noun accepted as a guess ("april"), and -2 is a proper noun that isn't
// accepted at all ("dakar") but is listed so players can be told why.
//
// Usage: go run ./cmd/wordgen -scowl <dir>/final -blocklist <file> -out internal/words/data
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// tierLevels maps each secret difficulty tier to the SCOWL levels it adds.
// Tiers are cumulative in play: a "hard" game accepts easy and medium secrets.
var tierLevels = map[int][]int{
	1: {10, 20},
	2: {35},
	3: {40, 50},
}

// guessLevels are the levels accepted as valid guesses, beyond the tiers.
var guessLevels = []int{55, 60, 70, 80}

// capitalLevels are the levels of SCOWL's always-capitalized words accepted
// as guesses (never secrets). Up to 40 these are common words like months,
// days, and nationalities; from 50 on they're mostly people and places.
var capitalLevels = []int{10, 35, 40}

// allLevels are every SCOWL level, for collecting the full set of proper nouns.
var allLevels = []int{10, 20, 35, 40, 50, 55, 60, 70, 80, 95}

// Tier values for proper nouns. Keep in sync with internal/words.
const (
	tierCommonProper = -1 // "april": a valid guess, never a secret
	tierProperName   = -2 // "dakar": rejected, but recognized for a clear message
)

var lengths = []int{4, 5, 6}

func main() {
	scowlDir := flag.String("scowl", "", "path to SCOWL final/ directory")
	blockFile := flag.String("blocklist", "", "optional file of words to exclude, one per line")
	outDir := flag.String("out", "internal/words/data", "output directory")
	flag.Parse()
	if *scowlDir == "" {
		log.Fatal("-scowl is required")
	}

	blocked := map[string]bool{}
	if *blockFile != "" {
		for _, w := range mustReadLines(*blockFile) {
			blocked[strings.ToLower(strings.TrimSpace(w))] = true
		}
	}

	// tier[word] = lowest tier the word appears in; 0 = guess-only.
	tier := map[string]int{}
	for t := 1; t <= 3; t++ {
		for _, lvl := range tierLevels[t] {
			for _, w := range readLevel(*scowlDir, lvl, false) {
				if _, ok := tier[w]; !ok {
					tier[w] = t
				}
			}
		}
	}
	for _, lvl := range append(allTierLevels(), guessLevels...) {
		// British spellings are fine to guess but not to pick as secrets.
		for _, w := range readLevel(*scowlDir, lvl, true) {
			if _, ok := tier[w]; !ok {
				tier[w] = 0
			}
		}
	}

	// Plurals and past tenses ("cats", "baked") make poor secrets: demote them.
	// This runs before proper nouns are added so "Marys" can't demote "marys".
	for w, t := range tier {
		if t > 0 && isInflection(w, tier) {
			tier[w] = 0
		}
	}

	// Proper nouns are recorded only so players get a clear message. A word
	// that is also an ordinary lowercase word ("march") keeps its tier.
	for _, lvl := range capitalLevels {
		for _, w := range readCapitalized(*scowlDir, "english-upper", lvl) {
			if _, ok := tier[w]; !ok {
				tier[w] = tierCommonProper
			}
		}
	}
	for _, list := range []string{"english-upper", "english-proper-names", "american-upper", "american-proper-names"} {
		for _, lvl := range allLevels {
			for _, w := range readCapitalized(*scowlDir, list, lvl) {
				if _, ok := tier[w]; !ok {
					tier[w] = tierProperName
				}
			}
		}
	}

	for w := range tier {
		if isBlocked(w, blocked, tier) {
			delete(tier, w)
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, n := range lengths {
		var lines []string
		counts := map[int]int{}
		for w, t := range tier {
			if len(w) == n {
				lines = append(lines, fmt.Sprintf("%s\t%d", w, t))
				counts[t]++
			}
		}
		sort.Strings(lines)
		path := filepath.Join(*outDir, fmt.Sprintf("words-%d.txt", n))
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("%d letters: easy=%d medium=%d hard=%d guess-only=%d proper-guessable=%d proper-rejected=%d",
			n, counts[1], counts[2], counts[3], counts[0], counts[tierCommonProper], counts[tierProperName])
	}
}

func allTierLevels() []int {
	var out []int
	for t := 1; t <= 3; t++ {
		out = append(out, tierLevels[t]...)
	}
	return out
}

var lowerWord = regexp.MustCompile(`^[a-z]{4,6}$`)

// readLevel returns the 4-6 letter lowercase words at one SCOWL level.
// Proper nouns, abbreviations, and contractions live in other SCOWL files or
// fail the regexp, so they are excluded naturally.
func readLevel(dir string, level int, includeBritish bool) []string {
	prefixes := []string{"english-words", "american-words"}
	if includeBritish {
		prefixes = append(prefixes, "british-words")
	}
	var out []string
	for _, p := range prefixes {
		path := filepath.Join(dir, fmt.Sprintf("%s.%d", p, level))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		for _, line := range mustReadLines(path) {
			// SCOWL files are Latin-1; accented words fail the regexp and drop out.
			if lowerWord.MatchString(line) {
				out = append(out, line)
			}
		}
	}
	return out
}

// readCapitalized returns the 4-6 letter words at one level of a SCOWL list
// of capitalized words ("upper" = words like "April", "proper-names" = people
// and places), lowercased. Acronyms and mixed case ("McCoy") are skipped.
func readCapitalized(dir, list string, level int) []string {
	path := filepath.Join(dir, fmt.Sprintf("%s.%d", list, level))
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	var out []string
	for _, line := range mustReadLines(path) {
		if w := strings.ToLower(line); lowerWord.MatchString(w) && line[1:] == w[1:] {
			out = append(out, w)
		}
	}
	return out
}

// isBlocked reports whether w is on the blocklist or is an inflection of a
// blocked word ("sluts" -> "slut"). A word that can also be read as the
// inflection of an innocent word ("spices" -> "spice") is kept.
func isBlocked(w string, blocked map[string]bool, known map[string]int) bool {
	if blocked[w] {
		return true
	}
	var stems []string
	for _, suffix := range []string{"s", "es", "ed", "d"} {
		if strings.HasSuffix(w, suffix) {
			stems = append(stems, strings.TrimSuffix(w, suffix))
		}
	}
	if strings.HasSuffix(w, "ies") {
		stems = append(stems, strings.TrimSuffix(w, "ies")+"y")
	}
	hit := false
	for _, stem := range stems {
		if blocked[stem] {
			hit = true
		} else if _, ok := known[stem]; ok {
			return false
		}
	}
	return hit
}

// isInflection reports whether w looks like a plural or past tense of another
// word in the bank. It is a heuristic: it may demote a few legitimate words.
func isInflection(w string, known map[string]int) bool {
	has := func(s string) bool { _, ok := known[s]; return ok }
	switch {
	case strings.HasSuffix(w, "ss"):
		return false
	case strings.HasSuffix(w, "ies") && has(strings.TrimSuffix(w, "ies")+"y"):
		return true
	case strings.HasSuffix(w, "es") && has(strings.TrimSuffix(w, "es")):
		return true
	case strings.HasSuffix(w, "s") && has(strings.TrimSuffix(w, "s")):
		return true
	case strings.HasSuffix(w, "ed") && (has(strings.TrimSuffix(w, "ed")) || has(strings.TrimSuffix(w, "d"))):
		return true
	}
	return false
}

func mustReadLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}
	return out
}
