import { useState } from "react";
import type { GameState } from "../api";
import { ForfeitButton } from "./ForfeitButton";
import { SettingsSummary } from "./SettingsSummary";

interface Props {
  state: GameState;
  error: string | null;
  send: (type: "secret" | "forfeit", word?: string) => void;
}

export function SecretPicker({ state, error, send }: Props) {
  const [word, setWord] = useState("");
  const opponent = state.players[1 - state.you];
  const n = state.settings.length;

  if (state.yourSecret) {
    return (
      <section className="card stack center">
        <p>
          Your secret word is <strong className="secret">{state.yourSecret.toUpperCase()}</strong>
        </p>
        <p className="muted">Waiting for {opponent.name} to choose theirs…</p>
        <ForfeitButton onForfeit={() => send("forfeit")} />
      </section>
    );
  }

  return (
    <form
      className="card stack"
      onSubmit={(e) => {
        e.preventDefault();
        send("secret", word);
      }}
    >
      <h2>Choose your secret word</h2>
      <SettingsSummary settings={state.settings} />
      <input
        className="word-input"
        value={word}
        maxLength={n}
        placeholder={"•".repeat(n)}
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        autoFocus
        onChange={(e) => setWord(e.target.value.replace(/[^a-z]/gi, "").toLowerCase())}
      />
      {error && <p className="error">{error}</p>}
      <div className="row">
        <button className="primary grow" disabled={word.length !== n}>
          Lock it in
        </button>
        <button type="button" onClick={() => send("secret", "")}>
          Pick for me
        </button>
      </div>
      <div className="row spread">
        <p className="muted small">{opponent.ready ? `${opponent.name} is ready.` : `${opponent.name} is choosing…`}</p>
        <ForfeitButton onForfeit={() => send("forfeit")} />
      </div>
    </form>
  );
}
