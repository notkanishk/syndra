import { firstTime } from "@/lib/memory";

/**
 * The first arrival of a sign-in (A-24 §3.2, Q4): you walk through the door
 * you signed in at, once, on whatever page the sign-in lands on, and never
 * again until the next sign-in.
 *
 * A sign-in is its own instant (`signedInAt`, stamped where the session is
 * made). The shell asks as it mounts, which is the sign-in's first page. This
 * device remembers the sign-in it opened for (`firstTime`), so a reload, a
 * second tab or a later page never replays it; a device that can't remember
 * shows nothing extra. Only the first ask in a page load can answer yes, so a
 * second run of the effect (React's Strict Mode) can't play it twice.
 */
const OPENED = "syndra:door-opened";
let asked: string | null = null;

export function firstArrival(signedInAt: string | undefined): boolean {
  if (!signedInAt || typeof window === "undefined" || asked === signedInAt) return false;
  asked = signedInAt;
  return firstTime("local", OPENED, signedInAt, 1);
}

/** Test-only: a fresh page load. */
export function resetDoorForTests() {
  asked = null;
}
