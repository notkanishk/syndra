import { resetMakerspaceZoneForTests, setMakerspaceZone } from "@/lib/time";

/**
 * Run as if the makerspace were configured in `name`. The zone is write-once
 * in the app (one deployment value), so a test forgets it first; the setup set
 * it from TZ for the file, and a test that picks its own puts that back after.
 */
export function inZone(name: string | undefined): void {
  resetMakerspaceZoneForTests();
  setMakerspaceZone(name);
}
