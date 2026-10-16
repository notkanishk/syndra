import { orderedSources } from "@/components/access/AccessSource";
import type { Panel } from "@/components/insights/AnswerPanel";
import { kindMeta, listOf, paint, reach, refNode, type Highlight, type Model, type Tone } from "@/components/insights/model";
import { formatList, humanizeKey } from "@/lib/format";
import type { InsightPerson, InsightsWhatIf, RoleRef } from "@/lib/queries/useInsights";
import type { AccessRoleReason } from "@/lib/queries/useUsers";

/**
 * What each question says, in sentences, and which threads the map draws for
 * it. Pure on purpose: the panel, the map and the tests read the same value,
 * and the hooks that fetch only hand their data to these.
 */

export interface Answer {
  panel: Panel;
  highlight: Highlight | null;
}

const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;
const SHADES: Tone[] = ["accent", "accent2", "accent3"];

/** A role is never named alone, and an unmapped one falls back to its key. */
export function roleName(m: Model, r: RoleRef): string {
  const id = m.roleId(r.project_id, r.role_key);
  return id ? m.nameOf(id) : humanizeKey(r.role_key);
}

const roles = (m: Model, rs: RoleRef[]) => listOf(rs.map((r) => roleName(m, r)));

const bundleNode = (m: Model, id: string) => refNode(m, "bundle", id);

export interface Route {
  /** Equal routes of two people have equal keys. */
  key: string;
  /** "by hand", "through Lab Tech": how it reads inside a longer sentence. */
  how: string;
  lead: string;
  text: string;
  nodes: string[];
  threads: string[];
}

/** One route per reason, in the fixed reading order: by hand, bundles, rules. */
export function routesOf(m: Model, roleId: string | null, label: string, reasons: AccessRoleReason[] | undefined): Route[] {
  const into = roleId ? (m.inn.get(roleId) ?? []) : [];
  return orderedSources(reasons).map((r): Route => {
    const wait = r.queued ? " It is waiting for you to send." : "";
    if (r.kind === "direct") {
      const why = r.description && r.description !== "Direct Zitadel grant" ? ` The reason recorded: “${r.description}”.` : "";
      return { key: "direct", how: "by hand", lead: "By hand:", text: `given to them directly.${why}${wait}`, nodes: [], threads: [] };
    }
    if (r.kind === "bundle") {
      const b = r.bundle_name || "a bundle";
      const via = r.via_bundle_name;
      const giver = bundleNode(m, r.via_bundle_id || r.bundle_id || "");
      const thread = into.find((t) => t.from === giver);
      return {
        key: `bundle:${r.bundle_id ?? b}:${r.via_bundle_id ?? ""}`,
        how: via ? `through ${b}, from ${via}` : `through ${b}`,
        lead: via ? `Through ${b}, from ${via}:` : `Through ${b}:`,
        text: via ? `${b} includes ${via}, and ${via} gives ${label}.${wait}` : `${b} gives ${label}.${wait}`,
        nodes: [bundleNode(m, r.bundle_id ?? ""), giver].filter((x): x is string => Boolean(x)),
        threads: thread ? [thread.id] : [],
      };
    }
    const from = m.roleId(r.trigger_project ?? "", r.trigger_role ?? "");
    const trigger = from ? m.nameOf(from) : humanizeKey(r.trigger_role ?? "another role");
    const thread = into.find((t) => t.rule && t.from === from);
    return {
      key: `rule:${r.trigger_project}:${r.trigger_role}`,
      how: `by rule from ${trigger}`,
      lead: `By rule from ${trigger}:`,
      text: `${label} comes automatically with ${trigger}.${wait}`,
      nodes: from ? [from] : [],
      threads: thread ? [thread.id] : [],
    };
  });
}

/** Paint a node or thread; two different shades meeting on one become ink. */
function mark(h: Highlight, kind: "nodes" | "threads", id: string, tone: Tone) {
  const was = h[kind].get(id);
  h[kind].set(id, was && was !== tone ? "ink" : tone);
}

const lit = (h: Highlight) => (h.nodes.size ? h : null);

