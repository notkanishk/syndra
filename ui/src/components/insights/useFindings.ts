"use client";

import { useMemo } from "react";

import type { Panel } from "@/components/insights/AnswerPanel";
import { glancePanel, overlapView, singleRoutesView, unusedView, type Lit } from "@/components/insights/findingsPanels";
import type { Mark } from "@/components/insights/LaneMap";
import { listOf, type Model } from "@/components/insights/model";
import { formatShortDate, humanizeKey } from "@/lib/format";
import { useDriftItems } from "@/lib/queries/useDrift";
import { useInsightsFindings } from "@/lib/queries/useInsights";
import { usePendingPropagations } from "@/lib/queries/usePropagation";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";

import type { Finding } from "@/components/insights/useInsightsUrl";

export interface RestCard {
  finding: Finding;
  kind: string;
  title: string;
  line: string;
  open: { label: string; href: string };
}

const people = (n: number) => `${n} ${n === 1 ? "person" : "people"}`;

/**
 * What is worth a look. Drift and the pending changes come from the reads
 * that already exist; single routes, unused things and overlaps are one read
 * of their own (`/insights/findings`). A finding that has not been read has
 * no count, so its chip is not drawn (never a zero for "unknown").
 */
export function useFindings(m: Model | null) {
  const drift = useDriftItems();
  const pending = usePendingPropagations();
  const catalog = useGlobalRoleCatalog();
  const found = useInsightsFindings();

  return useMemo(() => {
    const roleName = (projectId: string, key: string, fallback?: string) => {
      const id = m?.roleId(projectId, key);
      return id && m ? m.nameOf(id) : fallback || humanizeKey(key);
    };
    const roleNode = (projectId: string, key: string) => m?.roleId(projectId, key) ?? null;
    const nodes: { drift: string[]; send: string[] } = { drift: [], send: [] };

    // A person count is what Zitadel shows; absent when it has never been read.
    const held = new Map<string, number>();
    for (const r of catalog.data ?? []) {
      const id = roleNode(r.project_id, r.role_key);
      if (id && r.observed_user_count != null) held.set(id, r.observed_user_count);
    }

    // Drift: one line per role, counting the people it was found on.
    const driftRows = drift.data;
    const groups = new Map<string, { name: string; type: string; users: Set<string> }>();
    for (const item of driftRows ?? []) {
      for (const key of item.role_keys) {
        const gk = `${item.drift_type}\u0000${item.project_id}\u0000${key}`;
        const g = groups.get(gk) ?? { name: roleName(item.project_id, key), type: item.drift_type, users: new Set() };
        g.users.add(item.user_id);
        groups.set(gk, g);
        const id = roleNode(item.project_id, key);
        if (id && !nodes.drift.includes(id)) nodes.drift.push(id);
      }
    }
    const driftCount = driftRows?.length ?? 0;
    const everyone = new Set((driftRows ?? []).map((i) => i.user_id)).size;
    const groupList = Array.from(groups.values());
    const firstFound = (driftRows ?? []).map((i) => i.detected_at).sort()[0];
    const heldHere = groupList.filter((g) => g.type !== "syndra_only");

    // Waiting for you to send: rows that always wait, or whose send stopped.
    const waiting = (pending.data ?? []).filter((r) => r.wait_reason);
    const sendGroups = new Map<string, { text: (k: number) => string; users: Set<string> }>();
    for (const r of waiting) {
      const roles = r.role_keys.map((k) => roleName(r.project_id, k));
      for (const k of r.role_keys) {
        const id = roleNode(r.project_id, k);
        if (id && !nodes.send.includes(id)) nodes.send.push(id);
      }
      const gk = `${r.op_type}\u0000${r.project_id}\u0000${r.role_keys.join()}`;
      const text = (k: number) =>
        !roles.length
          ? `${k} other ${k === 1 ? "change" : "changes"} once sent.`
          : r.op_type === "revoke"
            ? `Revokes ${listOf(roles)} from ${people(k)} once sent.`
            : r.op_type === "replace"
              ? `Changes ${listOf(roles)} for ${people(k)} once sent.`
              : `Gives ${listOf(roles)} to ${people(k)} once sent.`;
      const g = sendGroups.get(gk) ?? { text, users: new Set() };
      g.users.add(r.id);
      sendGroups.set(gk, g);
    }
    const sendLines = Array.from(sendGroups.values()).map((g) => g.text(g.users.size));

    // The findings Syndra reads from how access is arranged.
    const arrangement = found.data;
    const unused = m && arrangement ? unusedView(m, arrangement.unused) : null;
    const single = m && arrangement?.single_routes.length ? singleRoutesView(m, arrangement.single_routes) : null;
    const overlap = m && arrangement?.overlaps.length ? overlapView(m, arrangement.overlaps) : null;

    const counts: Record<Finding, number> = {
      drift: driftCount,
      send: waiting.length,
      single: arrangement?.single_routes.length ?? 0,
      unused: unused?.count ?? 0,
      overlap: arrangement?.overlaps.length ?? 0,
    };
    const driftTitle = "Held in Zitadel, not given by Syndra";

    const panel = (f: Finding): Panel => {
      if (f === "drift")
        return {
          tone: "danger",
          eyebrow: "Drift",
          title: driftTitle,
          meta: `${people(everyone)}${firstFound ? ` · found ${formatShortDate(firstFound)}` : ""}`,
          lines: [
            ...groupList.slice(0, 4).map((g) => ({
              tone: "danger" as const,
              text:
                g.type === "syndra_only"
                  ? `${g.name}: ${people(g.users.size)} should have it, and Zitadel doesn't show it.`
                  : `${g.name}: ${people(g.users.size)}.`,
            })),
            ...(heldHere.length
              ? [
                  { tone: "ink" as const, text: "Nothing on the map explains it: no bundle, request or grant by hand gives it to them." },
                  { tone: "ink" as const, text: "They keep it until you decide." },
                ]
              : []),
          ],
          open: { label: "Open on Drift", href: "/governance/drift" },
        };
      if (f === "send")
        return {
          tone: "warn",
          eyebrow: "Approved, not yet in Zitadel",
          title: `${waiting.length} ${waiting.length === 1 ? "change" : "changes"}`,
          lines: sendLines.slice(0, 4).map((text) => ({ tone: "warn" as const, text })),
          open: { label: "Waiting for you to send", href: "/governance/pending" },
        };
      if (f === "single" && single) return single.panel;
      if (f === "overlap" && overlap) return overlap.panel;
      return unused!.panel;
    };

    const lit = (f: Finding): Lit => {
      const down = (ids: string[], tone: "danger" | "warn") => ({ nodes: ids.map((id) => ({ id, tone, down: true })), threads: [] });
      if (f === "drift") return down(nodes.drift, "danger");
      if (f === "send") return down(nodes.send, "warn");
      if (f === "single") return single?.lit ?? { nodes: [], threads: [] };
      if (f === "overlap") return overlap?.lit ?? { nodes: [], threads: [] };
      return { nodes: (unused?.ids ?? []).map((id) => ({ id, tone: "ink" as const })), threads: [] };
    };

    const marks = new Map<string, Mark[]>();
    const put = (id: string, mark: Mark) => marks.set(id, [...(marks.get(id) ?? []), mark]);
    nodes.drift.forEach((id) => put(id, "drift"));
    nodes.send.forEach((id) => put(id, "send"));
    single?.marked.forEach((id) => put(id, "single"));

    const rest: RestCard[] = [];
    if (driftCount)
      rest.push({
        finding: "drift",
        kind: "Drift",
        title:
          groups.size === 1 && heldHere.length === 1
            ? `${heldHere[0].name} is held in Zitadel, not given by Syndra`
            : `Syndra and Zitadel disagree in ${groups.size} ${groups.size === 1 ? "place" : "places"}`,
        line:
          groups.size === 1 && heldHere.length === 1
            ? `${people(everyone)}${firstFound ? ` · found ${formatShortDate(firstFound)}` : ""}. They keep it until you decide.`
            : `${people(everyone)}. Open Drift to decide each one.`,
        open: { label: "Open on Drift", href: "/governance/drift" },
      });
    if (waiting.length)
      rest.push({
        finding: "send",
        kind: "Waiting for you to send",
        title: `${waiting.length} ${waiting.length === 1 ? "change is" : "changes are"} approved, not yet in Zitadel`,
        line: sendLines.slice(0, 2).join(" "),
        open: { label: "Waiting for you to send", href: "/governance/pending" },
      });
    if (single)
      rest.push({
        finding: "single",
        kind: "Single route",
        title: single.panel.title,
        line: `${single.panel.meta}.`,
        open: single.panel.open!,
      });
    if (unused && unused.count)
      rest.push({
        finding: "unused",
        kind: "Unused",
        title: unused.panel.title,
        line: `${listOf(unused.names.slice(0, 2))}${unused.names.length > 2 ? ` and ${unused.names.length - 2} more` : ""}.`,
        open: unused.panel.open!,
      });
    if (overlap)
      rest.push({
        finding: "overlap",
        kind: "Overlap",
        title: overlap.panel.title,
        line: `${overlap.panel.meta}.`,
        open: overlap.panel.open!,
      });

    const glance = m ? glancePanel(m, held, arrangement?.unused.projects ?? []) : null;
    return {
      counts,
      lit,
      marks,
      unused: new Set(unused?.ids ?? []),
      panel,
      rest,
      glance,
      held,
      ready: Boolean(drift.data && pending.data && catalog.data && (found.data || found.isError)),
    };
  }, [m, drift.data, pending.data, catalog.data, found.data, found.isError]);
}
