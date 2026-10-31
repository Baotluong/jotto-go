# Jotto

Online two-player Jotto: each player picks a secret word, then you take turns
guessing each other's. Every guess tells you how many letters it shares with
your opponent's word — not which ones. First to crack it wins.

No accounts. Create a game, send the four-letter code to a friend, play.

This is the latest in a line of rewrites:
[2016](https://github.com/Baotluong/Jotto),
[2016](https://github.com/Baotluong/jotto2),
[2019 (React)](https://github.com/Baotluong/Jotto3),
[2023](https://github.com/Baotluong/jotto4).
This version adds real-time online multiplayer with a Go backend.

## Rules and settings

| Setting | Options | Default |
| --- | --- | --- |
| Word length | 4, 5, 6 | 5 |
| Difficulty | Easy, Medium, Hard | Medium |
| Feedback | **Jotto** (letters in common), **+ Positions** (also how many are in the right spot), **Wordle** (every letter marked) | Jotto |
| No repeated letters | on / off (applies to secret words) | on |
| Pick secrets for us | on / off | off |

- Guesses must be real words of the right length. They may repeat letters even
  when secrets can't.
- A typed secret must be at the chosen difficulty or easier, so nobody gets
  stumped by an obscure word. Auto-picked secrets come from exactly that tier.
- With repeated letters, a letter counts as many times as it appears in both
  words: `APPLE` vs `PAPER` share 4.
- **Fair finish:** if the player who moved first solves it, the other player
  gets one last guess. Solving it then is a draw.

## Architecture

```
 Browser (React)                         Go server (single binary)
 ───────────────                         ─────────────────────────
 POST /api/games ──────────────────────▶ hub.Create ─┐
 POST /api/games/{code}/join ──────────▶ hub.Join   ─┤
                                                     ├─▶ game (pure rules) ─▶ store (SQLite)
 WS /api/games/{code}/ws ◀────────────▶ hub.Handle ─┘          │
   ▲  {"type":"guess","word":"crane"}                          └─▶ webhook (signed, async)
   └─ {"type":"state","state":{...}}   full per-player snapshot after every change
```

- **`internal/game`** — the rules engine. Pure functions and a state machine
  with no I/O, so every rule is a table-driven unit test.
- **`internal/words`** — word banks generated from [SCOWL](http://wordlist.aspell.net/)
  and embedded in the binary. SCOWL's size levels map to difficulty tiers
  (easy = levels 10–20, medium = 35, hard = 40–50). Guesses are checked
  against a wider dictionary (up to level 80). Plurals, past tenses and
  offensive words are excluded from secrets. Regenerate with
  `scripts/build-words.sh`.
- **`internal/hub`** — loaded games ("rooms") keyed by code, each behind its
  own mutex. Rooms are reference-counted by connected clients and in-flight
  requests, and dropped from memory when the count reaches zero.
- **`internal/store`** — SQLite via `modernc.org/sqlite` (pure Go, no cgo).
  Every move is persisted before it's broadcast, so a restart or redeploy
  loses nothing; idle games are deleted after 7 days.
- **`internal/server`** — `net/http` routes, WebSockets via `coder/websocket`,
  and the SPA served from `embed`.
- **`internal/webhook`** — optional `game.created` / `game.started` /
  `game.finished` events, POSTed to `WEBHOOK_URL` and signed with HMAC-SHA256
  in `X-Jotto-Signature`. Events are queued, delivered off the request path,
  and retried with backoff.

### Design decisions

- **Identity without accounts.** Creating or joining a game returns a random
  128-bit token, kept in `localStorage`. The token is sent as the first
  WebSocket message, not in the URL, so it stays out of logs, and it's checked
  with a constant-time compare.
- **Full snapshots, not diffs.** The server pushes each player a complete
  view after every change. Payloads are tiny, and reconnecting needs no replay
  logic: the first message after reconnecting *is* the current state. The view
  never includes the opponent's secret until the game is over.
- **Server-authoritative.** Scoring and validation happen only on the server,
  so a modified client can't peek or cheat.
- **One instance, on purpose.** All players of a game must reach the same
  process. A single small machine handles that with no Redis or pub/sub,
  and SQLite on a volume costs nothing extra. Scaling out would mean routing
  by game code (e.g. Fly's `fly-replay`) or adding pub/sub between instances.

## Development

Requires Go 1.26+ and Node 24+.

```bash
go run ./cmd/server            # API on :8080
cd web && npm install && npm run dev   # app on :5173, proxies /api to :8080
```

```bash
go test -race ./...
```

To play against yourself locally, open one tab on `localhost` and the other
on `127.0.0.1`. They're different origins, so each gets its own token.

| Env var | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | listen port |
| `DB_PATH` | `jotto.db` | SQLite file |
| `WEBHOOK_URL` | — | enable webhooks |
| `WEBHOOK_SECRET` | — | HMAC key for webhook signatures |
| `ALLOWED_ORIGINS` | — | extra WebSocket origins, comma-separated |

## Deploying to Fly.io

```bash
fly launch --no-deploy          # pick an app name; keep the existing fly.toml
fly volumes create jotto_data --size 1
fly deploy
```

One `shared-cpu-1x` machine with 256 MB plus a 1 GB volume runs a few dollars
a month. CI deploys on every push to `main` once a `FLY_API_TOKEN` repo secret
exists (`fly tokens create deploy`).

## Mobile

The client speaks plain JSON over REST and WebSockets, so any client can play.
The cheapest path to app stores is wrapping this web app with Capacitor. A
React Native client could reuse `api.ts` and `useGame.ts` almost as-is.

## Credits

Word lists derived from SCOWL © Kevin Atkinson. See [NOTICE](NOTICE).
