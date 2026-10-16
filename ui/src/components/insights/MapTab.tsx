"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { AnswerPanel, type Panel } from "@/components/insights/AnswerPanel";
import { LaneMap } from "@/components/insights/LaneMap";
import { NodeList } from "@/components/insights/NodeList";
import {
  buildModel,
  crossing,
  describe,
  kindMeta,
  openOf,
  paint,
  pinLine,
  pinTones,
  reach,
  type Highlight,
} from "@/components/insights/model";
import { nodeIds, sentence, sentenceText, toParams, type Entity, type Question } from "@/components/insights/question";
import { QuestionSeat } from "@/components/insights/QuestionSeat";
import { RestCards } from "@/components/insights/RestCards";
import { useAnswer } from "@/components/insights/useAnswer";
import { FINDINGS, MAX_PINS, type Finding, type InsightsUrl } from "@/components/insights/useInsightsUrl";
import { useFindings } from "@/components/insights/useFindings";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { PILL } from "@/components/ui/Button";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { FilterPills, Segmented, Select } from "@/components/ui/Select";
import { Unfold } from "@/components/ui/Unfold";
import { useCatalogUsers } from "@/lib/queries/useCatalogUsers";
import { useTopology } from "@/lib/queries/useTopology";
import { useIsPhone, useIsTouch } from "@/lib/useViewport";

const DOT: Record<Finding, string> = { drift: "bg-danger", send: "bg-warn", single: "bg-warn", unused: "bg-muted", overlap: "bg-muted" };
const CHIP_WORDS: Record<Finding, [string, string]> = {
  drift: ["drift", "drift"],
  send: ["waiting for you to send", "waiting for you to send"],
  single: ["single route", "single routes"],
  unused: ["unused", "unused"],
  overlap: ["overlap", "overlaps"],
};
const PHONE_NOTE = "The map draws this on a wider screen. The link keeps the question, so it opens there as you left it.";
const THREADS = ["", "One thread", "Two threads, side by side", "Three threads, side by side"];

/**
 * The Map tab's four seats and nothing else: the question, the lens, the read
 * line, and the answer. Every state is in the link (`url`), so the answer is
 * the same wherever the link is opened.
 */
