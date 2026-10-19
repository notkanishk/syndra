"use client";

import { ArriveList } from "@/components/ui/ArriveList";
import { LANES, type Highlight, type Model } from "@/components/insights/model";
import { NodeGlyph, nodeSurface } from "@/components/insights/tone";

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
}: {
  model: Model;
  highlight: Highlight | null;
  focusId: string | null;
  unused: Set<string>;
  onPick: (id: string | null) => void;
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
          <ArriveList className="grid gap-2.5 tablet:grid-cols-2 desktop:grid-cols-3">
            {model.lanes[lane.kind].map((node) => {
              const tone = highlight?.nodes.get(node.id);
              return (
                <div key={node.id}>
                  <button
                    type="button"
                    aria-pressed={node.id === focusId}
                    onClick={() => onPick(node.id === focusId ? null : node.id)}
                    className="motion-tint flex min-h-[44px] w-full items-center gap-3 rounded-block border px-4 py-3 text-left"
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
