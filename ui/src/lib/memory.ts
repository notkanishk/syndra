/**
 * This device's memory: one guarded read and write over sessionStorage (this
 * tab, until it closes) and localStorage (this browser). A blocked or full
 * store, a private window or the server is a page that simply forgets.
 *
 * What lives here only ever makes a place easier to find again: where you
 * stood, a list's view, the places that have arrived this session, Go to's
 * Lately. None of it is read for anything about access, and nothing personal
 * goes in beyond a place's path.
 */
export type Memory = "session" | "local";

function store(kind: Memory): Storage | null {
  try {
    return kind === "session" ? window.sessionStorage : window.localStorage;
  } catch {
    return null;
  }
}

export function recall(kind: Memory, key: string): unknown {
  try {
    const raw = store(kind)?.getItem(key);
    return raw == null ? undefined : JSON.parse(raw);
  } catch {
    return undefined;
  }
}

export function remember(kind: Memory, key: string, value: unknown) {
  try {
    store(kind)?.setItem(key, JSON.stringify(value));
  } catch {
    // Forgetting is the whole failure mode.
  }
}

/** A remembered list of strings, or an empty one when what is there is anything else. */
export function recallPaths(kind: Memory, key: string): string[] {
  const kept = recall(kind, key);
  return Array.isArray(kept) ? kept.filter((item): item is string => typeof item === "string") : [];
}
