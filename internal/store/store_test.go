package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Baotluong/jotto-go/internal/game"
)

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	now := time.UnixMilli(time.Now().UnixMilli())
	g := game.New("WXYZ", game.DefaultSettings(), &game.Player{Token: "t0", Name: "Ann", Secret: "crane"}, 1, now)
	g.Players[1] = &game.Player{Token: "t1", Name: "Ben", Secret: "house"}
	g.Phase = game.Playing
	g.Guesses = []game.Guess{{Seat: 1, Word: "slate", Result: game.Result{Common: 3}, At: now}}
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	// A second save with one more guess must append, not duplicate.
	g.Guesses = append(g.Guesses, game.Guess{Seat: 0, Word: "mouse", Result: game.Result{Common: 4}, At: now})
	g.Turn = 1
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}

	got, err := s.Load(ctx, "WXYZ")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, g) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, g)
	}

	if _, err := s.Load(ctx, "NOPE"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing game: %v", err)
	}

	n, err := s.DeleteInactive(ctx, now.Add(time.Hour))
	if err != nil || n != 1 {
		t.Errorf("DeleteInactive = %d, %v", n, err)
	}
	if ok, _ := s.Exists(ctx, "WXYZ"); ok {
		t.Error("game still exists after delete")
	}
}
