// Package server exposes the hub over HTTP: a small JSON API for creating and
// joining games, a WebSocket per player for live play, and the embedded SPA.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/Baotluong/jotto-go/internal/game"
	"github.com/Baotluong/jotto-go/internal/hub"
	"github.com/Baotluong/jotto-go/internal/store"
)

type Server struct {
	hub            *hub.Hub
	store          *store.Store
	log            *slog.Logger
	static         fs.FS
	originPatterns []string
}

// New builds the handler. static is the built frontend (may be empty);
// originPatterns lists extra allowed WebSocket origins, e.g. the Vite dev server.
func New(h *hub.Hub, s *store.Store, log *slog.Logger, static fs.FS, originPatterns []string) http.Handler {
	srv := &Server{hub: h, store: s, log: log, static: static, originPatterns: originPatterns}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/games", srv.createGame)
	mux.HandleFunc("POST /api/games/{code}/join", srv.joinGame)
	mux.HandleFunc("GET /api/games/{code}/ws", srv.websocket)
	mux.HandleFunc("GET /healthz", srv.health)
	mux.Handle("/", srv.spa())
	return srv.logRequests(mux)
}

type createRequest struct {
	Name     string         `json:"name"`
	Settings *game.Settings `json:"settings"`
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decode(w, r, &req) {
		return
	}
	settings := game.DefaultSettings()
	if req.Settings != nil {
		settings = *req.Settings
	}
	j, err := s.hub.Create(r.Context(), settings, req.Name)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, j)
}

func (s *Server) joinGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	j, err := s.hub.Join(r.Context(), normCode(r.PathValue("code")), req.Name)
	if err != nil {
		s.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

const (
	helloTimeout = 10 * time.Second
	pingInterval = 25 * time.Second // keeps proxies from closing idle sockets
	writeTimeout = 10 * time.Second
)

// websocket upgrades the connection. The first client message must be
// {"token": "..."}; sending the token in-band keeps it out of URLs and logs.
func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.originPatterns})
	if err != nil {
		return // Accept already wrote the response
	}
	defer conn.CloseNow()
	conn.SetReadLimit(4096)
	ctx := r.Context()

	var hello struct {
		Token string `json:"token"`
	}
	helloCtx, cancel := context.WithTimeout(ctx, helloTimeout)
	err = wsjson.Read(helloCtx, conn, &hello)
	cancel()
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return
	}

	client, err := s.hub.Attach(ctx, normCode(r.PathValue("code")), hello.Token)
	switch {
	case errors.Is(err, store.ErrNotFound):
		conn.Close(4404, "game not found")
		return
	case errors.Is(err, hub.ErrBadToken):
		conn.Close(4401, "not a player in this game")
		return
	case err != nil:
		s.log.Error("attach", "err", err)
		conn.Close(websocket.StatusInternalError, "server error")
		return
	}
	defer s.hub.Detach(client)

	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	go s.writeLoop(ctx, cancel, conn, client)

	for {
		var m hub.Message
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			return
		}
		err := s.hub.Handle(ctx, client, m)
		var ue hub.UserError
		switch {
		case errors.As(err, &ue):
			s.sendError(ctx, client, userMessage(ue))
		case err != nil:
			s.log.Error("handle message", "type", m.Type, "err", err)
			s.sendError(ctx, client, "something went wrong, try again")
		}
	}
}

// writeLoop is the only goroutine that writes to conn.
func (s *Server) writeLoop(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, c *hub.Client) {
	defer cancel()
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.Send:
			wctx, wcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pctx, pcancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) sendError(ctx context.Context, c *hub.Client, msg string) {
	b, _ := json.Marshal(map[string]string{"type": "error", "error": msg})
	select {
	case c.Send <- b:
	case <-ctx.Done():
	}
}

// spa serves the built frontend, falling back to index.html so client-side
// routes like /g/ABCD work on refresh.
func (s *Server) spa() http.Handler {
	files := http.FileServerFS(s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(s.static, path); err == nil {
				if strings.HasPrefix(path, "assets/") {
					// Vite fingerprints these, so they can be cached forever.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(s.static, "index.html")
		if err != nil {
			http.Error(w, "frontend not built: run `npm run build` in web/", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}

func (s *Server) writeErr(w http.ResponseWriter, err error) {
	var ue hub.UserError
	switch {
	case errors.As(err, &ue):
		status := http.StatusBadRequest
		if errors.Is(err, game.ErrGameFull) {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": userMessage(ue)})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
	default:
		s.log.Error("request failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
	}
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.log.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
		}
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// userMessage turns a Go-style error ("not in the dictionary") into a
// sentence for players ("Not in the dictionary").
func userMessage(err error) string {
	msg := err.Error()
	r, size := utf8.DecodeRuneInString(msg)
	return string(unicode.ToUpper(r)) + msg[size:]
}

func normCode(c string) string { return strings.ToUpper(strings.TrimSpace(c)) }