export function pathAnswer(
  m: Model,
  p: { id: string; name: string; meta?: string },
  roleId: string,
  reasons: AccessRoleReason[] | undefined,
): Answer {
  const role = m.nameOf(roleId);
  const routes = routesOf(m, roleId, role, reasons);
  const open = { label: `Open ${p.name}`, href: `/users/${encodeURIComponent(p.id)}` };
  if (!routes.length)
    return {
      panel: {
        tone: "neutral",
        eyebrow: "Path",
        title: `${p.name} to ${role}`,
        meta: p.meta,
        lines: [{ tone: "ink", lead: `${p.name}`, text: `doesn't have ${role}.` }],
        open,
      },
      highlight: null,
    };
  const h: Highlight = { nodes: new Map(), threads: new Map() };
  routes.forEach((r, i) => {
    const tone = SHADES[Math.min(i, 2)];
    [roleId, ...r.nodes].forEach((id) => h.nodes.has(id) || h.nodes.set(id, tone));
    r.threads.forEach((id) => h.threads.has(id) || h.threads.set(id, tone));
  });
  return {
    panel: {
      tone: "accent",
      eyebrow: routes.length > 1 ? `Path · ${routes.length} routes` : "Path",
      title: `${p.name} to ${role}`,
      meta: p.meta,
      lines: [
        ...routes.map((r, i) => ({ tone: SHADES[Math.min(i, 2)], lead: r.lead, text: r.text })),
        ...(routes.length > 1
          ? [{ tone: "ink" as const, lead: `${routes.length} routes.`, text: `Removing one of them won't stop ${p.name} having it.` }]
          : []),
      ],
      open,
    },
    highlight: lit(h),
  };
}

export function compareAnswer(
  m: Model,
  a: { id: string; name: string; reasons: AccessRoleReason[] | undefined },
  b: { id: string; name: string; reasons: AccessRoleReason[] | undefined },
  roleId: string,
): Answer {
  const role = m.nameOf(roleId);
  const ra = routesOf(m, roleId, role, a.reasons);
  const rb = routesOf(m, roleId, role, b.reasons);
  const inB = new Set(rb.map((r) => r.key));
  const inA = new Set(ra.map((r) => r.key));
  const both = ra.filter((r) => inB.has(r.key));
  const onlyA = ra.filter((r) => !inB.has(r.key));
  const onlyB = rb.filter((r) => !inA.has(r.key));
  const say = (rs: Route[]) => `${formatList(rs.map((r) => r.how))}.`;

  const lines: Panel["lines"] = [];
  if (!ra.length && !rb.length) lines.push({ tone: "ink", lead: "Neither", text: `has ${role}.` });
  else {
    for (const [p, rs] of [[a, ra], [b, rb]] as const) if (!rs.length) lines.push({ tone: "ink", lead: p.name, text: `doesn't have ${role}.` });
    if (both.length) lines.push({ tone: "ink", lead: "Both:", text: say(both) });
    if (onlyA.length) lines.push({ tone: "accent", lead: `Only ${a.name}:`, text: say(onlyA) });
    if (onlyB.length) lines.push({ tone: "accent2", lead: `Only ${b.name}:`, text: say(onlyB) });
  }

  const h: Highlight = { nodes: new Map(), threads: new Map() };
  const draw = (rs: Route[], own: Tone) =>
    rs.forEach((r) => {
      const tone = (own === "accent" ? inB : inA).has(r.key) ? "ink" : own;
      [roleId, ...r.nodes].forEach((id) => mark(h, "nodes", id, tone));
      r.threads.forEach((id) => mark(h, "threads", id, tone));
    });
  draw(ra, "accent");
  draw(rb, "accent2");

  return {
    panel: {
      tone: "accent",
      eyebrow: `Compare · on ${role}`,
      title: `${a.name} and ${b.name}`,
      meta: ra.length && rb.length ? "Both have it." : ra.length ? `${a.name} has it, ${b.name} doesn't.` : rb.length ? `${b.name} has it, ${a.name} doesn't.` : undefined,
      lines,
      open: { label: `Open ${a.name}`, href: `/users/${encodeURIComponent(a.id)}` },
    },
    highlight: lit(h),
  };
}

