import type { NavEntry } from "@/lib/nav";
import { sortAccounts, sortByName } from "@/lib/sort";
import type { ApplicationCatalog, Bundle, ProjectCatalog, UserProfile } from "@/lib/types";

/**
 * Go to (A-19 P4): one way to reach anything by typing its name.
 *
 * Pure, so the surface and the phone sheet match the same way. Groups are
 * fixed and in this order; inside a group the rows are A to Z by the name
 * shown, never by score and never by recency (RULES: alphabetical, always;
 * nothing is pinned). A group with no match is left out.
 */
export const GO_TO_GROUPS = ["Pages", "People", "Projects", "Roles", "Apps", "Bundles"] as const;
export type GoToGroup = (typeof GO_TO_GROUPS)[number];

export interface GoToItem {
  /** Unique within the list: a page's href, or `kind:id`. */
  key: string;
  name: string;
  /** One quiet line: what it is in context. */
  descriptor: string;
  /** What it is, said on the right. */
  kind: string;
  href: string;
}

export interface GoToResult {
  group: GoToGroup;
  items: GoToItem[];
}

export interface GoToSource {
  /** The pages this session may open: its own view's navigation tree (lib/nav.ts). */
  nav: NavEntry[];
  /** People, projects, roles, apps and bundles are an operator's to open; a member's list is pages only. */
  names?: boolean;
  /** Bundles are Advanced's: Basic has no Bundles page to land on. */
  bundles?: boolean;
  people?: UserProfile[];
  projects?: ProjectCatalog[];
  apps?: ApplicationCatalog[];
  bundleList?: Bundle[];
}

/** Case and accents aside. */
export const fold = (text: string): string =>
  text
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .trim();

function pages(nav: NavEntry[]): GoToItem[] {
  return nav.flatMap((entry) =>
    entry.kind === "leaf"
      ? [{ key: entry.href, name: entry.label, descriptor: "A page", kind: "Page", href: entry.href }]
      : entry.children.map((child) => ({
          key: child.href,
          name: child.label,
          descriptor: entry.label,
          kind: "Page",
          href: child.href,
        })),
  );
}

export function goTo(source: GoToSource, query: string): GoToResult[] {
  const needle = fold(query);
  const hit = (name: string) => !needle || fold(name).includes(needle);
  const out: GoToResult[] = [];
  const add = (group: GoToGroup, items: GoToItem[]) => {
    const matched = items.filter((item) => hit(item.name));
    if (matched.length) out.push({ group, items: matched });
  };

  add("Pages", sortByName(pages(source.nav), (item) => item.name, (item) => item.key));
  // With nothing typed the list is Pages only.
  if (!needle || !source.names) return out;

  // People A to Z, then service accounts A to Z, as every picker does.
  add(
    "People",
    sortAccounts(source.people ?? []).map((user) => ({
      key: `person:${user.id}`,
      name: user.name,
      descriptor: [user.title, user.team].filter(Boolean).join(" · ") || user.email,
      kind: user.service_account ? "Service account" : "Person",
      href: `/users/${encodeURIComponent(user.id)}`,
    })),
  );
  const projects = source.projects ?? [];
  add(
    "Projects",
    sortByName(projects, (p) => p.name).map((p) => ({
      key: `project:${p.id}`,
      name: p.name,
      descriptor: `${p.roles?.length ?? 0} ${p.roles?.length === 1 ? "role" : "roles"}`,
      kind: "Project",
      href: `/projects/${encodeURIComponent(p.id)}`,
    })),
  );
  add(
    "Roles",
    sortByName(
      projects.flatMap((p) =>
        (p.roles ?? []).map((role) => ({
          key: `role:${p.id}:${role.key}`,
          name: role.label || role.key,
          descriptor: p.name,
          kind: "Role",
          href: `/projects/${encodeURIComponent(p.id)}/roles/${encodeURIComponent(role.key)}`,
        })),
      ),
      (item) => item.name,
      (item) => item.key,
    ),
  );
  const projectName = new Map(projects.map((p) => [p.id, p.name]));
  add(
    "Apps",
    sortByName(source.apps ?? [], (a) => a.name).map((a) => ({
      key: `app:${a.id}`,
      name: a.name,
      descriptor: projectName.get(a.project_id) ?? "An app",
      kind: "App",
      href: `/applications/${encodeURIComponent(a.id)}`,
    })),
  );
  if (source.bundles) {
    add(
      "Bundles",
      sortByName(source.bundleList ?? [], (b) => b.name).map((b) => ({
        key: `bundle:${b.id}`,
        name: b.name,
        descriptor: "A bundle of roles",
        kind: "Bundle",
        href: `/bundles/${encodeURIComponent(b.id)}`,
      })),
    );
  }
  return out;
}

/** Rows in the order they are drawn, for arrow keys. */
export const flatItems = (groups: GoToResult[]): GoToItem[] => groups.flatMap((g) => g.items);
