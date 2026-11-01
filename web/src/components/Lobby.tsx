import { useState } from "react";
import type { GameState } from "../api";
import { SettingsSummary } from "./SettingsSummary";

// Desktop browsers support navigator.share too, but there people expect a
// plain copy, so the share sheet is offered only on touch devices.
const canShare = !!navigator.share && matchMedia("(pointer: coarse)").matches;

export function Lobby({ state }: { state: GameState }) {
  const [copied, setCopied] = useState(false);
  const [showLink, setShowLink] = useState(false);
  const link = `${location.origin}/g/${state.code}`;

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
    } catch {
      // Clipboard access can be blocked; show the link to copy by hand.
      setShowLink(true);
      return;
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  function share() {
    navigator.share({ title: "Play Jotto with me", url: link }).catch(() => {});
  }

  return (
    <section className="card stack center">
      <p className="muted">Waiting for your opponent. Send them this code:</p>
      <p className="big-code">{state.code}</p>
      <div className="row">
        <button className="primary" onClick={copy}>
          {copied ? "Copied!" : "Copy invite link"}
        </button>
        {canShare && <button onClick={share}>Share…</button>}
      </div>
      {showLink && (
        <input className="link-input" readOnly value={link} autoFocus onFocus={(e) => e.target.select()} />
      )}
      <SettingsSummary settings={state.settings} />
    </section>
  );
}
