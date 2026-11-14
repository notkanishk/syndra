import { displayLocale } from "@/lib/locale";

/**
 * Names in the order a person reads them: case and accents aside, "Lab 2"
 * before "Lab 10". Every dropdown or picker of names, and every list of places
 * looked up by name, is alphabetical by its shown name; any other list opens in
 * its most logical order (soonest due, most recent) with sortable headings
 * (owner, 13 Nov 2026).
 */
/**
 * In the deployment's locale (an order is a display: in sv, å sorts after z),
 * the same one the backend's textsort collates in. One collator per locale,
 * rebuilt only when the locale changes (it is learned once, at the first read).
 */
let collator: { locale: string | undefined; by: Intl.Collator } | undefined;
const collatorFor = (): Intl.Collator => {
  const locale = displayLocale();
  if (collator?.locale !== locale || !collator) collator = { locale, by: new Intl.Collator(displayLocale(), { sensitivity: "base", numeric: true }) };
  return collator.by;
};

export const compareNames = (a: string, b: string): number => collatorFor().compare(a, b);

/**
 * A copy of `items` alphabetical by `name`. Equal names fall back to `id`
 * (an item's own `id` unless told otherwise), so two "Lab" entries never swap
 * between renders.
 */
export function sortByName<T>(
  items: readonly T[],
  name: (item: T) => string,
  id: (item: T) => string = (item) => String((item as { id?: unknown }).id ?? ""),
): T[] {
  return [...items].sort((a, b) => compareNames(name(a), name(b)) || (id(a) < id(b) ? -1 : id(a) > id(b) ? 1 : 0));
}

/** Strings alphabetical (a picker of plain names: groups, systems, role keys). */
export const sortNames = (names: readonly string[]): string[] => [...names].sort(compareNames);

/**
 * Accounts for a picker: people A to Z, then service accounts A to Z — a
 * service account is not a person, so it never sits among them.
 */
export function sortAccounts<T extends { name: string; service_account?: boolean }>(accounts: readonly T[]): T[] {
  const sorted = sortByName(accounts, (a) => a.name);
  return [...sorted.filter((a) => !a.service_account), ...sorted.filter((a) => a.service_account)];
}
