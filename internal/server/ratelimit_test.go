package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(1, 3) // 1 per second, burst 3
	l.now = func() time.Time { return now }

	for i := range 3 {
		if !l.allow("a") {
			t.Fatalf("burst request %d denied", i)
		}
	}
	if l.allow("a") {
		t.Error("request past burst allowed")
	}
	if !l.allow("b") {
		t.Error("other IPs share a's bucket")
	}

	now = now.Add(time.Second)
	if !l.allow("a") || l.allow("a") {
		t.Error("expected exactly one token after 1s")
	}

	// Idle buckets are swept once they'd be full again.
	now = now.Add(time.Hour)
	l.allow("c")
	if _, ok := l.buckets["a"]; ok {
		t.Error("idle bucket not swept")
	}
}

func TestConnCounter(t *testing.T) {
	c := &connCounter{max: 2, n: map[string]int{}}
	if !c.acquire("a") || !c.acquire("a") || c.acquire("a") {
		t.Fatal("cap of 2 not enforced")
	}
	c.release("a")
	if !c.acquire("a") {
		t.Error("slot not freed on release")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:5555"
	r.Header.Set("Fly-Client-IP", "203.0.113.7")

	if ip := (&Server{}).clientIP(r); ip != "10.0.0.1" {
		t.Errorf("untrusted proxy: got %s", ip)
	}
	if ip := (&Server{trustProxy: true}).clientIP(r); ip != "203.0.113.7" {
		t.Errorf("trusted proxy: got %s", ip)
	}
}
