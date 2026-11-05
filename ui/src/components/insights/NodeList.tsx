"use client";

import { ArriveList } from "@/lib/motion";
import { LANES, type Highlight, type Model } from "@/components/insights/model";
import { NodeGlyph, nodeSurface } from "@/components/insights/tone";
import { Card, CardHeader } from "@/components/ui/Card";

const BLURB: Record<string, string> = {
  bundle: "Sets of roles given together.",
  role: "One named kind of access, inside a project.",
  project: "A machine or area with its own set of roles.",
  application: "Something that reads who has what.",
};

function connections(count: number): string {
  return count === 0 ? "nothing connects to this" : `${count} ${count === 1 ? "connection" : "connections"}`;
}

/**
 * The browsable list: everything, grouped as the map's lanes are, with no
 * typing needed. It is what a tablet shows instead of the map, and what the
 * map's List switch gives on a desktop. Picking one answers in the panel, and
 * what the answer does not reach steps back, as it does on the map.
 */
export function NodeList({
  model,
  highlight,
  focusId,
  unused,
  onPick,
  startSettled = false,
}: {
  model: Model;
  highlight: Highlight | null;
  focusId: string | null;
  unused: Set<string>;
  onPick: (id: string | null) => void;
  /** The reader switched to this view from the other: the page has arrived already, so its nodes are simply there. */
  startSettled?: boolean;
}) {
  const degree = (id: string) => (model.out.get(id)?.length ?? 0) + (model.inn.get(id)?.length ?? 0);

  return (
    <div className="flex flex-col gap-7">
      {LANES.filter((lane) => model.lanes[lane.kind].length > 0).map((lane) => (
        <section key={lane.kind}>
          <div className="mb-2.5 flex flex-wrap items-baseline gap-2.5">
            <h2 className="type-label">{lane.title}</h2>
            <span className="text-[13px] text-faint">
              {model.lanes[lane.kind].length} · {BLURB[lane.kind]}
            </span>
          </div>
          <ArriveList className="grid gap-2.5 tablet:grid-cols-2 desktop:grid-cols-3" startSettled={startSettled}>
            {model.lanes[lane.kind].map((node) => {
              const tone = highlight?.nodes.get(node.id);
              return (
                <div key={node.id}>
                  <button
                    type="button"
                    aria-pressed={node.id === focusId}
                    onClick={() => onPick(node.id === focusId ? null : node.id)}
                    className="motion-tint flex min-h-[var(--tap)] w-full items-center gap-3 rounded-block border px-4 py-3 text-left"
                    data-dim={(Boolean(highlight) && !tone) || undefined}
                    style={nodeSurface(tone, node.id === focusId, unused.has(node.id), Boolean(highlight) && !tone)}
                  >
                    <NodeGlyph kind={node.kind} />
                    <span className="min-w-0 flex-1">
                      <span className="block text-[14.5px] font-semibold">{model.nameOf(node.id)}</span>
                      <span className="block text-[12.5px] text-faint">{connections(degree(node.id))}</span>
                    </span>
                  </button>
                </div>
              );
            })}
          </ArriveList>
        </section>
      ))}
    </div>
  );
}

/**
 * Below tablet: the nodes as a plain list, each lane A to Z with its people
 * (RULES Phone, 87c). The counts are the map's own: observed holders from
 * Zitadel, or what the lens reads. A role not yet read has no number, not a
 * zero. The answer sits above, in the panel.
 */
export function PhoneNodes({
  model,
  focusId,
  count,
  unit = "",
  onPick,
}: {
  model: Model;
  focusId: string | null;
  count: (id: string) => number | null;
  unit?: string;
  onPick: (id: string | null) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      {LANES.filter((lane) => model.lanes[lane.kind].length > 0).map((lane) => (
        <Card key={lane.kind}>
          <CardHeader title={lane.title} count={model.lanes[lane.kind].length} />
          {model.lanes[lane.kind].map((node) => {
            const n = count(node.id);
            const name = model.nameOf(node.id);
            return (
              <button
                key={node.id}
                type="button"
                data-node={node.id}
                aria-pressed={node.id === focusId}
                onClick={() => onPick(node.id === focusId ? null : node.id)}
                className="row-divider motion-tint flex min-h-[var(--tap)] w-full items-center gap-3 px-5 py-3 text-left aria-pressed:bg-accent-soft"
              >
                <NodeGlyph kind={node.kind} />
                <span className="min-w-0 flex-1 text-[14.5px] font-semibold">{name}</span>
                {n != null && (
                  <span className="text-[13px] tabular-nums text-muted">
                    {n === 1 ? "1 person" : `${n} people`}
                    {unit ? ` ${unit}` : ""}
                  </span>
                )}
              </button>
            );
          })}
        </Card>
      ))}
    </div>
  );
}
