import { useState } from "react";
import { ApiError, joinGame, loadName, loadToken, saveName, saveToken } from "../api";
import { useGame } from "../useGame";
import { Board } from "./Board";
import { Lobby } from "./Lobby";
import { SecretPicker } from "./SecretPicker";

export function GamePage({ code }: { code: string }) {
  const [token, setToken] = useState(() => loadToken(code));
  if (!token) return <JoinForm code={code} onJoined={setToken} />;
  return <ConnectedGame code={code} token={token} />;
}

function JoinForm({ code, onJoined }: { code: string; onJoined: (token: string) => void }) {
  const [name, setName] = useState(loadName);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      saveName(name);
      const j = await joinGame(code, name);
      saveToken(j);
      onJoined(j.token);
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? "This game already has two players." : (err as Error).message);
      setBusy(false);
    }
  }

  return (
    <form className="card stack" onSubmit={onSubmit}>
      <h2>
        Join game <span className="code">{code}</span>
      </h2>
      <label className="field">
        <span>Your name</span>
        <input value={name} maxLength={20} placeholder="Player 2" onChange={(e) => setName(e.target.value)} autoFocus />
      </label>
      {error && <p className="error">{error}</p>}
      <button className="primary" disabled={busy}>
        {busy ? "Joining…" : "Join"}
      </button>
    </form>
  );
}

function ConnectedGame({ code, token }: { code: string; token: string }) {
  const { state, status, error, send } = useGame(code, token);

  if (status === "fatal") return <p className="card error">{error}</p>;
  if (!state) return <p className="card muted">Connecting…</p>;

  return (
    <div className="stack">
      {status !== "open" && <p className="banner">Reconnecting…</p>}
      {state.phase === "waiting" && <Lobby state={state} />}
      {state.phase === "choosing" && <SecretPicker state={state} error={error} send={send} />}
      {(state.phase === "playing" || state.phase === "finished") && <Board state={state} error={error} send={send} />}
    </div>
  );
}
