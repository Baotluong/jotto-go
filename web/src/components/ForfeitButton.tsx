import { useEffect, useState } from "react";

/** Two-step forfeit: the first tap arms it, the second confirms. */
export function ForfeitButton({ onForfeit, label = "Forfeit" }: { onForfeit: () => void; label?: string }) {
  const [armed, setArmed] = useState(false);

  // Disarm after a few seconds so a stray tap later can't end the game.
  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), 4000);
    return () => clearTimeout(t);
  }, [armed]);

  return (
    <button
      type="button"
      className={`forfeit ${armed ? "armed" : ""}`}
      onClick={() => (armed ? onForfeit() : setArmed(true))}
    >
      {armed ? `Tap again to ${label.toLowerCase()}` : label}
    </button>
  );
}
