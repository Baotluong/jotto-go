// Command server runs the Jotto web service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Baotluong/jotto-go/internal/hub"
	"github.com/Baotluong/jotto-go/internal/server"
	"github.com/Baotluong/jotto-go/internal/store"
	"github.com/Baotluong/jotto-go/internal/webhook"
	"github.com/Baotluong/jotto-go/internal/words"
	"github.com/Baotluong/jotto-go/web"
)

const gameMaxAge = 7 * 24 * time.Hour

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dict, err := words.Load()
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, env("DB_PATH", "jotto.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	hooks := webhook.New(os.Getenv("WEBHOOK_URL"), os.Getenv("WEBHOOK_SECRET"), log)
	go hooks.Run(ctx)

	h := hub.New(db, dict, hooks, log)
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			h.Cleanup(ctx, gameMaxAge)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()

	var origins []string
	if o := os.Getenv("ALLOWED_ORIGINS"); o != "" {
		origins = strings.Split(o, ",")
	}
	srv := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           server.New(h, db, log, web.Dist(), origins),
		ReadHeaderTimeout: 10 * time.Second,
		// Request contexts end on shutdown, which closes open WebSockets.
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
