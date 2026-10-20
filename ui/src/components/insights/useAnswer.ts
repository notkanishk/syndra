"use client";

import { useMemo } from "react";

import type { Panel } from "@/components/insights/AnswerPanel";
import { compareAnswer, pathAnswer, whatIfAnswer, whoAnswer, type Answer } from "@/components/insights/answers";
import type { Model } from "@/components/insights/model";
import type { Question } from "@/components/insights/question";
import { useInsightsWhatIf, useInsightsWho, type WhatIfChange } from "@/lib/queries/useInsights";
import type { CatalogUser } from "@/lib/queries/useCatalogUsers";
import { useUserAccess, type UserAccessView } from "@/lib/queries/useUsers";
import { UNKNOWN_ACCOUNT } from "@/components/names/Unresolved";

/** The reference the Insights reads take for a role or a bundle on the map. */
function refOf(m: Model, id: string): string | null {
  const n = m.byId.get(id);
  if (n?.kind === "role") return n.project_id && n.meta?.role_key ? `role:${n.project_id}:${n.meta.role_key}` : null;
  return n?.kind === "bundle" ? `bundle:${id.replace(/^bundle:/, "")}` : null;
}

/** How a person holds one role: the reasons on their access page, or none. */
function reasonsFor(view: UserAccessView | null | undefined, projectId: string, roleKey: string) {
  const project = view?.projects.find((p) => p.project_id === projectId);
  return [...(project?.source_roles ?? []), ...(project?.derived_roles ?? [])].find((r) => r.role_key === roleKey)?.reasons;
}

/** What a question that is not a plain follow needs to be answered: each read, and what it says. */
export function useAnswer(model: Model | null, q: Question | null, people: Map<string, CatalogUser>, title: string) {
  const person = q?.kind === "path" || q?.kind === "compare" ? q.person : "";
  const other = q?.kind === "compare" ? q.other : "";
  const a = useUserAccess(person);
  const b = useUserAccess(other);

  const who = useInsightsWho(
    q?.kind === "who" && model ? refOf(model, q.has) : null,
    q?.kind === "who" && q.without && model ? refOf(model, q.without) : null,
  );

  const change = useMemo<WhatIfChange | null>(() => {
    if (!model || (q?.kind !== "gone" && q?.kind !== "gives")) return null;
    const bundle_id = q.bundle.replace(/^bundle:/, "");
    if (q.kind === "gone") return { kind: "bundle_gone", bundle_id };
    const role = model.byId.get(q.role);
    return role?.project_id && role.meta?.role_key
      ? { kind: "bundle_gives_also", bundle_id, project_id: role.project_id, role_key: role.meta.role_key }
      : null;
  }, [model, q]);
  const whatIf = useInsightsWhatIf(change);

  return useMemo(() => {
    if (!model || !q || q.kind === "follow") return { answer: null as Answer | null, status: null as Panel | null, pin: null as string | null };
    const name = (id: string, fallback?: string) => people.get(id)?.name ?? fallback ?? UNKNOWN_ACCOUNT;
    const role = (id: string) => model.byId.get(id);

    const settled = (): Answer | "loading" | "failed" => {
      switch (q.kind) {
        case "path": {
          if (a.isError) return "failed";
          if (!a.data) return "loading";
          const r = role(q.role)!;
          return pathAnswer(
            model,
            { id: q.person, name: name(q.person, a.data.user.name), meta: people.get(q.person)?.title || undefined },
            q.role,
            reasonsFor(a.data, r.project_id ?? "", r.meta?.role_key ?? ""),
          );
        }
        case "compare": {
          if (a.isError || b.isError) return "failed";
          if (!a.data || !b.data) return "loading";
          const r = role(q.role)!;
          const side = (id: string, v: UserAccessView) => ({
            id,
            name: name(id, v.user.name),
            reasons: reasonsFor(v, r.project_id ?? "", r.meta?.role_key ?? ""),
          });
          return compareAnswer(model, side(q.person, a.data), side(q.other, b.data), q.role);
        }
        case "who":
          return who.isError ? "failed" : who.data ? whoAnswer(model, q.has, q.without, who.data) : "loading";
        default:
          return whatIf.isError || !change
            ? "failed"
            : whatIf.data
              ? whatIfAnswer(model, { bundle: q.bundle, role: q.kind === "gives" ? q.role : undefined }, whatIf.data)
              : "loading";
      }
    };

    const result = settled();
    const pin = q.kind === "path" || q.kind === "compare" ? q.role : q.kind === "who" ? q.has : q.bundle;
    if (typeof result !== "string") return { answer: result, status: null, pin };
    // Nothing is said about an answer that has not been read.
    const line = result === "loading" ? "Working it out." : "Couldn't load this answer.";
    return {
      answer: null,
      status: { tone: "neutral", eyebrow: "Asking", title, lines: [{ tone: "ink", text: line }] } satisfies Panel,
      pin,
    };
  }, [model, q, people, title, a.data, a.isError, b.data, b.isError, who.data, who.isError, whatIf.data, whatIf.isError, change]);
}
