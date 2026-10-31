CREATE TABLE IF NOT EXISTS games (
    code       TEXT PRIMARY KEY,
    settings   TEXT    NOT NULL, -- JSON game.Settings
    phase      TEXT    NOT NULL,
    first_seat INTEGER NOT NULL,
    turn       INTEGER NOT NULL,
    solved0    INTEGER NOT NULL DEFAULT 0,
    solved1    INTEGER NOT NULL DEFAULT 0,
    winner     INTEGER NOT NULL,
    created_at INTEGER NOT NULL, -- unix millis
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS games_updated_at ON games (updated_at);

CREATE TABLE IF NOT EXISTS players (
    game_code TEXT    NOT NULL REFERENCES games (code) ON DELETE CASCADE,
    seat      INTEGER NOT NULL,
    token     TEXT    NOT NULL,
    name      TEXT    NOT NULL,
    secret    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (game_code, seat)
);

CREATE TABLE IF NOT EXISTS guesses (
    game_code TEXT    NOT NULL REFERENCES games (code) ON DELETE CASCADE,
    seq       INTEGER NOT NULL,
    seat      INTEGER NOT NULL,
    word      TEXT    NOT NULL,
    result    TEXT    NOT NULL, -- JSON game.Result
    at        INTEGER NOT NULL,
    PRIMARY KEY (game_code, seq)
);
