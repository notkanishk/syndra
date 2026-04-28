"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";

import { ProjectName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Card, CardTitle } from "@/components/ui/Card";
import { Drawer } from "@/components/ui/Drawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Select } from "@/components/ui/Select";
import { Skeleton } from "@/components/ui/Skeleton";
import {
  useTopology,
  type TopologyNode,
  type TopologyNodeKind,
} from "@/lib/queries/useTopology";

const laneOrder: TopologyNodeKind[] = ["application", "bundle", "project", "role"];
const laneTitles: Record<TopologyNodeKind, string> = {
  application: "Applications",
  bundle: "Bundles",
  project: "Projects",
  role: "Roles",
};

const laneX: Record<TopologyNodeKind, number> = {
  application: 40,
  bundle: 340,
  project: 640,
  role: 940,
};

const nodeWidth = 240;
const nodeHeight = 96;
const nodeGap = 124;

function nodeTone(kind: TopologyNodeKind) {
  switch (kind) {
    case "application":
      return "border-sky-500/30 bg-sky-500/8 text-sky-600 dark:text-sky-300";
    case "bundle":
      return "border-amber-500/30 bg-amber-500/8 text-amber-600 dark:text-amber-300";
    case "project":
      return "border-primary-container/40 bg-primary-container/10 text-primary-container";
    case "role":
      return "border-emerald-500/30 bg-emerald-500/8 text-emerald-600 dark:text-emerald-300";
  }
}

function nodeDetailHref(kind: TopologyNodeKind): string {
  switch (kind) {
    case "application":
      return "/applications";
    case "bundle":
      return "/bundles";
    case "project":
    case "role":
      return "/projects";
  }
}

