package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Abuse limits. The server is one fixed-size machine, so a flood can't run
// up the bill, but it could fill the database or crowd out real players.
const (
	// Creating and joining games: a burst of 10, then one every 6 seconds.
	gameRate, gameBurst = 1.0 / 6, 10
	// WebSocket connection attempts, which include reconnects.
	connectRate, connectBurst = 1.0, 20
	// Open WebSockets per IP. Generous, since a household shares one IP.
	maxConnsPerIP = 20
	// Messages on one WebSocket. Real play is a few per minute.
	messageRate, messageBurst = 2.0, 10
)

// bucket is a token bucket: it holds up to burst tokens, refills at rate
// tokens per second, and each allowed event spends one.
type bucket struct {
	tokens float64
	last   time.Time
}

func (b *bucket) allow(rate, burst float64, now time.Time) bool {
	if b.last.IsZero() {
		b.tokens = burst
	} else {
		b.tokens = min(burst, b.tokens+now.Sub(b.last).Seconds()*rate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// limiter keeps one bucket per key (client IP).
type limiter struct {
	rate, burst float64
	now         func() time.Time

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

func newLimiter(rate, burst float64) *limiter {
	return &limiter{rate: rate, burst: burst, now: time.Now, buckets: map[string]*bucket{}}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{}
		l.buckets[key] = b
	}
	return b.allow(l.rate, l.burst, now)
}

// sweep drops buckets that have refilled completely; they'd behave the same
// as a fresh one. Runs at most once a minute so allow stays cheap.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	full := time.Duration(l.burst / l.rate * float64(time.Second))
	for k, b := range l.buckets {
		if now.Sub(b.last) >= full {
			delete(l.buckets, k)
		}
	}
}

// connCounter caps concurrent connections per key.
type connCounter struct {
	max int

	mu sync.Mutex
	n  map[string]int
}

func (c *connCounter) acquire(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n[key] >= c.max {
		return false
	}
	c.n[key]++
	return true
}

func (c *connCounter) release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n[key]--; c.n[key] <= 0 {
		delete(c.n, key)
	}
}

// limit wraps a handler with a per-IP rate limit.
func (s *Server) limit(l *limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(s.clientIP(r)) {
			tooMany(w)
			return
		}
		next(w, r)
	}
}

func tooMany(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests. Try again in a minute."})
}

// clientIP is the caller's address. Behind Fly's proxy every request comes
// from the proxy, so the real client is in Fly-Client-IP, which the proxy
// sets itself. Elsewhere that header could be forged, so it's only trusted
// when trustProxy is set.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
