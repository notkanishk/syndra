import { formatList } from "@/lib/format";
import type { TopologyGraph, TopologyNode } from "@/lib/queries/useTopology";

/**
 * Insights reads one graph and draws it as four lanes: what is given through
 * (bundles), the roles, the projects, and what reads them (apps). A thread
 * points the way access travels, so the API's edges are turned around where
 * they point at what they are about: a project "contains" a role, but access
 * travels from the role to the project.
 *
 * Pure on purpose. The map, the list, the panel's sentences and the tests all
 * read this one shape, so a preview and what it previews cannot disagree.
 */

export type Lane = TopologyNode["kind"];

export const LANES: Array<{ kind: Lane; title: string; noun: string }> = [
  { kind: "bundle", title: "Given through", noun: "bundles" },
  { kind: "role", title: "Roles", noun: "roles" },
  { kind: "project", title: "Projects", noun: "projects" },
  { kind: "application", title: "Read by", noun: "apps" },
];

export interface Thread {
  id: string;
  from: string;
  to: string;
  /** An automatic rule: drawn dashed. */
  rule: boolean;
}

export interface Model {
  byId: Map<string, TopologyNode>;
  lanes: Record<Lane, TopologyNode[]>;
  threads: Thread[];
  out: Map<string, Thread[]>;
  inn: Map<string, Thread[]>;
  nameOf: (id: string) => string;
  /** The project a role sits in, when the map has it. */
  projectOf: (id: string) => TopologyNode | undefined;
  /** The role node for a project and role key, when the map has one. */
  roleId: (projectId: string, roleKey: string) => string | null;
}

export function buildModel(graph: TopologyGraph): Model {
  const byId = new Map(graph.nodes.map((n) => [n.id, n]));
  const lanes: Record<Lane, TopologyNode[]> = { bundle: [], role: [], project: [], application: [] };
  for (const n of [...graph.nodes].sort((a, b) => a.label.localeCompare(b.label))) lanes[n.kind]?.push(n);

  const projectByPid = new Map(lanes.project.map((p) => [p.project_id ?? p.id, p]));
  const roleIds = new Map<string, string>();
  for (const r of lanes.role) {
    const key = r.meta?.role_key;
    if (r.project_id && key) roleIds.set(`${r.project_id}\u0000${key}`, r.id);
  }

  const threads: Thread[] = [];
  for (const e of graph.edges) {
    const turned = e.kind === "contains" || e.kind === "application";
    const [from, to] = turned ? [e.target, e.source] : [e.source, e.target];
    if (byId.has(from) && byId.has(to)) threads.push({ id: e.id, from, to, rule: e.kind === "rule" });
  }
  const out = new Map<string, Thread[]>();
  const inn = new Map<string, Thread[]>();
  for (const t of threads) {
    out.set(t.from, [...(out.get(t.from) ?? []), t]);
    inn.set(t.to, [...(inn.get(t.to) ?? []), t]);
  }

  return {
    byId,
    lanes,
    threads,
    out,
    inn,
    projectOf: (id) => {
      const n = byId.get(id);
      return n?.kind === "role" && n.project_id ? projectByPid.get(n.project_id) : undefined;
    },
    // A role is never named alone: two projects can each have an "Operator".
    nameOf: (id) => {
      const n = byId.get(id);
      if (!n) return id;
      const project = n.kind === "role" && n.project_id ? projectByPid.get(n.project_id) : undefined;
      return project ? `${project.label} · ${n.label}` : n.label;
    },
    roleId: (projectId, roleKey) => roleIds.get(`${projectId}\u0000${roleKey}`) ?? null,
  };
}

export interface Reach {
  nodes: Set<string>;
  threads: Set<string>;
}

/** Everything a node is given by (up) and everything it gives (down), transitively. */
export function reach(m: Model, id: string, dirs: Array<"up" | "down"> = ["up", "down"]): Reach {
  const nodes = new Set([id]);
  const threads = new Set<string>();
  for (const dir of dirs) {
    const seen = new Set([id]);
    const stack = [id];
    while (stack.length) {
      const at = stack.pop()!;
      for (const t of (dir === "down" ? m.out : m.inn).get(at) ?? []) {
        threads.add(t.id);
        const next = dir === "down" ? t.to : t.from;
        nodes.add(next);
        if (!seen.has(next)) {
          seen.add(next);
          stack.push(next);
        }
      }
    }
  }
  return { nodes, threads };
}

/** How a thread or node is drawn: a shade for a pin, or a finding's colour. */
export type Tone = "accent" | "accent2" | "accent3" | "warn" | "danger" | "ink";

export interface Highlight {
  nodes: Map<string, Tone>;
  threads: Map<string, Tone>;
}

export const PIN_TONES: Tone[] = ["accent", "accent2", "accent3"];

/** The first thing to paint a node or thread keeps it. */
export function paint(h: Highlight, r: Reach, tone: Tone) {
  r.nodes.forEach((id) => h.nodes.has(id) || h.nodes.set(id, tone));
  r.threads.forEach((id) => h.threads.has(id) || h.threads.set(id, tone));
}

/** Each pin's shade: the followed node always takes the main one. */
export function pinTones(pins: string[], focus: string | null): Map<string, Tone> {
  const tones = new Map<string, Tone>();
  if (focus) tones.set(focus, "accent");
  let i = focus ? 1 : 0;
  for (const p of pins) {
    if (p === focus) continue;
    tones.set(p, PIN_TONES[Math.min(i++, 2)]);
  }
  return tones;
}

