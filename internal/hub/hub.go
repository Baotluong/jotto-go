// Package hub owns live games: it loads them from the store, applies player
// actions under a per-game lock, persists the result, and pushes each player
// a fresh View over their WebSocket.
//
// The service is designed to run as a single instance, so in-memory rooms
// are the source of truth for connected clients and SQLite is the source of
// truth for game state.
package hub

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/Baotluong/jotto-go/internal/game"
	"github.com/Baotluong/jotto-go/internal/store"
	"github.com/Baotluong/jotto-go/internal/webhook"
	"github.com/Baotluong/jotto-go/internal/words"
)

// ErrBadToken means the token doesn't belong to a player in this game.
var ErrBadToken = errors.New("unknown player token")

// UserError wraps an error that is safe and useful to show a player.
type UserError struct{ Err error }

func (e UserError) Error() string { return e.Err.Error() }
func (e UserError) Unwrap() error { return e.Err }

type Hub struct {
	store *store.Store
	dict  *words.Bank
	hooks *webhook.Notifier
	log   *slog.Logger
	now   func() time.Time

	mu    sync.Mutex
	rooms map[string]*room
}

func New(s *store.Store, dict *words.Bank, hooks *webhook.Notifier, log *slog.Logger) *Hub {
	return &Hub{store: s, dict: dict, hooks: hooks, log: log, now: time.Now, rooms: map[string]*room{}}
}

// room is a game loaded in memory. refs counts clients plus in-flight
// requests; the room is dropped from memory when it reaches zero.
type room struct {
	mu      sync.Mutex
	g       *game.Game
	clients map[*Client]struct{}
	refs    int
}

func (r *room) connected(seat int) bool {
	for c := range r.clients {
		if c.seat == seat {
			return true
		}
	}
	return false
}

// Client is one WebSocket connection. Outgoing messages are queued on Send
// and written by the connection's writer goroutine.
type Client struct {
	seat int
	room *room
	code string
	Send chan []byte
}

func (h *Hub) acquire(ctx context.Context, code string) (*room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[code]; ok {
		r.refs++
		return r, nil
	}
	g, err := h.store.Load(ctx, code)
	if err != nil {
		return nil, err
	}
	r := &room{g: g, clients: map[*Client]struct{}{}, refs: 1}
	h.rooms[code] = r
	return r, nil
}

func (h *Hub) release(code string, r *room) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r.refs--
	if r.refs == 0 {
		delete(h.rooms, code)
	}
}

// Joined is returned to a player when they create or join a game.
type Joined struct {
	Code  string `json:"code"`
	Token string `json:"token"`
	Seat  int    `json:"seat"`
}

// Create starts a new game with the caller in seat 0.
func (h *Hub) Create(ctx context.Context, s game.Settings, name string) (Joined, error) {
	if err := s.Validate(); err != nil {
		return Joined{}, UserError{err}
	}
	code, err := h.newCode(ctx)
	if err != nil {
		return Joined{}, err
	}
	token := newToken()
	p := &game.Player{Token: token, Name: cleanName(name, 0)}
	var g *game.Game
	if s.Solo {
		if g, err = game.NewSolo(code, s, p, h.dict, h.now()); err != nil {
			return Joined{}, err
		}
	} else {
		g = game.New(code, s, p, mrand.IntN(2), h.now())
	}
	if err := h.store.Save(ctx, g); err != nil {
		return Joined{}, err
	}
	h.hooks.Send(webhook.Event{Type: "game.created", Code: code})
	if g.Phase == game.Playing {
		h.hooks.Send(webhook.Event{Type: "game.started", Code: code})
	}
	return Joined{Code: code, Token: token, Seat: 0}, nil
}

// Join seats a second player.
func (h *Hub) Join(ctx context.Context, code, name string) (Joined, error) {
	token := newToken()
	err := h.mutate(ctx, code, func(g *game.Game, now time.Time) error {
		return g.Join(&game.Player{Token: token, Name: cleanName(name, 1)}, h.dict, now)
	})
	if err != nil {
		return Joined{}, err
	}
	return Joined{Code: code, Token: token, Seat: 1}, nil
}

// Attach registers a WebSocket client for the player holding token and
// queues the current state for them.
func (h *Hub) Attach(ctx context.Context, code, token string) (*Client, error) {
	// The solo house seat has an empty token; it must never match.
	if token == "" {
		return nil, ErrBadToken
	}
	r, err := h.acquire(ctx, code)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	seat := -1
	for i, p := range r.g.Players {
		if p != nil && subtle.ConstantTimeCompare([]byte(p.Token), []byte(token)) == 1 {
			seat = i
		}
	}
	if seat < 0 {
		r.mu.Unlock()
		h.release(code, r)
		return nil, ErrBadToken
	}
	c := &Client{seat: seat, room: r, code: code, Send: make(chan []byte, 16)}
	r.clients[c] = struct{}{}
	h.broadcastLocked(r) // tells the opponent we're online, and gives c its state
	r.mu.Unlock()
	return c, nil
}

