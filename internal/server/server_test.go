package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Baotluong/jotto-go/internal/game"
	"github.com/Baotluong/jotto-go/internal/hub"
	"github.com/Baotluong/jotto-go/internal/store"
	"github.com/Baotluong/jotto-go/internal/words"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	dict, err := words.Load()
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	static := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	ts := httptest.NewServer(New(hub.New(db, dict, nil, log), db, log, static, nil))
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, url string, body any) (int, hub.Joined) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var j hub.Joined
	json.NewDecoder(resp.Body).Decode(&j)
	return resp.StatusCode, j
}

type player struct {
	t    *testing.T
	conn *websocket.Conn
}

func connect(t *testing.T, ts *httptest.Server, j hub.Joined) *player {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/games/" + j.Code + "/ws"
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	if err := wsjson.Write(ctx, conn, map[string]string{"token": j.Token}); err != nil {
		t.Fatal(err)
	}
	return &player{t, conn}
}

func (p *player) send(typ, word string) {
	p.t.Helper()
	if err := wsjson.Write(context.Background(), p.conn, hub.Message{Type: typ, Word: word}); err != nil {
		p.t.Fatal(err)
	}
}

type serverMsg struct {
	Type  string   `json:"type"`
	State hub.View `json:"state"`
	Error string   `json:"error"`
}

// waitFor reads messages until one satisfies ok.
func (p *player) waitFor(ok func(serverMsg) bool) serverMsg {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var m serverMsg
		if err := wsjson.Read(ctx, p.conn, &m); err != nil {
			p.t.Fatalf("waiting for message: %v", err)
		}
		if ok(m) {
			return m
		}
	}
}

func TestFullGame(t *testing.T) {
	ts := newTestServer(t)
	settings := game.DefaultSettings()
	settings.Difficulty = words.Hard

	status, a := postJSON(t, ts.URL+"/api/games", map[string]any{"name": "Ann", "settings": settings})
	if status != http.StatusCreated || len(a.Code) != 4 {
		t.Fatalf("create: %d %+v", status, a)
	}
	pa := connect(t, ts, a)
	pa.waitFor(func(m serverMsg) bool { return m.State.Phase == game.Waiting })

	status, b := postJSON(t, ts.URL+"/api/games/"+strings.ToLower(a.Code)+"/join", map[string]string{"name": "Ben"})
	if status != http.StatusOK || b.Seat != 1 {
		t.Fatalf("join: %d %+v", status, b)
	}
	if status, _ := postJSON(t, ts.URL+"/api/games/"+a.Code+"/join", map[string]string{}); status != http.StatusConflict {
		t.Errorf("third player join: %d, want 409", status)
	}
	pb := connect(t, ts, b)

	pa.send("secret", "crane")
	pa.waitFor(func(m serverMsg) bool { return m.State.YourSecret == "crane" })
	pb.send("secret", "house")
	s := pb.waitFor(func(m serverMsg) bool { return m.State.Phase == game.Playing }).State
	if s.OpponentSecret != "" {
		t.Fatal("opponent secret leaked during play")
	}

	first, second := pa, pb
	firstSecret := "crane"
	if s.Turn == 1 {
		first, second, firstSecret = pb, pa, "house"
	}
	opponentOf := map[string]string{"crane": "house", "house": "crane"}

	first.send("guess", "qqqqq")
	first.waitFor(func(m serverMsg) bool { return m.Type == "error" })
	second.send("guess", "slate")
	second.waitFor(func(m serverMsg) bool { return m.Type == "error" }) // not their turn

	// First player solves; second gets a last chance and misses.
	first.send("guess", opponentOf[firstSecret])
	s = second.waitFor(func(m serverMsg) bool { return m.State.LastChance }).State
	second.send("guess", "slate")
	s = first.waitFor(func(m serverMsg) bool { return m.State.Phase == game.Finished }).State
	if s.Winner != s.You {
		t.Errorf("winner = %d, want first player %d", s.Winner, s.You)
	}
	if s.OpponentSecret == "" || len(s.Guesses) != 2 {
		t.Errorf("finished state: %+v", s)
	}
}

func TestBadToken(t *testing.T) {
	ts := newTestServer(t)
	_, a := postJSON(t, ts.URL+"/api/games", map[string]any{})
	p := connect(t, ts, hub.Joined{Code: a.Code, Token: "nope"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := p.conn.Read(ctx)
	if websocket.CloseStatus(err) != 4401 {
		t.Errorf("close status = %v", err)
	}
}

func TestSPAFallback(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/g/ABCD")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Errorf("SPA fallback: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
