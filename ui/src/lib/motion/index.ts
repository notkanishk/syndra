/**
 * The one motion module (A-19 P6). Every verb the product moves with lives in
 * this folder, each beside its still form:
 *
 *   still      reduced motion, the exit phase and the token durations
 *   arrive     a list's rows rise in on its first paint (and its memory of Back)
 *   unfold     a region opens to its height and closes by fading first
 *   leave      a row that goes on a later read fades, then its gap closes
 *   crossfade  one seat whose words change in place
 *   glide      the scroll to a section, then the landing flash; the indicator
 *   flash      a value that changed while you were looking elsewhere
 *
 * bloom and lift are CSS classes (`.bloom`, `.lift-in`) in globals.css; their
 * Moves lines are in `moves.ts`, which also feeds the design-system cards.
 */
export * from "./still";
export * from "./glide";
export * from "./glide-indicator";
export * from "./arrive";
export * from "./unfold";
export * from "./leave";
export * from "./crossfade";
export * from "./flash";