const MORE = 8;

export function whoAnswer(m: Model, hasId: string, withoutId: string | null, found: InsightPerson[]): Answer {
  const has = m.nameOf(hasId);
  const label = withoutId ? `Who has ${has} without ${m.nameOf(withoutId)}` : `Who has ${has}`;
  const h: Highlight = { nodes: new Map(), threads: new Map() };
  paint(h, reach(m, hasId, ["up"]), "accent");
  if (withoutId) h.nodes.set(withoutId, "ink");

  const lines: Panel["lines"] = found.slice(0, MORE).map((p) => {
    const hows = routesOf(m, hasId, has, p.how).map((r) => r.how);
    return { tone: "accent" as const, lead: `${p.name}:`, text: hows.length ? `${formatList(hows)}.` : "holds it." };
  });
  if (found.length > MORE) lines.push({ tone: "ink", text: `And ${found.length - MORE} more.` });
  if (!found.length)
    lines.push({
      tone: "ink",
      lead: withoutId ? "Everyone who has" : "Nobody has",
      text: withoutId ? `${has} also has ${m.nameOf(withoutId)}.` : `${has}.`,
    });

  return {
    panel: {
      tone: "accent",
      eyebrow: label,
      title: found.length ? people(found.length) : "Nobody",
      lines,
      open:
        found.length === 1
          ? { label: `Open ${found[0].name}`, href: `/users/${encodeURIComponent(found[0].user_id)}` }
          : found.length
            ? { label: "See them in People", href: "/users" }
            : undefined,
    },
    highlight: lit(h),
  };
}

/** What a change would do, by name. Nothing here changes anything. */
export function whatIfAnswer(
  m: Model,
  q: { bundle: string; role?: string },
  data: InsightsWhatIf,
): Answer {
  const bundle = m.nameOf(q.bundle);
  const gone = !q.role;
  const h: Highlight = { nodes: new Map(), threads: new Map() };
  const tone: Tone = gone ? "warn" : "accent";
  h.nodes.set(q.bundle, tone);
  if (gone) {
    for (const t of m.out.get(q.bundle) ?? []) h.threads.set(t.id, "warn");
    paint(h, reach(m, q.bundle, ["down"]), "ink");
  } else if (q.role) {
    h.nodes.set(q.role, "accent");
    paint(h, reach(m, q.role, ["down"]), "ink");
  }

  const lines: Panel["lines"] = [];
  const who = (list: InsightsWhatIf["gains"], verb: string, t: Tone) => {
    if (!list.length) return;
    lines.push({ tone: t, lead: people(list.length), text: `would ${verb} something.` });
    for (const p of list.slice(0, MORE)) lines.push({ tone: t, lead: p.name, text: `would ${verb} ${roles(m, p.roles)}.` });
    if (list.length > MORE) lines.push({ tone: "ink", text: `And ${list.length - MORE} more.` });
  };
  who(data.losses, "lose", "warn");
  who(data.gains, "gain", "accent");
  const nothing = !data.gains.length && !data.losses.length;
  if (nothing) lines.push({ tone: "ink", lead: "Nobody", text: "would gain or lose anything." });
  if (data.unchanged > 0)
    lines.push({ tone: "ink", lead: nothing ? "Everyone:" : "The rest:", text: `${people(data.unchanged)} would stay as they are.` });

  return {
    panel: {
      tone: gone ? "warn" : "accent",
      eyebrow: gone ? "What if it went · a preview, nothing changes" : "What if · a preview, nothing changes",
      title: gone ? bundle : `${bundle} also gave ${q.role ? m.nameOf(q.role) : ""}`,
      meta: kindMeta(m, q.bundle, null),
      lines,
      open: bundleOpen(m, q.bundle),
    },
    highlight: lit(h),
  };
}

function bundleOpen(m: Model, id: string): Panel["open"] {
  const node = m.byId.get(id);
  return node ? { label: `Change it on ${node.label}`, href: `/bundles/${encodeURIComponent(id.replace("bundle:", ""))}` } : undefined;
}
