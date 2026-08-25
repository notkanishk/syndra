/**
 * A value that can be read exactly once (§17).
 *
 * TanStack Query keeps a mutation's VARIABLES in the MutationCache for the life
 * of the mutation observer — that is how `mutation.variables` works on a retry
 * and in devtools. Two mutations in this app pass a secret as that variable: a
 * member's storage password, and the delete-capable elevated credential the
 * dormant sweep needs. Both had docblocks promising the value was kept nowhere,
 * and both left it sitting in the cache after the request finished.
 *
 * Calling `reset()` afterwards is not the fix — it would also discard the
 * outcome the screen is still rendering. So the secret is not the variable: a
 * box is. The mutation function takes it out on the way past, and what the
 * cache retains is an empty box.
 *
 * It is not a security boundary against anything running in the page — nothing
 * in a browser can be. It closes the gap between what the code says it keeps
 * and what it keeps, which is the gap that makes an audit of the former
 * worthless.
 */
export interface OneShotSecret {
  /** Reads and destroys. A second call throws rather than returning a stale value. */
  take(): string;
  /** For tests and assertions: whether anything is still in here. */
  readonly spent: boolean;
}

export function oneShot(value: string): OneShotSecret {
  let held: string | null = value;
  return {
    take() {
      if (held === null) {
        // A retry that re-read the box would send an empty password, and the
        // target would answer about the value rather than about the mistake.
        // Louder is better: the caller has to build a new box, which means
        // asking the person again.
        throw new Error("This value has already been sent. Enter it again to retry.");
      }
      const out = held;
      held = null;
      return out;
    },
    get spent() {
      return held === null;
    },
  };
}
