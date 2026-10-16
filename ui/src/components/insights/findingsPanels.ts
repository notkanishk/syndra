import type { Panel } from "@/components/insights/AnswerPanel";
import { roleName } from "@/components/insights/answers";
import { listOf, openOf, refNode, type Model, type Tone } from "@/components/insights/model";
import type { InsightOverlap, InsightSingleRoute, InsightsFindings } from "@/lib/queries/useInsights";

/**
 * The three findings that read how access is arranged (single route, unused,
 * overlap) and the panel at rest. Each is a sentence the map can draw: the
 * panel, and what to light, come from the same value.
 */

/** What a finding lights: nodes (some also light what they give) and threads. */
export interface Lit {
  nodes: Array<{ id: string; tone: Tone; down?: boolean }>;
  threads: Array<{ id: string; tone: Tone }>;
}

export interface FindingView {
  panel: Panel;
  lit: Lit;
  /** Nodes that carry a mark on the map. */
  marked: string[];
}

const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
const roleWord = (n: number) => `${n} ${n === 1 ? "role" : "roles"}`;
const MORE = 4;
const bundleHref = (id: string) => `/bundles/${encodeURIComponent(id)}`;

export function singleRoutesView(m: Model, all: InsightSingleRoute[]): FindingView {
  const routes = [...all].sort((a, b) => b.only_that_way - a.only_that_way);
  const top = routes[0];
  const role = roleName(m, top);
  const src = top.source.name;
  const others = top.holders - top.only_that_way;
  const holders = (r: InsightSingleRoute) => `${r.only_that_way} of ${r.holders} ${r.holders === 1 ? "person" : "people"}`;

  const roleNode = m.roleId(top.project_id, top.role_key);
  const thread =
    top.source.kind === "rule"
      ? m.threads.find((t) => t.id === `rule:${top.source.id}`)
      : (m.inn.get(roleNode ?? "") ?? []).find((t) => t.from === refNode(m, "bundle", top.source.id));
  const sourceNode = thread?.from ?? null;

  const lines: Panel["lines"] = [
    { tone: "warn", lead: `A change to ${src}`, text: `reaches ${role} for ${people(top.only_that_way)}.` },
    ...(others > 0 ? [{ tone: "ink" as const, lead: `The other ${others}`, text: "get it another way." }] : []),
    ...routes.slice(1, 1 + MORE).map((r) => ({
      tone: "ink" as const,
      lead: `${roleName(m, r)} depends on ${r.source.name}:`,
      text: `${holders(r)}.`,
    })),
    ...(routes.length > 1 + MORE ? [{ tone: "ink" as const, text: `And ${routes.length - 1 - MORE} more.` }] : []),
  ];

  return {
    panel: {
      tone: "warn",
      eyebrow: "Single route",
      title: `${role} depends on ${src}`,
      meta: `${holders(top)} get it only that way`,
      lines,
      open: { label: `Open ${src}`, href: top.source.kind === "bundle" ? bundleHref(top.source.id) : "/policies" },
    },
    lit: {
      nodes: [
        ...(roleNode ? [{ id: roleNode, tone: "warn" as const, down: true }] : []),
        ...(sourceNode ? [{ id: sourceNode, tone: "warn" as const }] : []),
      ],
      threads: thread ? [{ id: thread.id, tone: "warn" }] : [],
    },
    marked: sourceNode && top.source.kind === "bundle" ? [sourceNode] : [],
  };
}

export function unusedView(m: Model, u: InsightsFindings["unused"]) {
  const inProject = new Set(u.projects.map((p) => p.project_id));
  const loose = u.roles.filter((r) => !inProject.has(r.project_id));
  const count = u.projects.length + u.bundles.length + loose.length;

  const projectNodes = u.projects.map((p) => refNode(m, "project", p.project_id));
  const bundleNodes = u.bundles.map((b) => refNode(m, "bundle", b.bundle_id));
  const roleNodes = u.roles.map((r) => m.roleId(r.project_id, r.role_key));
  const ids = [...projectNodes, ...bundleNodes, ...roleNodes].filter((x): x is string => Boolean(x));

  const lines: Panel["lines"] = [
    ...u.projects.map((p) => {
      const k = u.roles.filter((r) => r.project_id === p.project_id).length;
      return {
        tone: "ink" as const,
        lead: `${p.name}:`,
        text: k === 0 ? "no roles yet." : `${roleWord(k)}, and nobody has ${k === 1 ? "it" : k === 2 ? "either" : "any of them"}.`,
      };
    }),
    ...u.bundles.map((b) => ({
      tone: "ink" as const,
      lead: `${b.name}:`,
      text: b.holders > 0 ? `its ${people(b.holders)} already have everything it gives.` : "nobody holds it yet.",
    })),
  ];
  if (loose.length) {
    const where = Array.from(new Set(loose.map((r) => m.byId.get(refNode(m, "project", r.project_id) ?? "")?.label ?? "another project")));
    lines.push({ tone: "ink", lead: roleWord(loose.length), text: `in ${listOf(where)} that nobody has.` });
  }

  const first = projectNodes[0] ?? bundleNodes[0] ?? roleNodes[0];
  const target = first ? openOf(m, first) : null;
  const panel: Panel = {
    tone: "neutral",
    eyebrow: "Unused",
    title: `${count} ${count === 1 ? "thing gives" : "things give"} nothing`,
    meta: "Dotted on the map",
    lines: lines.slice(0, 6),
    open: target ?? { label: "See them in Roles", href: "/roles?unused=1" },
  };
  const names = [...u.projects.map((p) => p.name), ...u.bundles.map((b) => b.name), ...loose.map((r) => roleName(m, r))];
  return { panel, count, ids, names };
}

