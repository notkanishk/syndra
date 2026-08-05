"use client";

import { useMemo, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented } from "@/components/ui/Select";
import { useTopology, type TopologyNode } from "@/lib/queries/useTopology";
import { useDebounce } from "@/lib/useDebounce";

type Depth = "1" | "2" | "all";

/** Kind order at the root: the thing you almost always start from comes first. */
const KIND_ORDER: Array<{ kind: TopologyNode["kind"]; label: string; blurb: string }> = [
  { kind: "project", label: "Projects", blurb: "A boundary that owns roles." },
  { kind: "role", label: "Roles", blurb: "What somebody actually holds." },
  { kind: "bundle", label: "Bundles", blurb: "A set of roles handed out as one unit." },
  { kind: "application", label: "Apps", blurb: "Things that receive a token." },
];

/**
 * S5 · Automation › Access map.
 *
 * The old graph was unreadable because it drew everything at once. The fix is
 * not a better force layout — it is not drawing everything. Pick one node and
 * the map answers exactly two questions, left to right: what feeds this, and
 * what does this feed.
 *
 * But "pick one node" needs somewhere to pick FROM. The map opens on a root
 * view — every node grouped by kind, browsable without typing — and a
 * breadcrumb returns to it from anywhere. Search is a shortcut, never the only
 * way in: a map you can only enter by already knowing what you are looking for
 * is a map that answers questions you did not need it for.
 *
 * Node shapes are a separate namespace from Access source and deliberately do
 * not borrow that component: project, role, bundle, rule and app are kinds of
 * thing, not reasons somebody holds access.
 */
export default function AccessMapPage() {
  const topology = useTopology();
  // null is the root — deliberately the initial state. Auto-focusing the
  // most-connected node used to make the map look like it had no overview at
  // all, and left no way back to one.
  const [focusId, setFocusId] = useState<string | null>(null);
  const [depth, setDepth] = useState<Depth>("1");
  // Holds the kinds that are switched OFF, so the default (an empty set) shows
  // everything. A legend where clicking one entry silently hides the other
  // three reads as a bug the first time it happens.
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [query, setQuery] = useState("");
  const search = useDebounce(query, 200).trim().toLowerCase();

  const nodes = useMemo(() => topology.data?.nodes ?? [], [topology.data]);
  const edges = useMemo(() => topology.data?.edges ?? [], [topology.data]);

  const byId = useMemo(() => new Map(nodes.map((node) => [node.id, node])), [nodes]);
  const focus = focusId ? byId.get(focusId) : undefined;

  const degree = useMemo(() => {
    const counts = new Map<string, number>();
    for (const edge of edges) {
      counts.set(edge.source, (counts.get(edge.source) ?? 0) + 1);
      counts.set(edge.target, (counts.get(edge.target) ?? 0) + 1);
    }
    return counts;
  }, [edges]);

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

  function toggleKind(kind: string) {
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });
  }

  const shown = (kind: string) => !hidden.has(kind);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Access map"
        meta={
          focus
            ? "One node at a time, and what it touches."
            : "Everything Syndra knows about, grouped. Pick something to see what feeds it and what it feeds."
        }
      />

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
            <div className="flex flex-col gap-1 text-[14px]">
              {KIND_ORDER.map(({ kind, label }) => (
                <button
                  key={kind}
                  type="button"
                  onClick={() => toggleKind(kind)}
                  aria-pressed={shown(kind)}
                  className={`flex items-center gap-2.5 rounded-nav px-2 py-1.5 text-left motion-tint hover:bg-[var(--hover)] ${
                    shown(kind) ? "" : "opacity-40"
                  }`}
                >
                  <NodeGlyph kind={kind} />
                  <span>{label}</span>
                  <span className="flex-1" />
                  <span className="text-[12.5px] text-faint">
                    {nodes.filter((node) => node.kind === kind).length}
                  </span>
                </button>
              ))}
              {/* A rule is an edge, not a node: it is the dashed link between
                  two roles rather than a thing you can hold. */}
              <div className="flex items-center gap-2.5 px-2 py-1.5">
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
            {nodes.length} nodes, {edges.length} edges in total. Drawing them all at once is what
            made this screen useless.
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
          ) : nodes.length === 0 ? (
            <div className="type-empty-title">Nothing to draw yet.</div>
          ) : !focus ? (
            <RootView
              nodes={nodes}
              degree={degree}
              shown={shown}
              onFocus={setFocusId}
            />
          ) : (
            <div className="flex flex-col gap-5">
              {/*
                The way back. A map you can enter but not leave is why this
                screen felt like a dead end — search in, and stuck there.
              */}
              <nav className="flex items-center gap-2 text-[14.5px]">
                <button
                  type="button"
                  onClick={() => setFocusId(null)}
                  className="font-semibold text-accent-text motion-tint hover:brightness-110"
                >
                  All nodes
                </button>
                <span className="text-faint">/</span>
                <span className="font-semibold">{focus.label}</span>
              </nav>

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
                      {focus.project_id && focus.project_id !== focus.id ? (
                        <> · {byId.get(focus.project_id)?.label ?? focus.project_id}</>
                      ) : null}
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

/**
 * The birds-eye view: everything, grouped by kind, ordered by how connected
 * each node is so the ones worth opening surface first. Still not a hairball —
 * these are lists, not a force layout — but you can see the whole shape of the
 * system without knowing a single name to type.
 */
function RootView({
  nodes,
  degree,
  shown,
  onFocus,
}: {
  nodes: TopologyNode[];
  degree: Map<string, number>;
  shown: (kind: string) => boolean;
  onFocus: (id: string) => void;
}) {
  const groups = KIND_ORDER.filter(({ kind }) => shown(kind)).map((group) => ({
    ...group,
    items: nodes
      .filter((node) => node.kind === group.kind)
      .sort((a, b) => (degree.get(b.id) ?? 0) - (degree.get(a.id) ?? 0)),
  }));

  const visible = groups.filter((group) => group.items.length > 0);

  if (visible.length === 0) {
    return (
      <div className="flex flex-col gap-2">
        <div className="type-empty-title">Nothing shown.</div>
        <p className="max-w-[52ch] text-[14px] text-muted">
          Every kind is switched off in the panel on the left. Turn one back on.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-7">
      {visible.map((group) => (
        <section key={group.kind}>
          <div className="mb-2.5 flex flex-wrap items-baseline gap-2.5">
            <h2 className="type-label">{group.label}</h2>
            <span className="text-[13px] text-faint">
              {group.items.length} · {group.blurb}
            </span>
          </div>

          <div className="grid gap-2.5 sm:grid-cols-2 xl:grid-cols-3">
            {group.items.map((node) => (
              <button
                key={node.id}
                type="button"
                onClick={() => onFocus(node.id)}
                className="flex items-center gap-3 rounded-block border border-line bg-surface-1 px-4 py-3 text-left motion-tint hover:bg-[var(--hover)]"
              >
                <NodeGlyph kind={node.kind} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14.5px] font-semibold">{node.label}</span>
                  <span className="block truncate text-[12.5px] text-faint">
                    {connectionSummary(degree.get(node.id) ?? 0)}
                  </span>
                </span>
              </button>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function connectionSummary(count: number): string {
  if (count === 0) return "nothing connects to this";
  return `${count} ${count === 1 ? "connection" : "connections"}`;
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
              className={`flex items-center gap-3 rounded-block bg-surface-1 px-4 py-3.5 text-left motion-tint hover:bg-[var(--hover)] ${
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
 * Four shapes, one per kind. Deliberately geometric rather than iconographic:
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
