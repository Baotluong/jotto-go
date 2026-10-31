import { useCallback, useEffect, useRef, useState } from "react";
import type { GameState } from "./api";

export type ConnStatus = "connecting" | "open" | "closed" | "fatal";

// Close codes the server uses for errors that retrying won't fix.
const FATAL_CODES: Record<number, string> = {
  4404: "Game not found. It may have expired.",
  4401: "You're not a player in this game.",
};

/**
 * Keeps a WebSocket open to the game, reconnecting with backoff. The server
 * sends a full state snapshot on every change, so reconnecting needs no
 * replay logic: the first message after connecting is the current state.
 */
export function useGame(code: string, token: string) {
  const [state, setState] = useState<GameState | null>(null);
  const [status, setStatus] = useState<ConnStatus>("connecting");
  const [error, setError] = useState<string | null>(null);
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    let stopped = false;
    let retry = 0;
    let timer: ReturnType<typeof setTimeout>;

    const connect = () => {
      const proto = location.protocol === "https:" ? "wss" : "ws";
      const sock = new WebSocket(`${proto}://${location.host}/api/games/${code}/ws`);
      ws.current = sock;
      setStatus("connecting");

      sock.onopen = () => {
        sock.send(JSON.stringify({ token }));
        retry = 0;
        setStatus("open");
      };
      sock.onmessage = (ev) => {
        const msg = JSON.parse(ev.data);
        if (msg.type === "state") {
          setState(msg.state);
          setError(null);
        } else if (msg.type === "error") {
          setError(msg.error);
        }
      };
      sock.onclose = (ev) => {
        if (stopped) return;
        if (FATAL_CODES[ev.code]) {
          setStatus("fatal");
          setError(FATAL_CODES[ev.code]);
          return;
        }
        setStatus("closed");
        const delay = Math.min(1000 * 2 ** retry, 15000);
        retry++;
        timer = setTimeout(connect, delay);
      };
    };

    connect();
    // Phones suspend sockets in the background; reconnect when we come back.
    const onVisible = () => {
      if (document.visibilityState === "visible" && ws.current?.readyState === WebSocket.CLOSED) {
        clearTimeout(timer);
        connect();
      }
    };
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      stopped = true;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
      ws.current?.close();
    };
  }, [code, token]);

  const send = useCallback((type: "secret" | "guess" | "forfeit", word = "") => {
    setError(null);
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify({ type, word }));
    } else {
      setError("Reconnecting, try again in a moment.");
    }
  }, []);

  return { state, status, error, send, clearError: () => setError(null) };
}
