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

// A database created before migrations were versioned (user_version 0, but
// with the original tables) must upgrade without losing data.
func TestMigrateLegacyDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	g := game.New("OLD1", game.DefaultSettings(), &game.Player{Token: "t"}, 0, time.UnixMilli(1))
	if err := s.Save(ctx, g); err != nil {
		t.Fatal(err)
	}
	// Roll the file back to the pre-versioning shape.
	for _, q := range []string{`ALTER TABLE games DROP COLUMN forfeited_by`, `PRAGMA user_version = 0`} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()

	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load(ctx, "OLD1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ForfeitedBy != game.NoWinner {
		t.Errorf("ForfeitedBy = %d after migration, want %d", got.ForfeitedBy, game.NoWinner)
	}
}