export function overlapView(m: Model, all: InsightOverlap[]): FindingView {
  const overlaps = [...all].sort((a, b) => b.shared.length - a.shared.length);
  const o = overlaps[0];
  const [big, small, bigOnly, smallOnly] = o.a_only.length >= o.b_only.length ? [o.a, o.b, o.a_only, o.b_only] : [o.b, o.a, o.b_only, o.a_only];
  const n = o.shared.length;

  const lines: Panel["lines"] = [
    { tone: "ink", lead: "Both give", text: `${listOf(o.shared.map((r) => roleName(m, r)))}.` },
    ...(o.a_only.length ? [{ tone: "accent2" as const, lead: `Only ${o.a.name} gives`, text: `${listOf(o.a_only.map((r) => roleName(m, r)))}.` }] : []),
    ...(o.b_only.length ? [{ tone: "accent2" as const, lead: `Only ${o.b.name} gives`, text: `${listOf(o.b_only.map((r) => roleName(m, r)))}.` }] : []),
    smallOnly.length === 0
      ? {
          tone: "accent" as const,
          lead: "You could include",
          text: `${small.name} inside ${big.name}, so a change to the shared ${n === 1 ? "role" : n} is made once.`,
        }
      : { tone: "ink" as const, lead: "Each also gives roles the other doesn't,", text: "so including one in the other would change who gets what." },
    ...overlaps.slice(1, 4).map((x) => ({
      tone: "ink" as const,
      lead: `${x.a.name} and ${x.b.name}:`,
      text: `${roleWord(x.shared.length)} in common.`,
    })),
    ...(overlaps.length > 4 ? [{ tone: "ink" as const, text: `And ${overlaps.length - 4} more.` }] : []),
  ];

  const nodes: Lit["nodes"] = [];
  const threads: Lit["threads"] = [];
  for (const b of [o.a, o.b]) {
    const id = refNode(m, "bundle", b.bundle_id);
    if (id) nodes.push({ id, tone: "accent" });
  }
  const give = (roleRefs: typeof o.shared, tone: Tone, bundles: Array<typeof o.a>) =>
    roleRefs.forEach((r) => {
      const id = m.roleId(r.project_id, r.role_key);
      if (!id) return;
      nodes.push({ id, tone });
      for (const t of m.inn.get(id) ?? [])
        if (bundles.some((b) => t.from === refNode(m, "bundle", b.bundle_id))) threads.push({ id: t.id, tone });
    });
  give(o.shared, "ink", [o.a, o.b]);
  give(o.a_only, "accent2", [o.a]);
  give(o.b_only, "accent2", [o.b]);

  return {
    panel: {
      tone: "accent",
      eyebrow: "Overlap",
      title: `${o.a.name} and ${o.b.name}`,
      meta: `${roleWord(n)} in common · ${big.name} adds ${bigOnly.length}`,
      lines,
      open: { label: `Open ${big.name}`, href: bundleHref(big.bundle_id) },
    },
    lit: { nodes, threads },
    marked: [],
  };
}

/**
 * The panel at rest: three things worth knowing, each only when it can be
 * computed from what has been read. A count that has not been read is not a
 * zero, so a missing one drops its line.
 */
export function glancePanel(m: Model, held: Map<string, number>, noReach: InsightsFindings["unused"]["projects"]): Panel | null {
  const lines: Panel["lines"] = [];

  let widest: { id: string; n: number } | null = null;
  for (const r of m.lanes.role) {
    const n = held.get(r.id);
    if (n && (!widest || n > widest.n)) widest = { id: r.id, n };
  }
  if (widest) {
    const via = (m.inn.get(widest.id) ?? []).filter((t) => !t.rule).map((t) => m.nameOf(t.from));
    lines.push({
      tone: "ink",
      lead: "Widest reach:",
      text: `${m.nameOf(widest.id)}, ${people(widest.n)}${via.length ? `, through ${listOf(via)}` : ""}.`,
    });
  }

  if (noReach.length) {
    const only = noReach.length === 1 ? refNode(m, "project", noReach[0].project_id) : null;
    const k = only ? (m.inn.get(only) ?? []).length : 0;
    lines.push({
      tone: "ink",
      lead: "Nothing reaches:",
      text: only
        ? `${noReach[0].name}. It has ${k === 0 ? "no roles yet" : `${roleWord(k)} and nobody has ${k === 1 ? "it" : k === 2 ? "either" : "any of them"}`}.`
        : `${listOf(noReach.map((p) => p.name))}.`,
    });
  }

  let auto: { to: string; from: string; n: number } | null = null;
  for (const t of m.threads) {
    const n = t.rule ? held.get(t.from) : undefined;
    if (n && (!auto || n > auto.n)) auto = { to: t.to, from: t.from, n };
  }
  if (auto)
    lines.push({ tone: "ink", lead: "Most given automatically:", text: `${m.nameOf(auto.to)}, ${people(auto.n)}, by rule from ${m.nameOf(auto.from)}.` });

  if (!lines.length) return null;
  return {
    tone: "neutral",
    eyebrow: "The makerspace at a glance",
    title: "How access travels today",
    meta: `${["", "One thing", "Two things", "Three things"][lines.length]} worth knowing`,
    lines,
  };
}
