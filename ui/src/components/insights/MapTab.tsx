"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { AnswerPanel, type Panel } from "@/components/insights/AnswerPanel";
import { LaneMap, type Mark } from "@/components/insights/LaneMap";
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
import { RestCards } from "@/components/insights/RestCards";
import { NodeGlyph } from "@/components/insights/tone";
import { MAX_PINS, type Finding, type InsightsUrl } from "@/components/insights/useInsightsUrl";
import { useFindings } from "@/components/insights/useFindings";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { AnchoredPanel } from "@/components/ui/Anchored";
import { PILL } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { FilterPills, Segmented, Select } from "@/components/ui/Select";
import { Unfold } from "@/components/ui/Unfold";
import { useTopology } from "@/lib/queries/useTopology";
import { useIsPhone, useIsTouch } from "@/lib/useViewport";

const DOT: Record<Finding, string> = { drift: "bg-danger", send: "bg-warn", unused: "bg-muted" };
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
  const [keysOpen, setKeysOpen] = useState(false);
  const [text, setText] = useState("");
  const input = useRef<HTMLInputElement | null>(null);
  const seat = useRef<HTMLDivElement | null>(null);

  const focus = model && url.q && model.byId.has(url.q) ? url.q : null;
  const pins = model ? url.pins.filter((id) => model.byId.has(id)) : [];
  const finding = url.finding && findings.counts[url.finding] > 0 ? url.finding : null;
  const tones = pinTones(pins, focus);

  const unused = useMemo(() => new Set(findings.nodes.unused), [findings.nodes.unused]);
  const marks = useMemo(() => {
    const out = new Map<string, Mark[]>();
    for (const mark of ["drift", "send"] as const)
      for (const id of findings.nodes[mark]) out.set(id, [...(out.get(id) ?? []), mark]);
    return out;
  }, [findings.nodes]);

  const highlight = useMemo<Highlight | null>(() => {
    if (!model) return null;
    const h: Highlight = { nodes: new Map(), threads: new Map() };
    if (finding) {
      const tone = finding === "drift" ? "danger" : finding === "send" ? "warn" : "ink";
      for (const id of findings.nodes[finding]) h.nodes.set(id, tone);
      if (finding !== "unused") for (const id of findings.nodes[finding]) paint(h, reach(model, id, ["down"]), "ink");
    } else if (focus) {
      paint(h, reach(model, focus), "accent");
      for (const p of pins) if (p !== focus) paint(h, reach(model, p), tones.get(p)!);
    } else {
      for (const p of pins) paint(h, reach(model, p), tones.get(p)!);
    }
    return h.nodes.size ? h : null;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [model, finding, focus, pins.join(), findings.nodes]);

  const held = (id: string) => findings.held.get(id) ?? null;
  let panel: Panel | null = null;
  if (model && finding) panel = findings.panel(finding);
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
  }

  const pick = (id: string | null) => url.set({ q: id, finding: null });
  const back = () => {
    if (finding) url.set({ finding: null });
    else if (focus) url.set({ q: null });
    else if (pins.length) url.set({ pin: null });
  };
  const pin = () => focus && pins.length < MAX_PINS && url.set({ pin: [...pins, focus].join(",") });
  const unpin = (id: string) => url.set({ pin: pins.filter((p) => p !== id).join(",") || null });
  const togglePin = () => (focus && pins.includes(focus) ? unpin(focus) : pin());

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
      input.current?.focus();
    } else if (e.key === "?") setKeysOpen((o) => !o);
    else if (k === "p" && focus) togglePin();
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
    const on = (e: KeyboardEvent) => keyRef.current(e);
    document.addEventListener("keydown", on);
    return () => document.removeEventListener("keydown", on);
  }, []);

  const needle = text.trim().toLowerCase();
  const matches = model && needle ? Array.from(model.byId.keys()).filter((id) => model.nameOf(id).toLowerCase().includes(needle)).slice(0, 8) : [];
  const choose = (id: string) => {
    pick(id);
    setText("");
  };

  const view = touch ? "list" : url.view;
  const chips = (["drift", "send", "unused"] as const).filter((f) => findings.counts[f] > 0);
  const chipWords: Record<Finding, string> = { drift: "drift", send: "waiting for you to send", unused: "unused" };

  return (
    <>
      <div className="flex flex-col gap-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2.5">
          <div ref={seat} className="relative min-w-0 flex-[1_1_320px]">
            <Input
              ref={input}
              value={text}
              onChange={(e) => setText(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && matches[0] && choose(matches[0])}
              placeholder="Ask about a person, a door, a role, or find anything"
              aria-label="Ask the map"
            />
            <AnchoredPanel
              anchor={seat}
              open={matches.length > 0}
              width="anchor"
              expectHeight={360}
              className="settle-in overflow-hidden rounded-panel border border-line-strong bg-surface-2 shadow-popover"
            >
              {matches.map((id) => (
                <button
                  key={id}
                  type="button"
                  onClick={() => choose(id)}
                  className="motion-tint flex min-h-[44px] w-full items-center gap-2.5 px-3 text-left text-[13.5px] hover:bg-[var(--hover)] desktop:min-h-0 desktop:py-2"
                >
                  <NodeGlyph kind={model!.byId.get(id)!.kind} />
                  <span className="truncate">{model!.nameOf(id)}</span>
                </button>
              ))}
            </AnchoredPanel>
          </div>
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
                onClick={() => url.set({ finding: finding === f ? null : f, q: null })}
                className={`motion-tint inline-flex items-center gap-2 rounded-pill font-semibold ${PILL.sm} ${finding === f ? "bg-tint-3" : "bg-tint-2"}`}
              >
                <span aria-hidden className={`size-1.5 rounded-pill ${DOT[f]}`} />
                {findings.counts[f]} {chipWords[f]}
              </button>
            ))}
        </div>
      </div>

      <Unfold open={Boolean(panel)}>
        {panel && (
          <AnswerPanel
            key={`${finding}|${focus}|${pins.join()}`}
            panel={panel}
            onBack={back}
            pins={pins.map((id) => ({ id, name: model!.nameOf(id), tone: tones.get(id)! }))}
            onUnpin={unpin}
            onPin={focus && !pins.includes(focus) && pins.length < MAX_PINS ? pin : undefined}
            onlyPinned={focus && pins.length >= 2 ? () => url.set({ q: null }) : undefined}
          />
        )}
      </Unfold>

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
            unused={unused}
            marks={marks}
            keysOpen={keysOpen}
            onKeys={() => setKeysOpen((o) => !o)}
            onPick={pick}
          />
        )}
        {model && !phone && view === "list" && (
          <NodeList model={model} highlight={highlight} focusId={focus} unused={unused} onPick={pick} />
        )}
      </ListStates>
    </>
  );
}
