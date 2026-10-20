import type { UiView } from "@/lib/nav";

/**
 * The stored Basic/Advanced choice lives in a cookie so the SERVER can render
 * it. A view read from localStorage after mount means every full page load
 * paints Basic until the bundle hydrates, then slides to Advanced; with the
 * cookie the first HTML is already the right one and hydration agrees with it.
 * A layout preference, not a credential, so it is plain and script-readable.
 *
 * Its own module, not `ui-view.tsx`: that file is a client module, and a server
 * component (the root layout) cannot call a function imported from one.
 */
export const VIEW_COOKIE = "syndra_ui_view";

/** The stored view if it is one of the two, else undefined (never a guess). */
export function parseUiView(value: string | null | undefined): UiView | undefined {
  return value === "basic" || value === "advanced" ? value : undefined;
}

export function writeViewCookie(view: UiView) {
  document.cookie = `${VIEW_COOKIE}=${view}; path=/; max-age=31536000; samesite=lax`;
}
