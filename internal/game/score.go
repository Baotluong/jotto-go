package game

// Feedback is how much a guess reveals about the secret.
type Feedback string

const (
	// Classic is true Jotto: only the number of letters in common.
	Classic Feedback = "classic"
	// Positional adds how many of those letters are in the right spot,
	// without saying which (Mastermind-style).
	Positional Feedback = "positional"
	// Wordle marks every letter as hit, present, or miss.
	Wordle Feedback = "wordle"
)

func (f Feedback) Valid() bool {
	return f == Classic || f == Positional || f == Wordle
}

// Mark is per-letter Wordle feedback.
type Mark string

const (
	Hit     Mark = "hit"     // right letter, right spot
	Present Mark = "present" // in the word, wrong spot
	Miss    Mark = "miss"
)

// Result is the feedback for one guess. Only the fields for the game's
// Feedback mode are populated.
type Result struct {
	Common int    `json:"common"`          // letters in common (all modes)
	Exact  int    `json:"exact,omitempty"` // Positional and Wordle
	Marks  []Mark `json:"marks,omitempty"` // Wordle only
	Solved bool   `json:"solved"`
}

// Score compares guess against secret. Both must be lowercase a-z and the
// same length.
func Score(guess, secret string, mode Feedback) Result {
	r := Result{
		Common: CommonLetters(guess, secret),
		Solved: guess == secret,
	}
	if mode == Positional || mode == Wordle {
		for i := range len(guess) {
			if guess[i] == secret[i] {
				r.Exact++
			}
		}
	}
	if mode == Wordle {
		r.Marks = WordleMarks(guess, secret)
	}
	return r
}

// CommonLetters counts letters shared by a and b, counting a repeated
// letter only as many times as it appears in both: APPLE vs PAPER = 4.
func CommonLetters(a, b string) int {
	var ca, cb [26]int
	for i := range len(a) {
		ca[a[i]-'a']++
	}
	for i := range len(b) {
		cb[b[i]-'a']++
	}
	n := 0
	for i := range 26 {
		n += min(ca[i], cb[i])
	}
	return n
}

// WordleMarks applies Wordle's rules: exact matches first, then remaining
// letters are marked present left to right while unmatched copies remain.
func WordleMarks(guess, secret string) []Mark {
	marks := make([]Mark, len(guess))
	var left [26]int
	for i := range len(guess) {
		if guess[i] == secret[i] {
			marks[i] = Hit
		} else {
			left[secret[i]-'a']++
		}
	}
	for i := range len(guess) {
		if marks[i] == Hit {
			continue
		}
		if c := guess[i] - 'a'; left[c] > 0 {
			marks[i] = Present
			left[c]--
		} else {
			marks[i] = Miss
		}
	}
	return marks
}