/** The map node for a bundle or project id, whether it is given bare or with the node's prefix. */
export function refNode(m: Model, kind: "bundle" | "project", id: string): string | null {
  return [`${kind}:${id}`, id].find((x) => m.byId.get(x)?.kind === kind) ?? null;
}

/** Up to four names, then how many more. */
export function listOf(names: string[], max = 4): string {
  return names.length > max ? `${names.slice(0, max).join(", ")} and ${names.length - max} more` : formatList(names);
}

const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;

function ofKind(m: Model, ids: Set<string>, kind: Lane, except: string): string[] {
  return Array.from(ids).filter((x) => x !== except && m.byId.get(x)?.kind === kind).map(m.nameOf);
}

/** What a followed node says, in sentences. */
export function describe(m: Model, id: string): string[] {
  const node = m.byId.get(id);
  if (!node) return [];
  const n = m.nameOf(id);
  const inn = m.inn.get(id) ?? [];
  const out = m.out.get(id) ?? [];
  const all = reach(m, id);
  const apps = ofKind(m, all.nodes, "application", id);
  const lines: string[] = [];

  if (node.kind === "role") {
    const given = inn.filter((t) => !t.rule).map((t) => m.nameOf(t.from));
    if (given.length && apps.length) lines.push(`${n} is given by ${listOf(given)} and read by ${listOf(apps)}.`);
    else if (given.length) lines.push(`${n} is given by ${listOf(given)}. No app reads it.`);
    else if (apps.length) lines.push(`No bundle gives ${n}. It is read by ${listOf(apps)}.`);
    else lines.push(`No bundle gives ${n}, and no app reads it.`);
    const ruleIn = inn.filter((t) => t.rule).map((t) => m.nameOf(t.from));
    const ruleOut = out.filter((t) => t.rule).map((t) => m.nameOf(t.to));
    if (ruleIn.length) lines.push(`It also comes automatically with ${listOf(ruleIn)}.`);
    if (ruleOut.length) lines.push(`It gives ${listOf(ruleOut)} automatically.`);
  } else if (node.kind === "bundle") {
    const roles = out.map((t) => m.nameOf(t.to));
    lines.push(roles.length ? `${n} gives ${listOf(roles)}.` : `${n} gives no roles yet.`);
    if (apps.length) lines.push(`Through them it is read by ${listOf(apps)}.`);
  } else if (node.kind === "project") {
    const roles = inn.map((t) => m.nameOf(t.from));
    lines.push(roles.length ? `${n} has ${listOf(roles)}.` : `${n} has no roles yet.`);
    const bundles = ofKind(m, all.nodes, "bundle", id);
    if (bundles.length) lines.push(`It is given through ${listOf(bundles)}.`);
    lines.push(apps.length ? `It is read by ${listOf(apps)}.` : "No app reads it.");
  } else {
    const projects = inn.map((t) => m.nameOf(t.from));
    lines.push(projects.length ? `${n} reads ${listOf(projects)}.` : `${n} reads no project yet.`);
    const bundles = ofKind(m, all.nodes, "bundle", id);
    if (bundles.length) lines.push(`Access reaches it through ${listOf(bundles)}.`);
  }
  return lines;
}

/** One pinned thread, in a line: its people, and what reads it. */
export function pinLine(m: Model, id: string, count: number | null): string {
  const readers = ofKind(m, reach(m, id).nodes, "application", id);
  const bits = [count != null ? people(count) : null, readers.length ? `read by ${listOf(readers)}` : "no app reads it"];
  return `${m.nameOf(id)}: ${bits.filter(Boolean).join(", ")}.`;
}

/** The apps that read more than one pinned thread. */
export function crossing(m: Model, pins: string[]): string | null {
  if (pins.length < 2) return null;
  const sets = pins.map((p) => new Set(ofKind(m, reach(m, p).nodes, "application", p)));
  const common = Array.from(new Set(sets.flatMap((s) => Array.from(s)))).filter((a) => sets.filter((s) => s.has(a)).length >= 2);
  return common.length ? `Where they cross: ${listOf(common)} reads ${pins.length > 2 ? "more than one of them" : "both"}.` : null;
}

export function kindMeta(m: Model, id: string, count: number | null): string {
  const node = m.byId.get(id);
  if (!node) return "";
  const kind = { role: "Role", bundle: "Bundle", project: "Project", application: "App" }[node.kind];
  const project = m.projectOf(id);
  return [kind, project?.label, count != null ? people(count) : null].filter(Boolean).join(" · ");
}

/** The page where a node is changed. A role has no page: its project does. */
export function openOf(m: Model, id: string): { label: string; href: string } | null {
  const node = m.byId.get(id);
  if (!node) return null;
  const bare = (prefix: string) => encodeURIComponent(node.id.replace(prefix, ""));
  switch (node.kind) {
    case "bundle":
      return { label: `Open ${node.label}`, href: `/bundles/${bare("bundle:")}` };
    case "application":
      return { label: `Open ${node.label}`, href: `/applications/${bare("application:")}` };
    case "project":
      return { label: `Open ${node.label}`, href: `/projects/${encodeURIComponent(node.project_id ?? node.id.replace("project:", ""))}` };
    case "role": {
      return node.project_id
        ? { label: `Open ${m.projectOf(id)?.label ?? "the project"}`, href: `/projects/${encodeURIComponent(node.project_id)}` }
        : null;
    }
  }
}
