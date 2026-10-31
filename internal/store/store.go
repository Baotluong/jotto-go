// Package store persists games to SQLite so they survive restarts and
// players can reconnect. There are no user accounts: a player is identified
// only by the random token issued when they create or join a game.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Baotluong/jotto-go/internal/game"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

var ErrNotFound = errors.New("game not found")

type Store struct{ db *sql.DB }

// Open opens (or creates) the database at path and applies the schema.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer at a time; a single connection avoids
	// SQLITE_BUSY churn and is plenty for this workload.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Save writes the game's current state. Guesses are append-only, so only
// ones not already stored are inserted.
func (s *Store) Save(ctx context.Context, g *game.Game) error {
	settings, err := json.Marshal(g.Settings)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO games (code, settings, phase, first_seat, turn, solved0, solved1, winner, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET
			phase = excluded.phase, turn = excluded.turn,
			solved0 = excluded.solved0, solved1 = excluded.solved1,
			winner = excluded.winner, updated_at = excluded.updated_at`,
		g.Code, settings, g.Phase, g.First, g.Turn, g.SolvedBy[0], g.SolvedBy[1], g.Winner,
		g.Created.UnixMilli(), g.Updated.UnixMilli())
	if err != nil {
		return err
	}
	for seat, p := range g.Players {
		if p == nil {
			continue
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO players (game_code, seat, token, name, secret) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (game_code, seat) DO UPDATE SET name = excluded.name, secret = excluded.secret`,
			g.Code, seat, p.Token, p.Name, p.Secret)
		if err != nil {
			return err
		}
	}
	for i, gs := range g.Guesses {
		result, err := json.Marshal(gs.Result)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO guesses (game_code, seq, seat, word, result, at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT DO NOTHING`,
			g.Code, i, gs.Seat, gs.Word, result, gs.At.UnixMilli())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Load reads a full game by code.
func (s *Store) Load(ctx context.Context, code string) (*game.Game, error) {
	g := &game.Game{Code: code}
	var settings []byte
	var created, updated int64
	err := s.db.QueryRowContext(ctx, `
		SELECT settings, phase, first_seat, turn, solved0, solved1, winner, created_at, updated_at
		FROM games WHERE code = ?`, code).
		Scan(&settings, &g.Phase, &g.First, &g.Turn, &g.SolvedBy[0], &g.SolvedBy[1], &g.Winner, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(settings, &g.Settings); err != nil {
		return nil, err
	}
	g.Created, g.Updated = time.UnixMilli(created), time.UnixMilli(updated)

	rows, err := s.db.QueryContext(ctx, `SELECT seat, token, name, secret FROM players WHERE game_code = ?`, code)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var seat int
		p := &game.Player{}
		if err := rows.Scan(&seat, &p.Token, &p.Name, &p.Secret); err != nil {
			rows.Close()
			return nil, err
		}
		g.Players[seat] = p
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT seat, word, result, at FROM guesses WHERE game_code = ? ORDER BY seq`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var gs game.Guess
		var result []byte
		var at int64
		if err := rows.Scan(&gs.Seat, &gs.Word, &result, &at); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(result, &gs.Result); err != nil {
			return nil, err
		}
		gs.At = time.UnixMilli(at)
		g.Guesses = append(g.Guesses, gs)
	}
	return g, rows.Err()
}

// Exists reports whether a game code is taken.
func (s *Store) Exists(ctx context.Context, code string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM games WHERE code = ?`, code).Scan(&n)
	return n > 0, err
}

// DeleteInactive removes games not updated since before cutoff, so the
// database stays small without any manual cleanup.
func (s *Store) DeleteInactive(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM games WHERE updated_at < ?`, cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
