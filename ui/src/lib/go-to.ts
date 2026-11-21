import { recallPaths, remember } from "@/lib/memory";
import type { NavEntry } from "@/lib/nav";
import { sortAccounts, sortByName } from "@/lib/sort";
import type { ApplicationCatalog, Bundle, ProjectCatalog, UserProfile } from "@/lib/types";

/**
 * Go to (A-19 P4): one way to reach anything by typing its name.
 *
 * Pure, so the surface and the phone sheet match the same way. Groups are
 * fixed and in this order; inside a group the rows are A to Z by the name
 * shown, never by score (RULES: alphabetical, always; nothing is pinned). A
 * group with no match is left out.
 *
 * The one list in time order is Lately (A-24 §8, Q13): with nothing typed, the
 * last places opened on this device, most recent first, above Pages. Each is a
 * path, drawn from what this session can open now, so a place this view has no
 * page for, or that is gone, is simply not listed.
 */
export const GO_TO_GROUPS = ["Lately", "Pages", "People", "Projects", "Roles", "Apps", "Bundles"] as const;
export type GoToGroup = (typeof GO_TO_GROUPS)[number];

export interface GoToItem {
  /** Unique within the list: a page's href, or `kind:id`. */
  key: string;
  name: string;
  /**
   * One quiet line: where it is, or what it holds. Empty when there is nothing
   * to add to the kind on the right: a page at the top of the rail is a "Page"
   * and nothing more, never "A page" said twice.
   */
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
  /** Paths opened on this device, most recent first (`recallLately`). */
  lately?: string[];
  /** The path on screen: Lately never offers the place you are in. */
  here?: string;
}

/** How many places Lately lists. */
export const LATELY_SHOWN = 5;
// Kept a little longer than shown: some paths will not be listable in this view.
const LATELY_KEPT = 12;
const LATELY = "syndra:lately";

/** The places opened on this device, most recent first. */
export const recallLately = (): string[] => recallPaths("local", LATELY);

/** Notes a place as opened: to the front, once, and the oldest falls off. Its path, nothing else. */
export function keepLately(path: string) {
  remember("local", LATELY, [path, ...recallLately().filter((kept) => kept !== path)].slice(0, LATELY_KEPT));
}

/** Forgets every place, at sign-out: the next person on this computer starts with none. */
export const forgetLately = () => remember("local", LATELY, null);

/** Case and accents aside. */
export const fold = (text: string): string =>
  text
    .normalize("NFD")
    // ASCII only: the design-system bundle can be read without a charset.
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim();

const roleCount = (n = 0) => `${n} ${n === 1 ? "role" : "roles"}`;

function pages(nav: NavEntry[]): GoToItem[] {
  return nav.flatMap((entry) =>
    entry.kind === "leaf"
      ? [{ key: entry.href, name: entry.label, descriptor: "", kind: "Page", href: entry.href }]
      : entry.children.map((child) => ({
          key: child.href,
          name: child.label,
          descriptor: entry.label,
          kind: "Page",
          href: child.href,
        })),
  );
}

/**
 * Typed: each group's matches, in its own order. Nothing typed: Lately, then
 * Pages, and nothing else. Lately's rows are the very rows this session would
 * list, found by path, so they open what the group below would.
 */
function narrow(all: GoToResult[], query: string, source: GoToSource): GoToResult[] {
  const needle = fold(query);
  const every = all.flatMap((group) => group.items);
  const shown = needle
    ? all.map(({ group, items }) => ({ group, items: items.filter((item) => fold(item.name).includes(needle)) }))
    : [
        {
          group: "Lately" as const,
          items: (source.lately ?? [])
            .filter((path) => path !== source.here)
            .flatMap((path) => every.find((item) => item.href === path) ?? [])
            .slice(0, LATELY_SHOWN)
            .map((item) => ({ ...item, key: `lately:${item.key}` })),
        },
        all[0],
      ];
  return shown.filter((group) => group.items.length > 0);
}

export function goTo(source: GoToSource, query: string): GoToResult[] {
  return narrow(catalogue(source), query, source);
}

/** Everything this session may open, by group, each A to Z: Pages first, then the names. */
function catalogue(source: GoToSource): GoToResult[] {
  const out: GoToResult[] = [];
  const add = (group: GoToGroup, items: GoToItem[]) => out.push({ group, items });

  add("Pages", sortByName(pages(source.nav), (item) => item.name, (item) => item.key));
  if (!source.names) return out;

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
      descriptor: roleCount(p.roles?.length),
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
      descriptor: projectName.get(a.project_id) ?? "",
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
        descriptor: roleCount(b.roles?.length),
        kind: "Bundle",
        href: `/bundles/${encodeURIComponent(b.id)}`,
      })),
    );
  }
  return out;
}

/** Rows in the order they are drawn, for arrow keys. */
export const flatItems = (groups: GoToResult[]): GoToItem[] => groups.flatMap((g) => g.items);

// ── A-26, Go to as chosen (21 Nov 2026): one list, scopes, and the offer ──────────────────────────────────