export function MapTab({ url }: { url: InsightsUrl }) {
  const topology = useTopology();
  const touch = useIsTouch();
  const phone = useIsPhone();
  const model = useMemo(() => (topology.data ? buildModel(topology.data) : null), [topology.data]);
  const findings = useFindings(model);
  const catalog = useCatalogUsers();
  const [keysOpen, setKeysOpen] = useState(false);
  const [text, setText] = useState("");
  const input = useRef<HTMLInputElement | null>(null);
  const wantInput = useRef(false);

  const people = useMemo(() => new Map(catalog.data.map((u) => [u.id, u])), [catalog.data]);
  const entities = useMemo<Entity[]>(
    () => [
      ...(model ? Array.from(model.byId.values(), (n) => ({ id: n.id, kind: n.kind, label: model.nameOf(n.id) })) : []),
      ...catalog.data.map((u) => ({ id: u.id, kind: "person" as const, label: u.name })),
    ],
    [model, catalog.data],
  );
  const nodeName = (id: string) => model?.nameOf(id) ?? id;
  const personName = (id: string) => people.get(id)?.name ?? "Unknown account";

  // A question naming something that is not on the map is not a question.
  const question: Question | null = model && url.question && nodeIds(url.question).every((id) => model.byId.has(id)) ? url.question : null;
  const focus = question?.kind === "follow" ? question.node : null;
  const asked = question && question.kind !== "follow" ? question : null;
  const answer = useAnswer(model, asked, people, asked ? sentenceText(sentence(asked, nodeName, personName)) : "");
  const target = focus ?? answer.pin;
  const pins = model ? url.pins.filter((id) => model.byId.has(id)) : [];
  const finding = url.finding && findings.counts[url.finding] > 0 ? url.finding : null;
  const tones = pinTones(pins, focus);

  const highlight = useMemo<Highlight | null>(() => {
    if (!model) return null;
    const h: Highlight = { nodes: new Map(), threads: new Map() };
    if (finding) {
      const lit = findings.lit(finding);
      for (const n of lit.nodes) h.nodes.set(n.id, n.tone);
      for (const t of lit.threads) h.threads.set(t.id, t.tone);
      for (const n of lit.nodes) if (n.down) paint(h, reach(model, n.id, ["down"]), "ink");
    } else if (asked) {
      if (answer.answer?.highlight) {
        answer.answer.highlight.nodes.forEach((tone, id) => h.nodes.set(id, tone));
        answer.answer.highlight.threads.forEach((tone, id) => h.threads.set(id, tone));
      }
      for (const p of pins) paint(h, reach(model, p), tones.get(p)!);
    } else if (focus) {
      paint(h, reach(model, focus), "accent");
      for (const p of pins) if (p !== focus) paint(h, reach(model, p), tones.get(p)!);
    } else {
      for (const p of pins) paint(h, reach(model, p), tones.get(p)!);
    }
    return h.nodes.size ? h : null;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [model, finding, findings.lit, focus, asked, answer.answer, pins.join()]);

  const held = (id: string) => findings.held.get(id) ?? null;
  let panel: Panel | null = null;
  if (model && finding) panel = findings.panel(finding);
  else if (model && asked) panel = answer.answer?.panel ?? answer.status;
  else if (model && focus)
    panel = {
      tone: "accent",
      eyebrow: pins.length ? `Following · ${pins.length} pinned` : "Following",
      title: model.nameOf(focus),
      meta: kindMeta(model, focus, held(focus)),
      lines: describe(model, focus).map((t) => ({ tone: "accent", text: t })),
      open: openOf(model, focus) ?? undefined,
    };
  else if (model && pins.length) {
    const cross = crossing(model, pins);
    panel = {
      tone: "accent",
      eyebrow: `Pinned · ${pins.length}`,
      title: THREADS[pins.length],
      meta: "Each keeps its own shade",
      lines: [
        ...pins.map((p) => ({ tone: tones.get(p)!, text: pinLine(model, p, held(p)) })),
        ...(cross ? [{ tone: "ink" as const, text: cross }] : []),
      ],
    };
  } else if (model && !phone && findings.ready) panel = findings.glance;

  const ask = (q: Question | null) => url.set({ ...toParams(q), finding: null });
  const pick = (id: string | null) => ask(id ? { kind: "follow", node: id } : null);
  const back = () => {
    if (finding) url.set({ finding: null });
    else if (question) ask(null);
    else if (pins.length) url.set({ pin: null });
  };
  const pin = () => target && !pins.includes(target) && pins.length < MAX_PINS && url.set({ pin: [...pins, target].join(",") });
  const unpin = (id: string) => url.set({ pin: pins.filter((p) => p !== id).join(",") || null });
  const togglePin = () => (target && pins.includes(target) ? unpin(target) : pin());

  // The keys act on whatever is current, so the listener is rebound each render.
  const keyRef = useRef<(e: KeyboardEvent) => void>(() => {});
  keyRef.current = (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "Escape") {
      if (keysOpen) setKeysOpen(false);
      else if (text) setText("");
      else back();
      return;
    }
    if ((e.target as HTMLElement | null)?.closest("input, textarea, select, [contenteditable]")) return;
    const k = e.key.toLowerCase();
    if (e.key === "/") {
      e.preventDefault();
      // A question holds the seat as chips; asking again clears it first.
      if (question) {
        wantInput.current = true;
        ask(null);
      } else input.current?.focus();
    } else if (e.key === "?") setKeysOpen((o) => !o);
    else if (k === "p" && target) togglePin();
    else if (model && focus && e.key.startsWith("Arrow")) {
      const node = model.byId.get(focus)!;
      const lane = model.lanes[node.kind];
      const here = lane.findIndex((n) => n.id === focus);
      const along = (ts: Array<{ rule: boolean; from: string; to: string }> | undefined, end: "from" | "to") =>
        (ts?.find((t) => !t.rule) ?? ts?.[0])?.[end];
      const next =
        e.key === "ArrowUp" ? lane[here - 1]?.id : e.key === "ArrowDown" ? lane[here + 1]?.id
        : e.key === "ArrowRight" ? along(model.out.get(focus), "to") : e.key === "ArrowLeft" ? along(model.inn.get(focus), "from") : undefined;
      e.preventDefault();
      if (next) pick(next);
    }
  };
  useEffect(() => {
    if (wantInput.current && input.current) {
      wantInput.current = false;
      input.current.focus();
    }
  });
  useEffect(() => {
    const on = (e: KeyboardEvent) => keyRef.current(e);
    document.addEventListener("keydown", on);
    return () => document.removeEventListener("keydown", on);
  }, []);

  const view = touch ? "list" : url.view;
  const chips = FINDINGS.filter((f) => findings.counts[f] > 0);

  return (
    <>
      <div className="flex flex-col gap-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2.5">
          <QuestionSeat
            entities={entities}
            question={question}
            node={nodeName}
            person={personName}
            text={text}
            onText={setText}
            onAsk={ask}
            onClear={() => ask(null)}
            inputRef={input}
          />
          {phone ? (
            <div className="w-full">
              <Select aria-label="Lens" defaultValue="people">
                <option value="people">Lens: People</option>
              </Select>
            </div>
          ) : (
            <div className="flex items-center gap-3">
              <FilterPills label="Lens" value="people" onChange={() => {}} options={[{ value: "people", label: "People" }]} />
              {!touch && (
                <Segmented
                  size="sm"
                  label="View"
                  value={url.view}
                  onChange={(v) => url.set({ view: v === "list" ? "list" : null })}
                  options={[
                    { value: "map", label: "Map" },
                    { value: "list", label: "List" },
                  ]}
                />
              )}
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          {topology.data && (
            <ReadFreshness
              subject="The access map"
              state={{
                readAt: topology.dataUpdatedAt ? new Date(topology.dataUpdatedAt).toISOString() : null,
                current: topology.isError ? false : undefined,
              }}
            />
          )}
          <span className="flex-1" />
          {!phone &&
            chips.map((f) => (
              <button
                key={f}
                type="button"
                aria-pressed={finding === f}
                onClick={() => url.set({ ...toParams(null), finding: finding === f ? null : f })}
                className={`motion-tint inline-flex items-center gap-2 rounded-pill font-semibold ${PILL.sm} ${finding === f ? "bg-tint-3" : "bg-tint-2"}`}
              >
                <span aria-hidden className={`size-1.5 rounded-pill ${DOT[f]}`} />
                {findings.counts[f]} {CHIP_WORDS[f][findings.counts[f] === 1 ? 0 : 1]}
              </button>
            ))}
        </div>
      </div>

      <Unfold open={Boolean(panel)}>
        {panel && (
          <AnswerPanel
            key={`${finding}|${JSON.stringify(question)}|${pins.join()}`}
            panel={panel}
            phone={phone}
            onBack={question || finding || pins.length ? back : undefined}
            pins={pins.map((id) => ({ id, name: model!.nameOf(id), tone: tones.get(id)! }))}
            onUnpin={unpin}
            onPin={!finding && target && !pins.includes(target) && pins.length < MAX_PINS ? pin : undefined}
            onlyPinned={focus && pins.length >= 2 ? () => ask(null) : undefined}
          />
        )}
      </Unfold>
      {phone && panel && question && !finding && <p className="text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">{PHONE_NOTE}</p>}

      {phone && !panel && findings.ready && <RestCards cards={findings.rest} />}

      <ListStates
        isLoading={topology.isLoading}
        error={topology.data ? null : topology.error}
        isEmpty={(topology.data?.nodes.length ?? 0) === 0}
        onRetry={() => topology.refetch()}
        errorTitle="Couldn't load Insights."
        skeleton={<RowSkeleton rows={4} avatar={false} label="Loading Insights" />}
        empty={
          <EmptyState
            title="Nothing to show yet."
            guidance="Items appear here once Syndra has at least one project and one role."
          />
        }
      >
        {model && !phone && view === "map" && (
          <LaneMap
            model={model}
            highlight={highlight}
            focusId={focus}
            count={held}
            unused={findings.unused}
            marks={findings.marks}
            keysOpen={keysOpen}
            onKeys={() => setKeysOpen((o) => !o)}
            onPick={pick}
          />
        )}
        {model && !phone && view === "list" && (
          <NodeList model={model} highlight={highlight} focusId={focus} unused={findings.unused} onPick={pick} />
        )}
      </ListStates>
    </>
  );
}
