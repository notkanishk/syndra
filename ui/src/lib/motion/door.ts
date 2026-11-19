import { firstTime } from "@/lib/memory";

/**
 * The first arrival of a sign-in (A-24 §3.2, Q4): the door's light opens into
 * Home once, straight after signing in, and never again until the next one.
 *
 * A sign-in is its own instant (`signedInAt`, set when the door hands over).
 * The first page this browser paints after it takes the arrival, whichever
 * page that is: the shell notes it on every page (`noteSignIn`), so a sign-in
 * that lands on a person's page spends it there, and Home later is quiet.
 * This device remembers the sign-in it has opened for (`firstTime`), so a
 * reload, a second tab or a later visit never replays it; a device that can't
 * remember shows nothing extra.
 */
const OPENED = "syndra:door-opened";
let claim: { at: string; path: string; open: boolean } | null = null;

function claimFor(signedInAt: string) {
  if (claim?.at !== signedInAt) {
    claim = { at: signedInAt, path: window.location.pathname, open: firstTime("local", OPENED, signedInAt, 1) };
  }
  return claim;
}

/** The shell, on every page's first paint: the sign-in's first page is this one, if none was. */
export function noteSignIn(signedInAt: string | undefined) {
  if (signedInAt && typeof window !== "undefined") claimFor(signedInAt);
}

/**
 * Home asks once, as it mounts: true only when this page is the sign-in's
 * first on this device, and only the first time it is asked.
 */
export function takeFirstArrival(signedInAt: string | undefined): boolean {
  if (!signedInAt || typeof window === "undefined") return false;
  const sign = claimFor(signedInAt);
  const yes = sign.open && sign.path === window.location.pathname;
  sign.open = false;
  return yes;
}

/** Test-only: a fresh page load. */
export function resetDoorForTests() {
  claim = null;
}

/**
 * When the blocks begin to arrive after the door opens on Home (A-24 §3.2,
 * frame 7): `--door-blocks` in globals.css, mirrored here for the head's
 * settle timer, and a test holds the two equal.
 */
export const DOOR_BLOCKS_MS = 1160;
