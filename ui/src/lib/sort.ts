/**
 * Names in the order a person reads them: case and accents aside, "Lab 2"
 * before "Lab 10". Every list or picker of names is alphabetical by its shown
 * name (RULES.md); lists whose order carries meaning (send order, time,
 * relevance) keep theirs.
 */
const collator = new Intl.Collator(undefined, { sensitivity: "base", numeric: true });

export const compareNames = (a: string, b: string): number => collator.compare(a, b);

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
