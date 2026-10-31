import type { Settings } from "../api";

const difficulty = { 1: "Easy", 2: "Medium", 3: "Hard" };
const feedback = { classic: "Jotto", positional: "Jotto + positions", wordle: "Wordle" };

export function SettingsSummary({ settings: s }: { settings: Settings }) {
  const parts = [
    `${s.length} letters`,
    difficulty[s.difficulty],
    feedback[s.feedback],
    s.uniqueLetters ? "no repeats" : "repeats allowed",
  ];
  return <p className="muted small">{parts.join(" · ")}</p>;
}
