// Command wordgen builds the embedded word banks from a SCOWL "final" directory.
//
// SCOWL splits English into size levels (10 = most common ... 95 = obscure).
// We map those levels onto three secret-word difficulty tiers and use a wider
// slice of SCOWL as the "is this a real word" dictionary for guesses.
//
// Output: one file per word length, internal/words/data/words-<n>.txt, each
// line "word<TAB>tier" where tier 1-3 means the word may be a secret at that
// difficulty, and 0 means it is accepted as a guess only.
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

	for w := range tier {
		if blocked[w] {
			delete(tier, w)
		}
	}
	// Plurals and past tenses ("cats", "baked") make poor secrets: demote them.
	for w, t := range tier {
		if t > 0 && isInflection(w, tier) {
			tier[w] = 0
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
		log.Printf("%d letters: easy=%d medium=%d hard=%d guess-only=%d", n, counts[1], counts[2], counts[3], counts[0])
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
