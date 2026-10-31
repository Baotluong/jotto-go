import { useEffect, useState } from "react";

type LetterState = "unknown" | "out" | "in";
const next: Record<LetterState, LetterState> = { unknown: "out", out: "in", in: "unknown" };
const ROWS = ["qwertyuiop", "asdfghjkl", "zxcvbnm"];

/**
 * A scratchpad for deduction: tap a letter to mark it ruled out, then
 * confirmed, then back. Purely client-side and saved per game, so it
 * survives a refresh.
 */
export function LetterBoard({ code }: { code: string }) {
  const key = `jotto:letters:${code}`;
  const [marks, setMarks] = useState<Record<string, LetterState>>(() => {
    try {
      return JSON.parse(localStorage.getItem(key) ?? "{}");
    } catch {
      return {};
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify(marks));
    } catch {
      /* ignore */
    }
  }, [key, marks]);

  return (
    <section className="card">
      <h3>
        Notes <span className="muted small">tap: ruled out → confirmed → clear</span>
      </h3>
      <div className="keyboard">
        {ROWS.map((row) => (
          <div className="key-row" key={row}>
            {[...row].map((c) => {
              const s = marks[c] ?? "unknown";
              return (
                <button
                  key={c}
                  type="button"
                  className={`key ${s}`}
                  aria-label={`${c}, ${s}`}
                  onClick={() => setMarks((m) => ({ ...m, [c]: next[s] }))}
                >
                  {c.toUpperCase()}
                </button>
              );
            })}
          </div>
        ))}
      </div>
    </section>
  );
}
