// Package game is the pure Jotto rules engine: settings, turn order, and
// win/draw detection. It does no I/O, so every rule is unit-testable.
package game

import (
	"errors"
	"fmt"
	"time"

	"github.com/Baotluong/jotto-go/internal/words"
)

// Settings are fixed when the game is created.
type Settings struct {
	Length        int              `json:"length"`        // 4, 5, or 6
	UniqueLetters bool             `json:"uniqueLetters"` // secrets may not repeat a letter
	Difficulty    words.Difficulty `json:"difficulty"`
	Feedback      Feedback         `json:"feedback"`
	AutoSecrets   bool             `json:"autoSecrets"` // server picks both secrets
}

// DefaultSettings is classic Jotto.
func DefaultSettings() Settings {
	return Settings{Length: 5, UniqueLetters: true, Difficulty: words.Medium, Feedback: Classic}
}

func (s Settings) Validate() error {
	if s.Length < 4 || s.Length > 6 {
		return errors.New("length must be 4, 5, or 6")
	}
	if !s.Difficulty.Valid() {
		return errors.New("difficulty must be 1, 2, or 3")
	}
	if !s.Feedback.Valid() {
		return errors.New("feedback must be classic, positional, or wordle")
	}
	return nil
}

// Phase is where the game is in its lifecycle.
type Phase string

const (
	Waiting  Phase = "waiting"  // second player hasn't joined
	Choosing Phase = "choosing" // players are picking secrets
	Playing  Phase = "playing"
	Finished Phase = "finished"
)

// Outcome values for a finished game.
const (
	NoWinner = -1
	Draw     = 2
)

type Player struct {
	Token  string
	Name   string
	Secret string
}

type Guess struct {
	Seat   int       `json:"seat"`
	Word   string    `json:"word"`
	Result Result    `json:"result"`
	At     time.Time `json:"at"`
}

// Dictionary is what the rules need from the word bank.
type Dictionary interface {
	IsWord(w string) bool
	SecretAllowed(w string, d words.Difficulty) bool
	RandomSecret(length int, d words.Difficulty, unique bool) (string, error)
}

// Game is the full state of one match between seat 0 and seat 1.
type Game struct {
	Code     string
	Settings Settings
	Players  [2]*Player // Players[1] is nil until someone joins
	Phase    Phase
	First    int // seat that guesses first
	Turn     int // seat whose turn it is
	Guesses  []Guess
	// SolvedBy[seat] is set when that seat has guessed the other's word.
	SolvedBy [2]bool
	Winner   int // seat, Draw, or NoWinner
	Created  time.Time
	Updated  time.Time
}

// Errors returned for rule violations; the server shows these to players.
var (
	ErrNotYourTurn = errors.New("it's not your turn")
	ErrWrongPhase  = errors.New("that can't be done right now")
	ErrNotAWord    = errors.New("not in the dictionary")
	ErrGameFull    = errors.New("game is full")
)

// New creates a game with the creator in seat 0. first picks who opens.
func New(code string, s Settings, creator *Player, first int, now time.Time) *Game {
	return &Game{
		Code:     code,
		Settings: s,
		Players:  [2]*Player{creator, nil},
		Phase:    Waiting,
		First:    first,
		Turn:     first,
		Winner:   NoWinner,
		Created:  now,
		Updated:  now,
	}
}

// Join seats the second player. With AutoSecrets on, both secrets are
// drawn and play starts immediately.
func (g *Game) Join(p *Player, dict Dictionary, now time.Time) error {
	if g.Players[1] != nil {
		return ErrGameFull
	}
	// Draw secrets before changing anything, so a failure leaves g untouched.
	var picks [2]string
	if s := g.Settings; s.AutoSecrets {
		for i := range picks {
			w, err := dict.RandomSecret(s.Length, s.Difficulty, s.UniqueLetters)
			if err != nil {
				return err
			}
			picks[i] = w
		}
	}
	g.Players[1] = p
	g.Phase = Choosing
	g.Updated = now
	if g.Settings.AutoSecrets {
		g.Players[0].Secret, p.Secret = picks[0], picks[1]
		g.Phase = Playing
	}
	return nil
}

// SetSecret records a seat's secret word. An empty word asks the server to
// pick one. Once both secrets are in, play begins.
func (g *Game) SetSecret(seat int, word string, dict Dictionary, now time.Time) error {
	if g.Phase != Choosing {
		return ErrWrongPhase
	}
	s := g.Settings
	if word == "" {
		w, err := dict.RandomSecret(s.Length, s.Difficulty, s.UniqueLetters)
		if err != nil {
			return err
		}
		word = w
	} else {
		if err := checkShape(word, s.Length); err != nil {
			return err
		}
		if s.UniqueLetters && !words.HasUniqueLetters(word) {
			return errors.New("secret can't repeat a letter in this game")
		}
		if !dict.SecretAllowed(word, s.Difficulty) {
			return errors.New("pick a more common word for this difficulty")
		}
	}
	g.Players[seat].Secret = word
	g.Updated = now
	if g.Players[0].Secret != "" && g.Players[1] != nil && g.Players[1].Secret != "" {
		g.Phase = Playing
	}
	return nil
}

// MakeGuess plays seat's guess at the opponent's secret.
//
// Fairness: if the first player solves, the second player gets one last
// guess to tie, since otherwise moving first would be a built-in advantage.
func (g *Game) MakeGuess(seat int, word string, dict Dictionary, now time.Time) (Guess, error) {
	if g.Phase != Playing {
		return Guess{}, ErrWrongPhase
	}
	if seat != g.Turn {
		return Guess{}, ErrNotYourTurn
	}
	if err := checkShape(word, g.Settings.Length); err != nil {
		return Guess{}, err
	}
	if !dict.IsWord(word) {
		return Guess{}, ErrNotAWord
	}

	opp := 1 - seat
	gs := Guess{Seat: seat, Word: word, Result: Score(word, g.Players[opp].Secret, g.Settings.Feedback), At: now}
	g.Guesses = append(g.Guesses, gs)
	g.Updated = now
	if gs.Result.Solved {
		g.SolvedBy[seat] = true
	}

	switch {
	case seat == g.First && gs.Result.Solved:
		// Opponent gets a last-chance turn.
		g.Turn = opp
	case seat != g.First && g.SolvedBy[opp]:
		// That was the last-chance turn.
		g.finish(now)
	case gs.Result.Solved:
		g.finish(now)
	default:
		g.Turn = opp
	}
	return gs, nil
}

// LastChance reports whether the current turn is the tie-or-lose guess.
func (g *Game) LastChance() bool {
	return g.Phase == Playing && g.SolvedBy[g.First]
}

func (g *Game) finish(now time.Time) {
	g.Phase = Finished
	g.Updated = now
	switch {
	case g.SolvedBy[0] && g.SolvedBy[1]:
		g.Winner = Draw
	case g.SolvedBy[0]:
		g.Winner = 0
	default:
		g.Winner = 1
	}
}

func checkShape(word string, length int) error {
	if len(word) != length {
		return fmt.Errorf("word must be %d letters", length)
	}
	for i := range len(word) {
		if word[i] < 'a' || word[i] > 'z' {
			return errors.New("letters a-z only")
		}
	}
	return nil
}