export default function GraphPage() {
  const topologyQuery = useTopology();
  const topology = useMemo(
    () => topologyQuery.data ?? { nodes: [], edges: [] },
    [topologyQuery.data],
  );
  const loading = topologyQuery.isLoading;

  const [selectedNodeId, setSelectedNodeId] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [projectFilter, setProjectFilter] = useState("all");

  // Pan + zoom state for the topology canvas — mandatory per OpenSpec topology-graph
  // capability. Identical mechanics to the Stage 2 canvas; only chrome around it
  // changed (glass legend chip + Drawer for inspector).
  const [viewport, setViewport] = useState({ x: 0, y: 0, scale: 1 });
  const panStart = useRef<{ x: number; y: number; vx: number; vy: number } | null>(null);
  const canvasContainerRef = useRef<HTMLDivElement | null>(null);

  const onWheel = (event: React.WheelEvent<HTMLDivElement>) => {
    if (!event.ctrlKey && !event.metaKey) {
      // Allow normal vertical scroll outside the canvas; only zoom with modifier.
      // Supports trackpad pinch (which fires deltaY with ctrlKey) and Cmd+scroll.
      return;
    }
    event.preventDefault();
    setViewport((current) => {
      const next = current.scale * (event.deltaY < 0 ? 1.1 : 0.9);
      const clamped = Math.max(0.4, Math.min(2.5, next));
      return { ...current, scale: clamped };
    });
  };

  const onPanStart = (event: React.MouseEvent<HTMLDivElement>) => {
    // Only pan when the empty canvas surface itself is the target — not when
    // a node button (which absolutely-positions over it) was clicked.
    if (event.target !== event.currentTarget) return;
    panStart.current = {
      x: event.clientX,
      y: event.clientY,
      vx: viewport.x,
      vy: viewport.y,
    };
  };

  const onPanMove = (event: React.MouseEvent<HTMLDivElement>) => {
    if (!panStart.current) return;
    setViewport((current) => ({
      ...current,
      x: panStart.current!.vx + (event.clientX - panStart.current!.x),
      y: panStart.current!.vy + (event.clientY - panStart.current!.y),
    }));
  };

  const onPanEnd = () => {
    panStart.current = null;
  };

  const resetView = () => setViewport({ x: 0, y: 0, scale: 1 });

  const projectOptions = useMemo(
    () =>
      topology.nodes
        .filter((node) => node.kind === "project")
        .sort((a, b) => a.label.localeCompare(b.label)),
    [topology.nodes],
  );

  const filteredNodes = useMemo(() => {
    return topology.nodes
      .filter((node) => {
        if (projectFilter === "all") return true;
        if (
          node.kind === "project" ||
          node.kind === "role" ||
          node.kind === "application"
        ) {
          return node.project_id === projectFilter;
        }
        if (node.kind === "bundle") {
          return topology.edges.some(
            (edge) =>
              edge.source === node.id && edge.target.startsWith(`role:${projectFilter}:`),
          );
        }
        return true;
      })
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [topology, projectFilter]);

  const visibleNodeMap = useMemo(() => {
    const m = new Map<string, TopologyNode>();
    filteredNodes.forEach((node) => m.set(node.id, node));
    return m;
  }, [filteredNodes]);

  const filteredEdges = useMemo(
    () =>
      topology.edges.filter(
        (edge) => visibleNodeMap.has(edge.source) && visibleNodeMap.has(edge.target),
      ),
    [topology.edges, visibleNodeMap],
  );

  useEffect(() => {
    if (!selectedNodeId && filteredNodes.length > 0) {
      setSelectedNodeId(filteredNodes[0].id);
      return;
    }
    if (selectedNodeId && !filteredNodes.some((n) => n.id === selectedNodeId)) {
      setSelectedNodeId(filteredNodes[0]?.id ?? "");
    }
  }, [filteredNodes, selectedNodeId]);

  const positions = useMemo(() => {
    const p = new Map<string, { x: number; y: number }>();
    laneOrder.forEach((kind) => {
      const laneNodes = filteredNodes.filter((node) => node.kind === kind);
      laneNodes.forEach((node, index) => {
        p.set(node.id, { x: laneX[kind], y: 84 + index * nodeGap });
      });
    });
    return p;
  }, [filteredNodes]);

  const canvasHeight =
    Math.max(
      ...laneOrder.map((kind) => filteredNodes.filter((n) => n.kind === kind).length),
      1,
    ) *
      nodeGap +
    80;

  const selectedNode = filteredNodes.find((node) => node.id === selectedNodeId) || null;
  const selectedEdges = selectedNode
    ? filteredEdges.filter(
        (edge) => edge.source === selectedNode.id || edge.target === selectedNode.id,
      )
    : [];

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Topology</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          God-mode graph
        </h1>
        <p className="text-on-surface-variant mt-2">
          Trace how applications, bundles, projects, and roles connect across
          the whole makerspace in one visual map. Click a node to open the
          inspector drawer.
        </p>
      </header>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <Card variant="container">
          <Eyebrow>Nodes</Eyebrow>
          <p className="text-4xl font-semibold text-on-surface mt-2 font-display">
            {filteredNodes.length}
          </p>
          <p className="text-xs text-on-surface-variant mt-1">In current filter</p>
        </Card>
        <Card variant="container">
          <Eyebrow>Edges</Eyebrow>
          <p className="text-4xl font-semibold text-on-surface mt-2 font-display">
            {filteredEdges.length}
          </p>
          <p className="text-xs text-on-surface-variant mt-1">Topology relationships</p>
        </Card>
        <Card variant="container">
          <Eyebrow>Rule paths</Eyebrow>
          <p className="text-4xl font-semibold text-on-surface mt-2 font-display">
            {filteredEdges.filter((e) => e.kind === "rule").length}
          </p>
          <p className="text-xs text-on-surface-variant mt-1">Role propagation edges</p>
        </Card>
        <Card variant="container">
          <Eyebrow>Bundle links</Eyebrow>
          <p className="text-4xl font-semibold text-on-surface mt-2 font-display">
            {filteredEdges.filter((e) => e.kind === "bundle").length}
          </p>
          <p className="text-xs text-on-surface-variant mt-1">Bundle-to-role grants</p>
        </Card>
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between mb-6">
          <div>
            <CardTitle>Topology canvas</CardTitle>
            <p className="mt-2 text-sm text-on-surface-variant">
              Filter by project to reduce noise or keep the full graph for the
              macroscopic view.
            </p>
          </div>
          <Select
            value={projectFilter}
            onChange={(event) => setProjectFilter(event.target.value)}
            className="max-w-[16rem]"
          >
            <option value="all">All projects</option>
            {projectOptions.map((project) => (
              <option key={project.id} value={project.project_id}>
                {project.label}
              </option>
            ))}
          </Select>
        </div>

        {loading ? (
          <Skeleton className="h-72 w-full" />
        ) : filteredNodes.length === 0 ? (
          <EmptyState
            eyebrow="No graph"
            title="Topology is empty"
            description="Once projects, applications, bundles, and mapping rules exist, they'll appear here as a connected graph for visual exploration."
          />
        ) : (
          <div className="relative">
            {/* Floating controls — glass chip per Stage 3 spec. */}
            <div className="absolute right-2 top-2 z-20 flex gap-1 glass-card p-1 rounded-full">
              <button
                type="button"
                onClick={() =>
                  setViewport((v) => ({ ...v, scale: Math.min(2.5, v.scale * 1.1) }))
                }
                className="rounded-full px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                aria-label="Zoom in"
              >
                +
              </button>
              <button
                type="button"
                onClick={() =>
                  setViewport((v) => ({ ...v, scale: Math.max(0.4, v.scale * 0.9) }))
                }
                className="rounded-full px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                aria-label="Zoom out"
              >
                −
              </button>
              <button
                type="button"
                onClick={resetView}
                className="rounded-full px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                aria-label="Reset view"
              >
                Reset
              </button>
              <span className="px-2 py-1 text-[10px] text-on-surface-variant self-center">
                {Math.round(viewport.scale * 100)}%
              </span>
            </div>

            {/* Floating legend chip — top-left, glass surface. */}
            <div className="absolute left-2 top-2 z-20 glass-card p-2 rounded-full flex items-center gap-2">
              {laneOrder.map((kind) => (
                <span
                  key={kind}
                  className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold ${nodeTone(kind)}`}
                >
                  {kind}
                </span>
              ))}
            </div>

            <p className="absolute right-2 bottom-2 z-20 glass-card px-3 py-1 rounded-full text-[10px] text-on-surface-variant">
              Drag to pan · ⌘/Ctrl + scroll to zoom
            </p>

            <div
              ref={canvasContainerRef}
              onWheel={onWheel}
              onMouseDown={onPanStart}
              onMouseMove={onPanMove}
              onMouseUp={onPanEnd}
              onMouseLeave={onPanEnd}
              className="overflow-hidden rounded-card border border-outline-variant bg-[radial-gradient(circle_at_top,_rgba(129,140,248,0.10),_transparent_45%),linear-gradient(180deg,rgba(255,255,255,0.04),transparent)] cursor-grab active:cursor-grabbing"
              style={{ height: Math.min(canvasHeight, 720) }}
            >
              <div
                className="relative min-w-[1240px]"
                style={{
                  height: canvasHeight,
                  transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.scale})`,
                  transformOrigin: "top left",
                  transition: panStart.current ? "none" : "transform 0.1s ease-out",
                }}
              >
                <div className="absolute inset-0">
                  {laneOrder.map((kind) => (
                    <div
                      key={kind}
                      className="absolute top-0 bottom-0 border-r border-dashed border-outline-variant/70"
                      style={{ left: laneX[kind] - 24, width: nodeWidth + 48 }}
                    >
                      <div className="sticky top-0 z-10 m-4 rounded-full glass-card px-3 py-1 text-xs font-semibold uppercase tracking-[0.22em] text-on-surface-variant">
                        {laneTitles[kind]}
                      </div>
                    </div>
                  ))}
                </div>

                <svg className="absolute inset-0 h-full w-full pointer-events-none">
                  {filteredEdges.map((edge) => {
                    const source = positions.get(edge.source);
                    const target = positions.get(edge.target);
                    if (!source || !target) return null;

                    const startX = source.x + nodeWidth;
                    const startY = source.y + nodeHeight / 2;
                    const endX = target.x;
                    const endY = target.y + nodeHeight / 2;
                    const controlOffset = Math.max((endX - startX) * 0.45, 60);
                    const stroke =
                      edge.kind === "rule"
                        ? "rgba(129,140,248,0.85)"
                        : edge.kind === "bundle"
                          ? "rgba(245,158,11,0.85)"
                          : edge.kind === "application"
                            ? "rgba(14,165,233,0.85)"
                            : "rgba(16,185,129,0.6)";

                    return (
                      <path
                        key={edge.id}
                        d={`M ${startX} ${startY} C ${startX + controlOffset} ${startY}, ${endX - controlOffset} ${endY}, ${endX} ${endY}`}
                        stroke={stroke}
                        strokeWidth={
                          selectedNode &&
                          (edge.source === selectedNode.id || edge.target === selectedNode.id)
                            ? 3
                            : 2
                        }
                        fill="none"
                        strokeDasharray={edge.kind === "contains" ? "4 5" : undefined}
                        opacity={
                          selectedNode &&
                          edge.source !== selectedNode.id &&
                          edge.target !== selectedNode.id
                            ? 0.25
                            : 0.92
                        }
                      />
                    );
                  })}
                </svg>

                {filteredNodes.map((node) => {
                  const position = positions.get(node.id);
                  if (!position) return null;

                  const isSelected = selectedNodeId === node.id;
                  return (
                    <button
                      key={node.id}
                      onClick={() => {
                        setSelectedNodeId(node.id);
                        setDrawerOpen(true);
                      }}
                      onMouseDown={(e) => e.stopPropagation()}
                      className={`absolute rounded-card border p-4 text-left shadow-sm transition-all ${nodeTone(node.kind)} ${
                        isSelected
                          ? "ring-2 ring-primary-container shadow-xl scale-[1.02]"
                          : "hover:-translate-y-0.5 hover:shadow-lg"
                      }`}
                      style={{ left: position.x, top: position.y, width: nodeWidth, minHeight: nodeHeight }}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-base font-semibold leading-tight text-on-surface">
                            {node.label}
                          </p>
                          <p className="mt-1 text-[11px] uppercase tracking-[0.18em] text-on-surface-variant">
                            {node.kind}
                          </p>
                        </div>
                        {node.project_id && (
                          <Badge
                            variant="outline"
                            className="border-outline-variant/80 bg-surface-container/60 text-[10px]"
                          >
                            <ProjectName id={node.project_id} fallback={node.project_id} />
                          </Badge>
                        )}
                      </div>
                      <p className="mt-3 line-clamp-2 text-sm text-on-surface-variant">
                        {node.description || "No extra description attached to this node yet."}
                      </p>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        )}
      </Card>

      <Drawer
        open={drawerOpen && Boolean(selectedNode)}
        onClose={() => setDrawerOpen(false)}
        labelledBy="graph-inspector-title"
        size="lg"
      >
        {selectedNode && (
          <>
            <div className="flex items-start justify-between gap-3">
              <div>
                <Eyebrow tone="primary">{selectedNode.kind}</Eyebrow>
                <h2
                  id="graph-inspector-title"
                  className="text-2xl font-semibold text-on-surface mt-1 font-display"
                >
                  {selectedNode.label}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setDrawerOpen(false)}
                className="rounded-full px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                aria-label="Close inspector"
              >
                Close
              </button>
            </div>

            <p className="mt-3 text-sm text-on-surface-variant">
              {selectedNode.description ||
                "This node does not have an extended description yet."}
            </p>

            {selectedNode.project_id && (
              <p className="mt-3 text-xs text-on-surface-variant">
                Project:{" "}
                <span className="text-on-surface font-medium">
                  <ProjectName id={selectedNode.project_id} />
                </span>
              </p>
            )}

            <Link
              href={nodeDetailHref(selectedNode.kind)}
              className="mt-4 inline-flex items-center justify-center rounded-full bg-[linear-gradient(135deg,var(--primary),var(--secondary))] text-on-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
            >
              View details →
            </Link>

            <section className="mt-6">
              <Eyebrow>Metadata</Eyebrow>
              <div className="mt-2 flex flex-wrap gap-2">
                {Object.entries(selectedNode.meta || {}).map(([key, value]) => (
                  <Badge key={key} variant="secondary">
                    {key}: {value}
                  </Badge>
                ))}
                {!selectedNode.meta || Object.keys(selectedNode.meta).length === 0 ? (
                  <p className="text-sm text-on-surface-variant">
                    No extra metadata for this node.
                  </p>
                ) : null}
              </div>
            </section>

            <section className="mt-6">
              <Eyebrow>Connected edges</Eyebrow>
              <div className="mt-2 space-y-2">
                {selectedEdges.map((edge) => {
                  const otherId =
                    edge.source === selectedNode.id ? edge.target : edge.source;
                  const otherLabel =
                    filteredNodes.find((node) => node.id === otherId)?.label ??
                    "hidden node";
                  return (
                    <div
                      key={edge.id}
                      className="rounded-card border border-outline-variant bg-surface-container-low p-3"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <p className="text-sm font-medium text-on-surface">{edge.label}</p>
                        <Badge variant="outline">{edge.kind}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-on-surface-variant">
                        {edge.source === selectedNode.id ? "Outgoing to" : "Incoming from"}{" "}
                        {otherLabel}
                      </p>
                    </div>
                  );
                })}
                {selectedEdges.length === 0 && (
                  <p className="text-sm text-on-surface-variant">
                    No visible edges for this node in the current filter.
                  </p>
                )}
              </div>
            </section>
          </>
        )}
      </Drawer>
    </div>
  );
}