// Detach removes a client when its connection closes.
func (h *Hub) Detach(c *Client) {
	r := c.room
	r.mu.Lock()
	delete(r.clients, c)
	h.broadcastLocked(r)
	r.mu.Unlock()
	h.release(c.code, r)
}

// Message is what clients send over the WebSocket.
type Message struct {
	Type string `json:"type"` // "secret", "guess", or "forfeit"
	Word string `json:"word"` // for "secret", empty means pick one for me
}

// Handle applies a client's message. A UserError should be reported back to
// that client; other errors are server faults.
func (h *Hub) Handle(ctx context.Context, c *Client, m Message) error {
	word := strings.ToLower(strings.TrimSpace(m.Word))
	switch m.Type {
	case "secret":
		return h.mutateRoom(ctx, c.room, func(g *game.Game, now time.Time) error {
			return g.SetSecret(c.seat, word, h.dict, now)
		})
	case "guess":
		return h.mutateRoom(ctx, c.room, func(g *game.Game, now time.Time) error {
			_, err := g.MakeGuess(c.seat, word, h.dict, now)
			return err
		})
	case "forfeit":
		return h.mutateRoom(ctx, c.room, func(g *game.Game, now time.Time) error {
			return g.Forfeit(c.seat, now)
		})
	default:
		return UserError{errors.New("unknown message type")}
	}
}

func (h *Hub) mutate(ctx context.Context, code string, fn func(*game.Game, time.Time) error) error {
	r, err := h.acquire(ctx, code)
	if err != nil {
		return err
	}
	defer h.release(code, r)
	return h.mutateRoom(ctx, r, fn)
}

// mutateRoom applies fn, persists, and broadcasts. Rule violations leave the
// game untouched (game methods validate before changing state). If the save
// fails, the in-memory game is reloaded so it never drifts from the database.
func (h *Hub) mutateRoom(ctx context.Context, r *room, fn func(*game.Game, time.Time) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := r.g.Phase
	if err := fn(r.g, h.now()); err != nil {
		return UserError{err}
	}
	if err := h.store.Save(ctx, r.g); err != nil {
		if fresh, lerr := h.store.Load(ctx, r.g.Code); lerr == nil {
			r.g = fresh
		}
		return err
	}
	h.broadcastLocked(r)
	h.emitPhaseChange(r.g, before)
	return nil
}

func (h *Hub) emitPhaseChange(g *game.Game, before game.Phase) {
	if g.Phase == before {
		return
	}
	switch g.Phase {
	case game.Playing:
		h.hooks.Send(webhook.Event{Type: "game.started", Code: g.Code})
	case game.Finished:
		h.hooks.Send(webhook.Event{Type: "game.finished", Code: g.Code, Winner: &g.Winner, Guesses: len(g.Guesses)})
	}
}

// broadcastLocked sends every connected client its own view. r.mu must be held.
func (h *Hub) broadcastLocked(r *room) {
	for c := range r.clients {
		msg, err := json.Marshal(struct {
			Type  string `json:"type"`
			State View   `json:"state"`
		}{"state", r.viewFor(c.seat)})
		if err != nil {
			h.log.Error("marshal view", "err", err)
			continue
		}
		select {
		case c.Send <- msg:
		default:
			// A client this far behind is stuck; it will get fresh state on reconnect.
			h.log.Warn("dropping state for slow client", "code", r.g.Code, "seat", c.seat)
		}
	}
}

// Cleanup deletes games idle longer than maxAge. If one of them is still
// loaded in memory, its next Save simply re-creates the rows.
func (h *Hub) Cleanup(ctx context.Context, maxAge time.Duration) {
	n, err := h.store.DeleteInactive(ctx, h.now().Add(-maxAge))
	if err != nil {
		h.log.Error("cleanup", "err", err)
		return
	}
	if n > 0 {
		h.log.Info("cleaned up inactive games", "count", n)
	}
}

// Codes avoid look-alike characters (0/O, 1/I/L) so they're easy to read aloud.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func (h *Hub) newCode(ctx context.Context) (string, error) {
	for range 20 {
		b := make([]byte, 4)
		for i := range b {
			b[i] = codeAlphabet[mrand.IntN(len(codeAlphabet))]
		}
		code := string(b)
		taken, err := h.store.Exists(ctx, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, nil
		}
	}
	return "", errors.New("could not allocate a game code")
}

func newToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func cleanName(name string, seat int) string {
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 20 {
		name = string(r[:20])
	}
	if name == "" {
		return []string{"Player 1", "Player 2"}[seat]
	}
	return name
}
