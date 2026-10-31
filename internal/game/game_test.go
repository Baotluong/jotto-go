package game

import (
	"errors"
	"testing"
	"time"

	"github.com/Baotluong/jotto-go/internal/words"
)

// fakeDict accepts a fixed set of words so tests don't depend on the bank.
type fakeDict struct{ next string }

var dictWords = map[string]words.Difficulty{
	"crane": 1, "slate": 1, "house": 1, "mouse": 1, "plant": 2, "quirk": 3,
	"apple": 1, "sheep": 0,
}

func (fakeDict) IsWord(w string) bool { _, ok := dictWords[w]; return ok }
func (fakeDict) SecretAllowed(w string, d words.Difficulty) bool {
	t, ok := dictWords[w]
	return ok && t > 0 && t <= d
}
func (f fakeDict) RandomSecret(int, words.Difficulty, bool) (string, error) { return f.next, nil }

var now = time.Unix(0, 0)

// newPlaying returns a game in the Playing phase with seat 0 moving first.
func newPlaying(t *testing.T, s0, s1 string) *Game {
	t.Helper()
	d := fakeDict{}
	g := New("ABCD", DefaultSettings(), &Player{Token: "a"}, 0, now)
	if err := g.Join(&Player{Token: "b"}, d, now); err != nil {
		t.Fatal(err)
	}
	if err := g.SetSecret(0, s0, d, now); err != nil {
		t.Fatal(err)
	}
	if err := g.SetSecret(1, s1, d, now); err != nil {
		t.Fatal(err)
	}
	if g.Phase != Playing {
		t.Fatalf("phase = %s, want playing", g.Phase)
	}
	return g
}

func mustGuess(t *testing.T, g *Game, seat int, w string) Guess {
	t.Helper()
	gs, err := g.MakeGuess(seat, w, fakeDict{}, now)
	if err != nil {
		t.Fatalf("seat %d guess %q: %v", seat, w, err)
	}
	return gs
}

func TestSecretValidation(t *testing.T) {
	d := fakeDict{}
	g := New("ABCD", DefaultSettings(), &Player{}, 0, now)
	if err := g.SetSecret(0, "crane", d, now); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("secret before join: %v", err)
	}
	g.Join(&Player{}, d, now)
	for _, w := range []string{"cran", "CRANE", "zzzzz", "apple", "quirk"} {
		if err := g.SetSecret(0, w, d, now); err == nil {
			t.Errorf("secret %q accepted", w)
		}
	}
	if err := g.SetSecret(0, "plant", d, now); err != nil {
		t.Errorf("medium secret rejected in medium game: %v", err)
	}
}

func TestAutoSecrets(t *testing.T) {
	s := DefaultSettings()
	s.AutoSecrets = true
	g := New("ABCD", s, &Player{}, 0, now)
	if err := g.Join(&Player{}, fakeDict{next: "house"}, now); err != nil {
		t.Fatal(err)
	}
	if g.Phase != Playing || g.Players[0].Secret != "house" || g.Players[1].Secret != "house" {
		t.Errorf("auto secrets: phase %s, secrets %q %q", g.Phase, g.Players[0].Secret, g.Players[1].Secret)
	}
}

func TestTurnOrder(t *testing.T) {
	g := newPlaying(t, "crane", "house")
	if _, err := g.MakeGuess(1, "slate", fakeDict{}, now); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("out-of-turn guess: %v", err)
	}
	if _, err := g.MakeGuess(0, "zzzzz", fakeDict{}, now); !errors.Is(err, ErrNotAWord) {
		t.Errorf("non-word: %v", err)
	}
	// Guesses don't need to obey the unique-letters rule.
	gs := mustGuess(t, g, 0, "sheep")
	if gs.Result.Common != 3 { // s, h, e vs house
		t.Errorf("sheep vs house common = %d", gs.Result.Common)
	}
	if g.Turn != 1 {
		t.Errorf("turn = %d after seat 0 guessed", g.Turn)
	}
}

func TestSecondPlayerWinsOutright(t *testing.T) {
	g := newPlaying(t, "crane", "house")
	mustGuess(t, g, 0, "slate")
	mustGuess(t, g, 1, "crane")
	if g.Phase != Finished || g.Winner != 1 {
		t.Errorf("phase %s winner %d", g.Phase, g.Winner)
	}
}

func TestFirstPlayerSolvesThenLastChance(t *testing.T) {
	t.Run("opponent misses", func(t *testing.T) {
		g := newPlaying(t, "crane", "house")
		mustGuess(t, g, 0, "house")
		if g.Phase != Playing || !g.LastChance() || g.Turn != 1 {
			t.Fatalf("expected last chance for seat 1, phase %s turn %d", g.Phase, g.Turn)
		}
		mustGuess(t, g, 1, "slate")
		if g.Phase != Finished || g.Winner != 0 {
			t.Errorf("phase %s winner %d", g.Phase, g.Winner)
		}
	})
	t.Run("opponent ties", func(t *testing.T) {
		g := newPlaying(t, "crane", "house")
		mustGuess(t, g, 0, "house")
		mustGuess(t, g, 1, "crane")
		if g.Phase != Finished || g.Winner != Draw {
			t.Errorf("phase %s winner %d", g.Phase, g.Winner)
		}
	})
}

func TestNoGuessAfterFinish(t *testing.T) {
	g := newPlaying(t, "crane", "house")
	mustGuess(t, g, 0, "slate")
	mustGuess(t, g, 1, "crane")
	if _, err := g.MakeGuess(0, "house", fakeDict{}, now); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("guess after finish: %v", err)
	}
}

func TestRepeatGuessRejected(t *testing.T) {
	g := newPlaying(t, "crane", "house")
	mustGuess(t, g, 0, "slate")
	mustGuess(t, g, 1, "slate") // the opponent may guess the same word
	if _, err := g.MakeGuess(0, "slate", fakeDict{}, now); !errors.Is(err, ErrAlreadyGuessed) {
		t.Errorf("repeat guess: %v", err)
	}
	if g.Turn != 0 || len(g.Guesses) != 2 {
		t.Errorf("rejected guess changed state: turn %d, %d guesses", g.Turn, len(g.Guesses))
	}
}

func TestForfeit(t *testing.T) {
	g := New("ABCD", DefaultSettings(), &Player{}, 0, now)
	if err := g.Forfeit(0, now); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("forfeit before opponent joined: %v", err)
	}

	g = newPlaying(t, "crane", "house")
	mustGuess(t, g, 0, "slate")
	if err := g.Forfeit(0, now); err != nil {
		t.Fatal(err)
	}
	if g.Phase != Finished || g.Winner != 1 || g.ForfeitedBy != 0 {
		t.Errorf("phase %s winner %d forfeitedBy %d", g.Phase, g.Winner, g.ForfeitedBy)
	}
	if err := g.Forfeit(1, now); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("forfeit after finish: %v", err)
	}
}
