import { knownMakerspaceZone, setMakerspaceZone } from "@/lib/time";

/**
 * Offer the zone just read (or undefined when the read failed) and return the
 * zone this process actually holds: the one the server renders with, and so the
 * one the client must be given. A failed read returns what is already held.
 */
export function effectiveZone(read: string | undefined): string | undefined {
  setMakerspaceZone(read);
  return knownMakerspaceZone();
}
