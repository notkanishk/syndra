import { useState } from "react";

/**
 * Pending changes' one narrowing model (A-20 P16). `?q=` is the search and
 * stays the page's own; `?person=` and `?role=` are filters in force, each
 * shown as one FilterChip. Every filter is read, cleared and applied the same
 * way: clearing drops its URL parameter and nothing else, and none of them
 * ever changes what Send sends.
 */
export type FilterParam = "person" | "role";

/** `<project id>/<role key>`, or null when it is not that shape. */
export function parseRoleRef(ref: string | null): { projectId: string; roleKey: string } | null {
  const cut = ref ? ref.indexOf("/") : -1;
  return ref && cut > 0 && cut < ref.length - 1 ? { projectId: ref.slice(0, cut), roleKey: ref.slice(cut + 1) } : null;
}

export interface FilterableRow {
  user_id: string;
  project_id: string;
  role_keys?: string[] | null;
}

export interface InForce {
  person: string | null;
  role: { projectId: string; roleKey: string } | null;
}

export function narrow<T extends FilterableRow>(rows: T[], f: InForce): T[] {
  return rows
    .filter((row) => !f.person || row.user_id === f.person)
    .filter((row) => !f.role || (row.project_id === f.role.projectId && (row.role_keys ?? []).includes(f.role.roleKey)));
}

/** The URL's filters, less any the operator has cleared since arriving. */
export function usePendingFilters(params: { get(name: string): string | null } | null) {
  const [cleared, setCleared] = useState<Partial<Record<FilterParam, string>>>({});
  const live = (p: FilterParam) => {
    const v = params?.get(p) ?? null;
    return v && v !== cleared[p] ? v : null;
  };
  const person = live("person");
  const roleRef = live("role");
  function clear(param: FilterParam) {
    setCleared((c) => ({ ...c, [param]: params?.get(param) ?? "" }));
    const url = new URL(window.location.href);
    url.searchParams.delete(param);
    window.history.replaceState(window.history.state, "", url);
  }
  const inForce: InForce = { person, role: parseRoleRef(roleRef) };
  return { inForce, key: (person ?? "") + (roleRef ?? ""), active: Boolean(person || inForce.role), clear };
}
