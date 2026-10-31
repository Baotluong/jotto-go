package game

// Feedback is how much a guess reveals about the secret.
type Feedback string

const (
	// Classic is true Jotto: only the number of letters in common.
	Classic Feedback = "classic"
	// Pinpoint marks every letter: right spot, wrong spot, or not in the word.
	Pinpoint Feedback = "pinpoint"
)

func (f Feedback) Valid() bool {
	return f == Classic || f == Pinpoint
}

// Mark is per-letter feedback in Pinpoint mode.
type Mark string

const (
	Hit     Mark = "hit"     // right letter, right spot
	Present Mark = "present" // in the word, wrong spot
	Miss    Mark = "miss"
)

// Result is the feedback for one guess. Marks is set only in Pinpoint mode.
type Result struct {
	Common int    `json:"common"` // letters in common
	Marks  []Mark `json:"marks,omitempty"`
	Solved bool   `json:"solved"`
}

// Score compares guess against secret. Both must be lowercase a-z and the
// same length.
func Score(guess, secret string, mode Feedback) Result {
	r := Result{
		Common: CommonLetters(guess, secret),
		Solved: guess == secret,
	}
	if mode == Pinpoint {
		r.Marks = PinpointMarks(guess, secret)
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

// PinpointMarks marks exact matches first, then marks remaining letters present
// left to right while unmatched copies remain in the secret.
func PinpointMarks(guess, secret string) []Mark {
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
