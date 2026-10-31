import { useState } from "react";
import type { GameState } from "../api";
import { SettingsSummary } from "./SettingsSummary";

export function Lobby({ state }: { state: GameState }) {
  const [copied, setCopied] = useState(false);
  const link = `${location.origin}/g/${state.code}`;

  async function share() {
    // Native share sheet on phones, clipboard elsewhere.
    if (navigator.share) {
      await navigator.share({ title: "Play Jotto with me", url: link }).catch(() => {});
      return;
    }
    await navigator.clipboard.writeText(link);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <section className="card stack center">
      <p className="muted">Waiting for your opponent. Send them this code:</p>
      <p className="big-code">{state.code}</p>
      <button className="primary" onClick={share}>
        {copied ? "Link copied" : "Share invite link"}
      </button>
      <SettingsSummary settings={state.settings} />
    </section>
  );
}
