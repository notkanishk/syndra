"use client";

import { useEffect, useMemo, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented } from "@/components/ui/Select";
import { useTopology, type TopologyNode } from "@/lib/queries/useTopology";
import { useDebounce } from "@/lib/useDebounce";

type Depth = "1" | "2" | "all";

/**
 * S5 · Automation › Access map.
 *
 * The old graph was unreadable because it drew everything at once. The fix is
 * not a better force layout — it is not drawing everything. Pick one node and
 * the map answers exactly two questions, left to right: what feeds this, and
 * what does this feed.
 *
 * Node shapes are a separate namespace from Access source and deliberately do
 * not borrow that component: project, role, bundle, rule and app are kinds of
 * thing, not reasons somebody holds access.
 */
export default function AccessMapPage() {
  const topology = useTopology();
  const [focusId, setFocusId] = useState<string | null>(null);
  const [depth, setDepth] = useState<Depth>("1");
  const [query, setQuery] = useState("");
  const search = useDebounce(query, 200).trim().toLowerCase();

  // Memoised so the effect below doesn't see a new array identity per render.
  const nodes = useMemo(() => topology.data?.nodes ?? [], [topology.data]);
  const edges = useMemo(() => topology.data?.edges ?? [], [topology.data]);

  // Default to the most connected node: the map should open on something worth
  // looking at rather than on whatever sorted first.
  useEffect(() => {
    if (focusId || nodes.length === 0) return;
    const degree = new Map<string, number>();
    for (const edge of edges) {
      degree.set(edge.source, (degree.get(edge.source) ?? 0) + 1);
      degree.set(edge.target, (degree.get(edge.target) ?? 0) + 1);
    }
    const best = [...nodes].sort((a, b) => (degree.get(b.id) ?? 0) - (degree.get(a.id) ?? 0))[0];
    if (best) setFocusId(best.id);
  }, [nodes, edges, focusId]);

  const byId = useMemo(() => new Map(nodes.map((node) => [node.id, node])), [nodes]);
  const focus = focusId ? byId.get(focusId) : undefined;

  const { incoming, outgoing, secondHop } = useMemo(() => {
    if (!focus) return { incoming: [], outgoing: [], secondHop: 0 };

    const inbound = edges.filter((edge) => edge.target === focus.id);
    const outbound = edges.filter((edge) => edge.source === focus.id);

    const neighbours = new Set([
      ...inbound.map((edge) => edge.source),
      ...outbound.map((edge) => edge.target),
    ]);
    const second = edges.filter(
      (edge) =>
        (neighbours.has(edge.source) && edge.target !== focus.id && !neighbours.has(edge.target)) ||
        (neighbours.has(edge.target) && edge.source !== focus.id && !neighbours.has(edge.source)),
    ).length;

    return {
      incoming: inbound.map((edge) => ({
        node: byId.get(edge.source),
        label: edge.label,
        viaRule: edge.kind === "rule",
      })),
      outgoing: outbound.map((edge) => ({
        node: byId.get(edge.target),
        label: edge.label,
        viaRule: edge.kind === "rule",
      })),
      secondHop: second,
    };
  }, [focus, edges, byId]);

  const matches = search
    ? nodes.filter((node) => node.label.toLowerCase().includes(search)).slice(0, 8)
    : [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader title="Access map" meta="One node at a time, and what it touches." />

      <Card className="flex min-h-[560px] flex-col md:flex-row">
        <div className="flex w-full flex-none flex-col gap-4 border-b border-line p-4 md:w-[228px] md:border-b-0 md:border-r">
          <div className="relative">
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Find a node…"
              aria-label="Find a node"
              className="rounded-pill py-2.5 text-[14px]"
            />
            {matches.length > 0 && (
              <div className="absolute left-0 right-0 top-[calc(100%+6px)] z-20 overflow-hidden rounded-block border border-line-strong bg-surface-2 shadow-popover">
                {matches.map((node) => (
                  <button
                    key={node.id}
                    type="button"
                    onClick={() => {
                      setFocusId(node.id);
                      setQuery("");
                    }}
                    className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-[13.5px] hover:bg-[var(--hover)]"
                  >
                    <NodeGlyph kind={node.kind} />
                    <span className="truncate">{node.label}</span>
                  </button>
                ))}
              </div>
            )}
          </div>

          <div>
            <div className="mb-2.5 type-label">Show</div>
            <div className="flex flex-col gap-2.5 text-[14px]">
              {(
                [
                  ["project", "Projects"],
                  ["role", "Roles"],
                  ["bundle", "Bundles"],
                  ["application", "Apps"],
                ] as const
              ).map(([kind, label]) => (
                <div key={kind} className="flex items-center gap-2.5">
                  <NodeGlyph kind={kind} />
                  <span className={kind === "application" ? "text-faint" : undefined}>{label}</span>
                </div>
              ))}
              {/* A rule is an edge, not a node: it is the dashed link between
                  two roles rather than a thing you can hold. */}
              <div className="flex items-center gap-2.5">
                <span className="h-3.5 w-3.5 flex-none rounded-[5px] border-[1.5px] border-dashed border-ink/70" />
                <span>Automatic rules</span>
              </div>
            </div>
          </div>

          <div>
            <div className="mb-2.5 type-label">Depth</div>
            <Segmented<Depth>
              label="How many hops to draw"
              size="sm"
              value={depth}
              onChange={setDepth}
              options={[
                { value: "1", label: "1 hop" },
                { value: "2", label: "2" },
                { value: "all", label: "All" },
              ]}
            />
          </div>

          <div className="mt-auto text-[13px] leading-[1.55] text-faint">
            {nodes.length} nodes, {edges.length} edges in total. Drawing them all is what made this
            screen useless.
          </div>
        </div>

        <div className="min-w-0 flex-1 p-[26px]">
          {topology.isLoading ? (
            <RowSkeleton rows={4} avatar={false} label="Loading the access map" />
          ) : topology.error ? (
            <ErrorState
              title="Couldn't load the access map."
              error={topology.error}
              onRetry={() => topology.refetch()}
            />
          ) : !focus ? (
            <div className="type-empty-title">Nothing to draw yet.</div>
          ) : (
            <div className="flex flex-col gap-5">
              <div className="flex flex-wrap items-baseline gap-3.5">
                <h2 className="font-display text-[30px] font-medium tracking-[-0.02em]">
                  {focus.label}
                </h2>
                <span className="text-[14px] text-faint">
                  {incoming.length} in · {outgoing.length} out
                </span>
              </div>

              <div className="flex flex-wrap items-stretch gap-0">
                <NodeColumn
                  title="Feeds in"
                  entries={incoming}
                  empty="Nothing produces this."
                  onFocus={setFocusId}
                />
                <Rail direction="in" />
                <div className="flex w-[250px] flex-none items-center">
                  <div className="w-full rounded-card border-[1.5px] border-accent bg-accent-soft p-5">
                    <NodeGlyph kind={focus.kind} large />
                    <div className="mt-2.5 font-display text-[24px] font-semibold leading-[1.1]">
                      {focus.label}
                    </div>
                    <div className="mt-1 text-[13.5px] text-muted">
                      {focus.kind}
                      {focus.project_id ? " · " : ""}
                      {focus.project_id && <Mono>{focus.project_id}</Mono>}
                    </div>
                  </div>
                </div>
                <Rail direction="out" />
                <NodeColumn
                  title="Feeds out"
                  entries={outgoing}
                  empty="Nothing reads this."
                  onFocus={setFocusId}
                  footer={
                    depth === "1" && secondHop > 0 ? (
                      <button
                        type="button"
                        onClick={() => setDepth("2")}
                        className="rounded-block border border-dashed border-line-strong px-4 py-3 text-center text-[13.5px] text-faint hover:text-ink"
                      >
                        Expand to 2 hops · {secondHop} more nodes
                      </button>
                    ) : null
                  }
                />
              </div>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
}

function NodeColumn({
  title,
  entries,
  empty,
  onFocus,
  footer,
}: {
  title: string;
  entries: Array<{ node?: TopologyNode; label: string; viaRule: boolean }>;
  empty: string;
  onFocus: (id: string) => void;
  footer?: React.ReactNode;
}) {
  return (
    <div className="flex min-w-[240px] flex-1 flex-col gap-3">
      <div className="type-label">{title}</div>
      {entries.length === 0 ? (
        <div className="text-[13.5px] text-faint">{empty}</div>
      ) : (
        entries.slice(0, 6).map(({ node, label, viaRule }, index) =>
          node ? (
            <button
              key={`${node.id}-${index}`}
              type="button"
              onClick={() => onFocus(node.id)}
              // A dashed edge means an automatic rule produced this link — the
              // same dashed language the Access source chip uses for "nobody
              // clicked it". The node itself is still a role or a project.
              className={`flex items-center gap-3 rounded-block bg-surface-1 px-4 py-3.5 text-left transition-colors hover:bg-[var(--hover)] ${
                viaRule ? "border border-dashed border-ink/25" : "border border-line"
              }`}
            >
              <NodeGlyph kind={node.kind} />
              <span className="min-w-0">
                <span className="block truncate text-[14.5px] font-semibold">{node.label}</span>
                <span className="block truncate text-[12.5px] text-faint">
                  {viaRule ? "automatic rule" : node.kind}
                  {label ? ` · ${label}` : ""}
                </span>
              </span>
            </button>
          ) : null,
        )
      )}
      {entries.length > 6 && (
        <div className="text-[13px] text-faint">and {entries.length - 6} more</div>
      )}
      {footer}
    </div>
  );
}

/** A thin gradient connector fading toward the accent at the focused node. */
function Rail({ direction }: { direction: "in" | "out" }) {
  return (
    <div aria-hidden className="relative w-[54px] flex-none">
      <span
        className="absolute left-0 right-0 top-1/2 h-px"
        style={{
          background:
            direction === "in"
              ? "linear-gradient(90deg, color-mix(in srgb, var(--ink) 6%, transparent), var(--accent))"
              : "linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--ink) 6%, transparent))",
        }}
      />
    </div>
  );
}

/**
 * Five shapes, one per kind. Deliberately geometric rather than iconographic:
 * a shape is read before a glyph, and these appear at 14px.
 */
function NodeGlyph({ kind, large = false }: { kind: string; large?: boolean }) {
  const size = large ? "h-4 w-4" : "h-3.5 w-3.5";
  if (kind === "project") return <span className={`${size} flex-none rounded-[5px] bg-accent`} />;
  if (kind === "role") return <span className={`${size} flex-none rounded-pill bg-ink/80`} />;
  if (kind === "bundle")
    return <span className={`${size} flex-none rounded-[5px] border-[1.5px] border-ink/70`} />;
  return <span className={`${size} flex-none rounded-[5px] border-[1.5px] border-ink/25`} />;
}
