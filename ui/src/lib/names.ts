/**
 * The one place a role's name is decided. A role reads as its display name,
 * and as its key made readable when it has none. The name parts in
 * components/names/* and every screen that shows a role go through here; the
 * guard in lib/__tests__/names-one-path.test.ts fails on an inline
 * `display_name || humanizeKey(key)` anywhere else.
 */

/** A snake/kebab/lower role key made readable: `laser_trained` -> "Laser Trained". */
export function humanizeKey(key: string): string {
  if (!key) return "";
  return key
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * The one phrase for a person whose name is not known (one-path W2d): "this
 * person" in a sentence, "This person" opening one. Never "They", "Somebody"
 * or an id. A person who acted is the same person, so the same phrase.
 */
export const UNKNOWN_PERSON = "this person";

/** A person's name, or {@link UNKNOWN_PERSON}; `start` capitalises it to open a sentence. */
export function personOr(name: string | null | undefined, start = false): string {
  const shown = name?.trim();
  if (shown) return shown;
  return start ? "This person" : UNKNOWN_PERSON;
}

/** A role's name: its display name, else its key made readable. */
export function roleName(displayName: string | null | undefined, key: string): string {
  return displayName || humanizeKey(key);
}
