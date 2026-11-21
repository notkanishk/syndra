import { readOf, type Answered, type WireRead } from "@/lib/queries/read";

/**
 * Fixtures written in the backend's own shape, turned into what a hook hands a
 * component by the hooks' own builders, so a mocked hook answers exactly what
 * the real one would. A fixture with no `read` is a read taken now. Nothing
 * here imports a hook's module, so a `vi.mock` factory may import it; the
 * health route's builder is in `wireHealth.ts`.
 */
export const readNow = (at = new Date().toISOString()): WireRead => ({ state: "now", taken_at: at });

/** A system with nothing to give (503 TARGET_NOT_READ, or a composite `read: none`). */
export const NONE: WireRead = { state: "none", reason: "addon_unreachable", cause: "no_answer", retry_after: 30 };

/** A reply's body as the `Read` its hook answers. */
export function asRead<T extends object>(body: T): Answered<T> {
  return readOf((body as { read?: WireRead }).read ?? readNow(), body);
}
