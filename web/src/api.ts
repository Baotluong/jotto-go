// Types mirror the Go JSON in internal/game and internal/hub.

export type Feedback = "classic" | "positional" | "wordle";
export type Difficulty = 1 | 2 | 3;
export type Phase = "waiting" | "choosing" | "playing" | "finished";
export type Mark = "hit" | "present" | "miss";

export interface Settings {
  length: 4 | 5 | 6;
  uniqueLetters: boolean;
  difficulty: Difficulty;
  feedback: Feedback;
  autoSecrets: boolean;
}

export const defaultSettings: Settings = {
  length: 5,
  uniqueLetters: true,
  difficulty: 2,
  feedback: "classic",
  autoSecrets: false,
};

export interface Result {
  common: number;
  exact?: number;
  marks?: Mark[];
  solved: boolean;
}

export interface Guess {
  seat: number;
  word: string;
  result: Result;
  at: string;
}

export interface PlayerView {
  name: string;
  joined: boolean;
  ready: boolean;
  connected: boolean;
}

export interface GameState {
  code: string;
  settings: Settings;
  phase: Phase;
  you: number;
  turn: number;
  lastChance: boolean;
  players: [PlayerView, PlayerView];
  yourSecret: string;
  opponentSecret?: string;
  guesses: Guess[];
  winner: number; // seat, 2 = draw, -1 = none
}

export interface Joined {
  code: string;
  token: string;
  seat: number;
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(data.error ?? "Request failed", res.status);
  return data as T;
}

export const createGame = (name: string, settings: Settings) =>
  post<Joined>("/api/games", { name, settings });

export const joinGame = (code: string, name: string) =>
  post<Joined>(`/api/games/${encodeURIComponent(code)}/join`, { name });

// There are no accounts: the per-game token in localStorage is your identity.
// Storage can be unavailable (private mode), so every access is guarded.
const tokenKey = (code: string) => `jotto:token:${code}`;

export function saveToken(j: Joined) {
  try {
    localStorage.setItem(tokenKey(j.code), j.token);
  } catch {
    /* ignore */
  }
}

export function loadToken(code: string): string | null {
  try {
    return localStorage.getItem(tokenKey(code));
  } catch {
    return null;
  }
}

export function loadName(): string {
  try {
    return localStorage.getItem("jotto:name") ?? "";
  } catch {
    return "";
  }
}

export function saveName(name: string) {
  try {
    localStorage.setItem("jotto:name", name);
  } catch {
    /* ignore */
  }
}
