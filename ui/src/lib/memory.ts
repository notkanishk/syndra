/**
 * This device's memory: one guarded read and write over sessionStorage (this
 * tab, until it closes) and localStorage (this browser). A blocked or full
 * store, a private window or the server is a page that simply forgets.
 *
 * What lives here only ever makes a place easier to find again, or keeps a
 * moment meant once to once: where you stood, a list's view, the places that
 * have arrived this session, Go to's Lately, the sign-in the door last opened
 * for, the lit cards already seen. None of it is read for anything about
 * access, and nothing personal goes in beyond a place's path, an instant and
 * a person's id as a hash (`idHash`).
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

/** Keeps `value` under `key`; undefined or null forgets it. */
export function remember(kind: Memory, key: string, value: unknown) {
  try {
    if (value == null) store(kind)?.removeItem(key);
    else store(kind)?.setItem(key, JSON.stringify(value));
  } catch {
    // Forgetting is the whole failure mode.
  }
}

/** A remembered list of strings, or an empty one when what is there is anything else. */
export function recallPaths(kind: Memory, key: string): string[] {
  const kept = recall(kind, key);
  return Array.isArray(kept) ? kept.filter((item): item is string => typeof item === "string") : [];
}

/**
 * Whether this is the first time this device has seen `mark` under `key`, and
 * it now remembers it: true once, then false. A store that cannot keep it
 * answers false too, so a moment meant once shows nothing rather than
 * repeating on every visit (A-24 §3.2, §3.3). The newest `keep` marks are kept.
 */
export function firstTime(kind: Memory, key: string, mark: string, keep: number): boolean {
  const seen = recallPaths(kind, key);
  if (seen.includes(mark)) return false;
  remember(kind, key, [mark, ...seen].slice(0, keep));
  return recallPaths(kind, key).includes(mark);
}

/**
 * An id as this device keeps it: a short hash, never the id itself (FNV-1a,
 * 32 bits). Enough to tell two people on a shared computer apart; not a
 * secret and not a lookup.
 */
export function idHash(id: string): string {
  let hash = 0x811c9dc5;
  for (let i = 0; i < id.length; i += 1) hash = Math.imul(hash ^ id.charCodeAt(i), 0x01000193);
  return (hash >>> 0).toString(16).padStart(8, "0");
}
