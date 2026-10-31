package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeliverSignedWithRetry(t *testing.T) {
	got := make(chan Event, 1)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway) // first attempt fails, should retry
			return
		}
		body, _ := io.ReadAll(r.Body)
		mac := hmac.New(sha256.New, []byte("s3cret"))
		mac.Write(body)
		if r.Header.Get(SignatureHeader) != hex.EncodeToString(mac.Sum(nil)) {
			t.Error("bad signature")
		}
		var e Event
		json.Unmarshal(body, &e)
		got <- e
	}))
	defer srv.Close()

	n := New(srv.URL, "s3cret", slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go n.Run(ctx)
	n.Send(Event{Type: "game.finished", Code: "ABCD"})

	select {
	case e := <-got:
		if e.Type != "game.finished" || e.Code != "ABCD" {
			t.Errorf("event = %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("webhook not delivered")
	}
}

func TestNilNotifierIsNoop(t *testing.T) {
	var n *Notifier = New("", "", nil)
	n.Send(Event{Type: "x"}) // must not panic
}
