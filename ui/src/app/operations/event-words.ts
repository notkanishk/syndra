/**
 * The page's own words for the two streams it merges.
 *
 * Not `audit-vocabulary`: that maps what an OPERATOR did in Syndra (`bundle.assigned`),
 * and these are what reached Syndra from outside (`grant_added` from Zitadel's webhook, an
 * onboarding trigger). Different keys, different sentences, and a shared table would be
 * two vocabularies pretending to be one.
 *
 * Keys are the backend's `validEventTypes`. An event with no entry here gets a plain
 * title, never its key — the key stays in mono beneath the title for whoever is matching
 * it against Zitadel, but a reader should never have to decode it to know what happened.
 */
const ZITADEL_TITLE: Record<string, string> = {
  grant_added: "Access given in Zitadel",
  grant_removed: "Access removed in Zitadel",
  grant_changed: "Access changed in Zitadel",
  user_created: "Person added in Zitadel",
  user_deactivated: "Person deactivated in Zitadel",
  user_locked: "Person locked in Zitadel",
};

export const ONBOARDING_TITLE = "Person joined";

export function zitadelTitle(eventType: string): string {
  return ZITADEL_TITLE[eventType] ?? "Another event from Zitadel";
}
