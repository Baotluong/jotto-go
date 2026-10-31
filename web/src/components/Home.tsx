import { useState } from "react";
import { createGame, defaultSettings, loadName, saveName, saveToken, type Settings } from "../api";
import { navigate } from "../App";

const feedbackHelp: Record<Settings["feedback"], string> = {
  classic: "Only how many letters you got. True Jotto.",
  positional: "Letters in common, plus how many are in the right spot.",
  wordle: "Every letter marked: right spot, wrong spot, or not in the word.",
};

export function Home() {
  const [name, setName] = useState(loadName);
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [joinCode, setJoinCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) => setSettings((s) => ({ ...s, [k]: v }));

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      saveName(name);
      const j = await createGame(name, settings);
      saveToken(j);
      navigate(`/g/${j.code}`);
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  function onJoin(e: React.FormEvent) {
    e.preventDefault();
    const code = joinCode.trim().toUpperCase();
    if (code.length === 4) navigate(`/g/${code}`);
  }

  return (
    <div className="stack">
      <section className="card">
        <h1>Guess your opponent's secret word</h1>
        <p className="muted">
          Each of you picks a secret word. Take turns guessing. Every guess tells you how many letters it shares
          with your opponent's word. First to crack it wins.
        </p>
      </section>

      <form className="card stack" onSubmit={onCreate}>
        <h2>New game</h2>
        <label className="field">
          <span>Your name</span>
          <input value={name} maxLength={20} placeholder="Player 1" onChange={(e) => setName(e.target.value)} />
        </label>

        <fieldset className="field">
          <legend>Word length</legend>
          <div className="segmented">
            {([4, 5, 6] as const).map((n) => (
              <button type="button" key={n} aria-pressed={settings.length === n} onClick={() => set("length", n)}>
                {n}
              </button>
            ))}
          </div>
        </fieldset>

        <fieldset className="field">
          <legend>Word difficulty</legend>
          <div className="segmented">
            {(
              [
                [1, "Easy"],
                [2, "Medium"],
                [3, "Hard"],
              ] as const
            ).map(([d, label]) => (
              <button
                type="button"
                key={d}
                aria-pressed={settings.difficulty === d}
                onClick={() => set("difficulty", d)}
              >
                {label}
              </button>
            ))}
          </div>
          <small className="muted">Secret words must be this common or more.</small>
        </fieldset>

        <fieldset className="field">
          <legend>Feedback</legend>
          <div className="segmented">
            {(["classic", "positional", "wordle"] as const).map((f) => (
              <button type="button" key={f} aria-pressed={settings.feedback === f} onClick={() => set("feedback", f)}>
                {f === "classic" ? "Jotto" : f === "positional" ? "+ Positions" : "Wordle"}
              </button>
            ))}
          </div>
          <small className="muted">{feedbackHelp[settings.feedback]}</small>
        </fieldset>

        <label className="check">
          <input
            type="checkbox"
            checked={settings.uniqueLetters}
            onChange={(e) => set("uniqueLetters", e.target.checked)}
          />
          No repeated letters in secret words
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={settings.autoSecrets}
            onChange={(e) => set("autoSecrets", e.target.checked)}
          />
          Pick secret words for us
        </label>

        {error && <p className="error">{error}</p>}
        <button className="primary" disabled={busy}>
          {busy ? "Creating…" : "Create game"}
        </button>
      </form>

      <form className="card stack" onSubmit={onJoin}>
        <h2>Join a game</h2>
        <div className="row">
          <input
            className="code-input"
            value={joinCode}
            maxLength={4}
            placeholder="CODE"
            autoCapitalize="characters"
            onChange={(e) => setJoinCode(e.target.value)}
          />
          <button disabled={joinCode.trim().length !== 4}>Join</button>
        </div>
      </form>
    </div>
  );
}
