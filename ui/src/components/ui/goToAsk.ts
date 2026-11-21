"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";

import { buildModel } from "@/components/insights/model";
import { PART_KEYS, sentence, sentenceText, suggest, toParams, type Entity, type Question } from "@/components/insights/question";
import { useAnswer } from "@/components/insights/useAnswer";
import { UNKNOWN_ACCOUNT } from "@/components/names/Unresolved";
import { useTopology } from "@/lib/queries/useTopology";
import type { UserProfile } from "@/lib/types";

/** What reads as a question, not a name: its first word (A-26 §0, the board's own grammar). */
export const QUESTION_START = /^(who|what|path|compare|how|why|when|where|which)\b/i;

/** The sentences Insights can answer, as starts to type (A-26 §0a: "the six sentences Insights reads today"). */
export const GO_TO_ASKS = ["Who has ", "Path from ", "Compare ", "What if "] as const;

/** The link that opens Insights with a question already asked: the map's own words for it. */
export function insightsHref(q: Question): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(toParams(q))) if (value) params.set(key, value);
  return `/insights?${params}`;
}

/**
 * A question typed into Go to, answered in place by the very reads and sentences Insights uses
 * (`suggest`, `useAnswer`): one source, so Go to's answer and the map's are the same answer. Only a
 * question that names things on the map is asked; nothing is guessed. `text` empty: nothing is read.
 */
/** A read an answer is made from: the map (`topology`), Insights' own, or a person's access (a path, a comparison). */
const answerRead = (key: readonly unknown[]) => key[0] === "topology" || key[0] === "insights" || (key[0] === "users" && key[1] === "access");

export function useGoToAsk(text: string, users: UserProfile[] | undefined) {
  const on = text.trim() !== "";
  const topology = useTopology(on);
  const model = useMemo(() => (on && topology.data ? buildModel(topology.data) : null), [on, topology.data]);
  const people = useMemo(() => new Map((users ?? []).map((u) => [u.id, u])), [users]);
  const entities = useMemo<Entity[]>(
    () =>
      model
        ? [
            ...Array.from(model.byId.values(), (n) => ({ id: n.id, kind: n.kind, label: model.nameOf(n.id) })),
            ...(users ?? []).map((u) => ({ id: u.id, kind: "person" as const, label: u.name })),
          ]
        : [],
    [model, users],
  );
  const question = useMemo(
    () => (on && model ? (suggest(text, entities).find((q) => q.kind !== "follow") ?? null) : null),
    [on, model, text, entities],
  );
  const title = question && model ? sentenceText(sentence(question, (id) => model.nameOf(id), (id) => people.get(id)?.name ?? UNKNOWN_ACCOUNT)) : "";
  const answered = useAnswer(model, question, people, title);
  // What the question names, by the names the map gives them: Go to lists those places under the answer.
  const named = useMemo(() => {
    if (!question || !model) return [];
    const parts = question as Partial<Record<(typeof PART_KEYS)[number], string | null>>;
    return PART_KEYS.flatMap((key) => {
      const id = parts[key];
      return id ? [people.get(id)?.name ?? model.nameOf(id)] : [];
    });
  }, [question, model, people]);
  const client = useQueryClient();
  return {
    /** Nothing is said yet: the map, or the answer's own read, is still out. */
    working: on && ((!model && !topology.isError) || answered.computing),
    /** A read the answer needs failed: the map's, or the answer's own. */
    failed: on && (topology.isError || (answered.status !== null && !answered.computing)),
    /** Asks again for what failed among the answer's own reads (the map, Insights', a person's access), and only that. */
    retry: () => void client.resetQueries({ predicate: (query) => query.state.status === "error" && answerRead(query.queryKey) }),
    question,
    title,
    answer: answered.answer,
    named,
    href: question ? insightsHref(question) : null,
  };
}
