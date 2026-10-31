// Package webhook sends signed, fire-and-forget event notifications (game
// created, started, finished) to an optional URL, e.g. for a Discord bot or
// an analytics sink. Delivery never blocks gameplay.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Event struct {
	Type    string    `json:"type"`
	Code    string    `json:"code"`
	Winner  *int      `json:"winner,omitempty"`
	Guesses int       `json:"guesses,omitempty"`
	At      time.Time `json:"at"`
}

// SignatureHeader carries hex(HMAC-SHA256(secret, body)) so receivers can
// verify the request came from this server.
const SignatureHeader = "X-Jotto-Signature"

// Notifier delivers events from a bounded queue on a single goroutine.
// A nil *Notifier or one with no URL is a no-op.
type Notifier struct {
	url    string
	secret []byte
	client *http.Client
	queue  chan Event
	log    *slog.Logger
}

func New(url, secret string, log *slog.Logger) *Notifier {
	if url == "" {
		return nil
	}
	return &Notifier{
		url:    url,
		secret: []byte(secret),
		client: &http.Client{Timeout: 5 * time.Second},
		queue:  make(chan Event, 100),
		log:    log,
	}
}

// Send queues an event, dropping it if the queue is full.
func (n *Notifier) Send(e Event) {
	if n == nil {
		return
	}
	e.At = time.Now().UTC()
	select {
	case n.queue <- e:
	default:
		n.log.Warn("webhook queue full, dropping event", "type", e.Type)
	}
}

// Run delivers queued events until ctx is cancelled.
func (n *Notifier) Run(ctx context.Context) {
	if n == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-n.queue:
			n.deliver(ctx, e)
		}
	}
}

// deliver retries with backoff on network errors and 5xx responses.
func (n *Notifier) deliver(ctx context.Context, e Event) {
	body, err := json.Marshal(e)
	if err != nil {
		n.log.Error("webhook marshal", "err", err)
		return
	}
	mac := hmac.New(sha256.New, n.secret)
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		err = n.post(ctx, body, sig)
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	n.log.Warn("webhook delivery failed", "type", e.Type, "err", err)
}

func (n *Notifier) post(ctx context.Context, body []byte, sig string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, sig)
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
