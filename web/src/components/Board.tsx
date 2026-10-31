import { useEffect, useState } from "react";
import { navigate } from "../App";
import type { GameState, Guess } from "../api";
import { ForfeitButton } from "./ForfeitButton";
import { LetterBoard } from "./LetterBoard";

interface Props {
  state: GameState;
  error: string | null;
  send: (type: "guess" | "forfeit", word?: string) => void;
}

export function Board({ state, error, send }: Props) {
  const [word, setWord] = useState("");
  const me = state.you;
  const them = 1 - me;
  const opponent = state.players[them];
  const myTurn = state.phase === "playing" && state.turn === me;
  const n = state.settings.length;
  const mine = state.guesses.filter((g) => g.seat === me);
  const theirs = state.guesses.filter((g) => g.seat === them);

  // Clear the input only once a guess is accepted, so a rejected one
  // (typo, repeat) can be fixed instead of retyped.
  useEffect(() => setWord(""), [mine.length]);

  return (
    <div className="stack">
      <Status state={state} />

      {state.phase === "playing" && (
        <form
          className="card row"
          onSubmit={(e) => {
            e.preventDefault();
            send("guess", word);
          }}
        >
          <input
            className="word-input grow"
            value={word}
            maxLength={n}
            placeholder={myTurn ? "Your guess" : `${opponent.name} is guessing…`}
            disabled={!myTurn}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            onChange={(e) => setWord(e.target.value.replace(/[^a-z]/gi, "").toLowerCase())}
          />
          <button className="primary" disabled={!myTurn || word.length !== n}>
            Guess
          </button>
        </form>
      )}
      {error && <p className="error">{error}</p>}

      <div className="columns">
        <section className="card">
          <h3>Your guesses</h3>
          <GuessList guesses={mine} length={n} />
        </section>
        <section className="card">
          <h3>
            {opponent.name}'s guesses
            {!opponent.connected && <span className="muted small"> (offline)</span>}
          </h3>
          <GuessList guesses={theirs} length={n} />
          <p className="muted small">
            Your word: <strong className="secret">{state.yourSecret.toUpperCase()}</strong>
          </p>
        </section>
      </div>

      <LetterBoard code={state.code} />

      {state.phase === "playing" && (
        <div className="row end">
          <ForfeitButton onForfeit={() => send("forfeit")} />
        </div>
      )}
      {state.phase === "finished" && (
        <button className="primary" onClick={() => navigate("/")}>
          New game
        </button>
      )}
    </div>
  );
}

function Status({ state }: { state: GameState }) {
  const me = state.you;
  const opponent = state.players[1 - me];
  let text: string;
  let tone = "";
  if (state.phase === "finished") {
    const reveal = state.opponentSecret ? ` Their word was ${state.opponentSecret.toUpperCase()}.` : "";
    if (state.forfeitedBy === me) {
      text = "You forfeited." + reveal;
      tone = "lose";
    } else if (state.forfeitedBy === 1 - me) {
      text = `${opponent.name} forfeited. You win!` + reveal;
      tone = "win";
    } else if (state.winner === 2) {
      text = "Draw! You both cracked it." + reveal;
    } else if (state.winner === me) {
      text = "You win!" + reveal;
      tone = "win";
    } else {
      text = `${opponent.name} wins.` + reveal;
      tone = "lose";
    }
  } else if (state.lastChance) {
    text =
      state.turn === me
        ? `${opponent.name} solved it! Last chance: guess right to tie.`
        : `You solved it! ${opponent.name} gets one last guess to tie.`;
    tone = "alert";
  } else {
    text = state.turn === me ? "Your turn" : `${opponent.name}'s turn`;
    tone = state.turn === me ? "active" : "";
  }
  return <p className={`status ${tone}`}>{text}</p>;
}

function GuessList({ guesses, length }: { guesses: Guess[]; length: number }) {
  if (guesses.length === 0) return <p className="muted small">No guesses yet.</p>;
  return (
    <ol className="guesses">
      {guesses.map((g, i) => (
        <li key={i}>
          <Tiles guess={g} length={length} />
          <Score guess={g} />
        </li>
      ))}
    </ol>
  );
}

function Tiles({ guess, length }: { guess: Guess; length: number }) {
  const marks = guess.result.marks;
  return (
    <span className="tiles" style={{ "--n": length } as React.CSSProperties}>
      {[...guess.word].map((c, i) => (
        <span key={i} className={`tile ${marks ? marks[i] : ""}`}>
          {c.toUpperCase()}
        </span>
      ))}
    </span>
  );
}

function Score({ guess }: { guess: Guess }) {
  const r = guess.result;
  if (r.solved) return <span className="score solved">✓</span>;
  if (r.marks) return null; // tiles already say everything
  return (
    <span className="badge" title={`${r.common} letters in common`}>
      {r.common}
    </span>
  );
}
