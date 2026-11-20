import { firstTime, recallPaths } from "@/lib/memory";

/**
 * The first arrival of a sign-in (A-24 §3.2, Q4): Home's light glides into its
 * place once, straight after signing in, and never again until the next one.
 *
 * Decided before the first paint, so the light never rests and then starts
 * over. The sign-in (the OIDC callback, the dev identity picker) sets a
 * short-lived, readable cookie naming the sign-in's instant; the server's
 * render of Home reads it and marks the light in the first HTML.
 * The first page that mounts after the sign-in, whichever it is, spends it
 * (the shell deletes it), so a reload, a second tab or a later visit to Home
 * never replays it, and a sign-in that lands on another page spends its door
 * there. It names an instant only: no secret, no identity.
 *
 * ponytail: if the cookie were ever still there for a sign-in this device has
 * already opened for (two tabs mounting at once), the door plays once more
 * rather than flashing away; deleting it on the first mount makes that rare.
 */
export const DOOR_COOKIE = "syndra_door";
/** Long enough to cover the redirect and the first page's render, and no longer. */
export const DOOR_COOKIE_MAX_AGE_S = 60;
const OPENED = "syndra:door-opened";

/** The cookie the sign-in sets: the instant, readable by the page, gone in a minute. */
export const doorCookie = (signedInAt: string) =>
  ({ name: DOOR_COOKIE, value: signedInAt, httpOnly: false, sameSite: "lax", path: "/", maxAge: DOOR_COOKIE_MAX_AGE_S }) as const;

/** The server's answer for this render: the sign-in's own door, and only its own. */
export const doorFor = (cookie: string | undefined, signedInAt: string | undefined): boolean =>
  Boolean(cookie && signedInAt && cookie === signedInAt);

/**
 * Whether this device has already opened the door for this sign-in. Read as
 * Home first renders on the client: on the page the server drew the door into
 * it is not yet recorded (the shell spends it after), so the hydrated render
 * matches; on a later mount (Back to a cached Home) it is, and the door stays shut.
 */
export const openedAlready = (signedInAt: string | undefined): boolean =>
  typeof window !== "undefined" && Boolean(signedInAt) && recallPaths("local", OPENED).includes(signedInAt!);

/** On the first mount after a sign-in: the door is spent, and this device remembers it was opened. */
export function spendDoor(signedInAt: string | undefined) {
  if (typeof document === "undefined") return;
  document.cookie = `${DOOR_COOKIE}=; Max-Age=0; Path=/; SameSite=Lax`;
  if (signedInAt) firstTime("local", OPENED, signedInAt, 1);
}