/** A name, or what was typed, as words: case, accents and punctuation aside. */
export const matchKey = (text: string): string => fold(text).replace(/[\s·.,:;()_/\\-]+/g, " ").trim();

/**
 * How well a phrase names a label: 0 the whole name, 1 its start, 2 every word typed starts a word of it,
 * null not at all. Go to's list and Insights' questions read names the same way (one source).
 */
export function rankName(label: string, phrase: string): number | null {
  const name = matchKey(label);
  const typed = matchKey(phrase);
  if (!typed) return null;
  if (name === typed) return 0;
  if (name.startsWith(typed)) return 1;
  const words = name.split(" ");
  return typed.split(" ").every((part) => words.some((word) => word.startsWith(part))) ? 2 : null;
}

/**
 * The kinds Go to can be asked to look among (A-26 §0): each by its word, its first key, and the group it
 * keeps. A scope lists that kind only, A to Z.
 */
export const GO_TO_SCOPES = {
  people: { word: "People", hint: "A person", sigil: "@", one: "person", group: "People" },
  roles: { word: "Roles", hint: "A role", sigil: "#", one: "role", group: "Roles" },
  pages: { word: "Pages", hint: "A page", sigil: "/", one: "page", group: "Pages" },
} as const satisfies Record<string, { word: string; hint: string; sigil: string; one: string; group: GoToGroup }>;
export type GoToScope = keyof typeof GO_TO_SCOPES;
const SCOPES = Object.keys(GO_TO_SCOPES) as GoToScope[];

/** A kind's own word being typed offers its scope ("peo" offers People): two letters or more. */
export function offeredScope(query: string): GoToScope | null {
  const word = matchKey(query);
  if (word.length < 2) return null;
  return SCOPES.find((scope) => matchKey(GO_TO_SCOPES[scope].word).startsWith(word) || GO_TO_SCOPES[scope].one === word) ?? null;
}

/** A scope's first key typed first (`@` `#` `/`) is the scope, and is not kept as typing. */
export const sigilScope = (typed: string): GoToScope | null => SCOPES.find((scope) => typed.startsWith(GO_TO_SCOPES[scope].sigil)) ?? null;

/** A row of Go to's one list: an item, and the word at its right (its kind, Lately, or Needs you). */
export interface GoToRow extends GoToItem {
  tag: string;
}

/** Looking everywhere, the best matches shown at most. */
export const GO_TO_SHOWN = 7;
/** At rest, the last place only; `↓` asks for the rest. */
export const LATELY_AT_REST = 1;
const PENDING = "/governance/pending";

export interface GoToListSource extends GoToSource {
  /** How many changes wait for you to send, from the rail's own read. */
  waiting?: number;
}

/**
 * The one list (A-26 §0): at rest, the last place (`more`: five, and what needs you last, only when
 * something waits and this view has the page); looking everywhere, best match first, seven at most; in a
 * scope, every one of that kind that matches, A to Z.
 */
export function goToRows(source: GoToListSource, query: string, scope: GoToScope | null, more = false): GoToRow[] {
  const all = catalogue(source);
  const every = all.flatMap((group) => group.items);
  const typed = query.trim();
  if (scope) {
    const group = all.find((g) => g.group === GO_TO_SCOPES[scope].group)?.items ?? [];
    const needle = matchKey(typed);
    return group
      .filter((item) => !needle || rankName(item.name, typed) != null || matchKey(item.name).includes(needle))
      .map((item) => ({ ...item, tag: item.kind }));
  }
  if (!typed) {
    const lately = (source.lately ?? [])
      .filter((path) => path !== source.here)
      .flatMap((path) => every.find((item) => item.href === path) ?? [])
      .slice(0, more ? LATELY_SHOWN : LATELY_AT_REST)
      .map((item) => ({ ...item, key: `lately:${item.key}`, tag: "Lately" }));
    const page = every.find((item) => item.href === PENDING);
    const waiting = source.waiting ?? 0;
    const needs =
      more && waiting > 0 && page
        ? [{ ...page, key: "needs", descriptor: `${waiting} ${waiting === 1 ? "change is" : "changes are"} waiting for you to send`, tag: "Needs you" }]
        : [];
    return [...lately, ...needs];
  }
  return best(every, [typed], GO_TO_SHOWN);
}

/** Under an answer, the places its question names, best first: three at most (A-26 §0, the board). */
export function goToNamed(source: GoToSource, names: string[]): GoToRow[] {
  return names.length ? best(catalogue(source).flatMap((group) => group.items), names, 3) : [];
}

/** Best match first against any of the phrases: a whole name, its start, a word's start, then anywhere in it. */
function best(every: GoToItem[], phrases: string[], shown: number): GoToRow[] {
  const rank = (name: string) =>
    Math.min(...phrases.map((phrase) => rankName(name, phrase) ?? (matchKey(name).includes(matchKey(phrase)) ? 3 : Infinity)));
  return every
    .map((item) => ({ item, r: rank(item.name) }))
    .filter((hit) => hit.r !== Infinity)
    .sort((a, b) => a.r - b.r)
    .slice(0, shown)
    .map(({ item }) => ({ ...item, tag: item.kind }));
}
