"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

type NodeKind = "application" | "bundle" | "project" | "role";

interface TopologyNode {
  id: string;
  label: string;
  kind: NodeKind;
  project_id?: string;
  description?: string;
  meta?: Record<string, string>;
}

interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  kind: "application" | "bundle" | "contains" | "rule";
  label: string;
  meta?: Record<string, string>;
}

interface TopologyGraph {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

const laneOrder: NodeKind[] = ["application", "bundle", "project", "role"];
const laneTitles: Record<NodeKind, string> = {
  application: "Applications",
  bundle: "Bundles",
  project: "Projects",
  role: "Roles",
};

const laneX: Record<NodeKind, number> = {
  application: 40,
  bundle: 340,
  project: 640,
  role: 940,
};

const nodeWidth = 240;
const nodeHeight = 96;
const nodeGap = 124;

function nodeTone(kind: NodeKind) {
  switch (kind) {
    case "application":
      return "border-sky-500/30 bg-sky-500/8 text-sky-600 dark:text-sky-300";
    case "bundle":
      return "border-amber-500/30 bg-amber-500/8 text-amber-600 dark:text-amber-300";
    case "project":
      return "border-primary/30 bg-primary/8 text-primary";
    case "role":
      return "border-emerald-500/30 bg-emerald-500/8 text-emerald-600 dark:text-emerald-300";
  }
}

export default function GraphPage() {
  const [topology, setTopology] = useState<TopologyGraph>({ nodes: [], edges: [] });
  const [loading, setLoading] = useState(true);
  const [selectedNodeId, setSelectedNodeId] = useState("");
  const [projectFilter, setProjectFilter] = useState("all");

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const res = await fetch("/api/proxy/topology");
        const data = await res.json();
        const next: TopologyGraph = {
          nodes: Array.isArray(data?.nodes) ? data.nodes : [],
          edges: Array.isArray(data?.edges) ? data.edges : [],
        };
        setTopology(next);
        if (next.nodes.length > 0) {
          setSelectedNodeId(next.nodes[0].id);
        }
      } finally {
        setLoading(false);
      }
    }

    load();
  }, []);

  const projectOptions = topology.nodes
    .filter((node) => node.kind === "project")
    .sort((a, b) => a.label.localeCompare(b.label));

  const visibleNodeMap = new Map<string, TopologyNode>();
  const filteredNodes = topology.nodes
    .filter((node) => {
      if (projectFilter === "all") {
        return true;
      }

      if (node.kind === "project" || node.kind === "role" || node.kind === "application") {
        return node.project_id === projectFilter;
      }

      if (node.kind === "bundle") {
        return topology.edges.some(
          (edge) =>
            edge.source === node.id &&
            edge.target.startsWith(`role:${projectFilter}:`),
        );
      }

      return true;
    })
    .sort((a, b) => a.label.localeCompare(b.label));

  filteredNodes.forEach((node) => visibleNodeMap.set(node.id, node));

  const filteredEdges = topology.edges.filter(
    (edge) => visibleNodeMap.has(edge.source) && visibleNodeMap.has(edge.target),
  );

  useEffect(() => {
    if (selectedNodeId && filteredNodes.some((node) => node.id === selectedNodeId)) {
      return;
    }
    if (filteredNodes.length > 0) {
      setSelectedNodeId(filteredNodes[0].id);
    }
  }, [filteredNodes, selectedNodeId]);

  const positions = new Map<string, { x: number; y: number }>();
  laneOrder.forEach((kind) => {
    const laneNodes = filteredNodes.filter((node) => node.kind === kind);
    laneNodes.forEach((node, index) => {
      positions.set(node.id, {
        x: laneX[kind],
        y: 84 + index * nodeGap,
      });
    });
  });

  const canvasHeight =
    Math.max(
      ...laneOrder.map((kind) => filteredNodes.filter((node) => node.kind === kind).length),
      1,
    ) *
      nodeGap +
    80;

  const selectedNode = filteredNodes.find((node) => node.id === selectedNodeId) || null;
  const selectedEdges = selectedNode
    ? filteredEdges.filter((edge) => edge.source === selectedNode.id || edge.target === selectedNode.id)
    : [];

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">God Mode</h1>
        <p className="text-muted mt-2">
          Trace how applications, bundles, projects, and roles connect across the whole makerspace in one visual map.
        </p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Nodes</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{filteredNodes.length}</p>
          <p className="text-sm text-muted mt-1">Visible in the current graph filter</p>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Edges</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{filteredEdges.length}</p>
          <p className="text-sm text-muted mt-1">Topology relationships rendered</p>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Rule Paths</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">
            {filteredEdges.filter((edge) => edge.kind === "rule").length}
          </p>
          <p className="text-sm text-muted mt-1">Role propagation edges</p>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Bundle Links</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">
            {filteredEdges.filter((edge) => edge.kind === "bundle").length}
          </p>
          <p className="text-sm text-muted mt-1">Bundle-to-role grants</p>
        </Card>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1.55fr,0.8fr] gap-6">
        <Card className="overflow-hidden">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between mb-6">
            <div>
              <CardTitle>Topology Canvas</CardTitle>
              <p className="mt-2 text-sm text-muted">Filter by project to reduce noise or keep the full graph for the macroscopic view.</p>
            </div>
            <select
              value={projectFilter}
              onChange={(event) => setProjectFilter(event.target.value)}
              className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground"
            >
              <option value="all">All Projects</option>
              {projectOptions.map((project) => (
                <option key={project.id} value={project.project_id}>
                  {project.label}
                </option>
              ))}
            </select>
          </div>

          {loading ? (
            <p className="text-sm text-muted">Loading topology...</p>
          ) : (
            <div className="overflow-x-auto rounded-2xl border border-border bg-[radial-gradient(circle_at_top,_rgba(79,70,229,0.12),_transparent_35%),linear-gradient(180deg,rgba(255,255,255,0.04),transparent)]">
              <div className="relative min-w-[1240px]" style={{ height: canvasHeight }}>
                <div className="absolute inset-0">
                  {laneOrder.map((kind) => (
                    <div
                      key={kind}
                      className="absolute top-0 bottom-0 border-r border-dashed border-border/70"
                      style={{ left: laneX[kind] - 24, width: nodeWidth + 48 }}
                    >
                      <div className="sticky top-0 z-10 m-4 rounded-full border border-border bg-background/80 px-3 py-1 text-xs font-semibold uppercase tracking-[0.22em] text-muted backdrop-blur">
                        {laneTitles[kind]}
                      </div>
                    </div>
                  ))}
                </div>

                <svg className="absolute inset-0 h-full w-full pointer-events-none">
                  {filteredEdges.map((edge) => {
                    const source = positions.get(edge.source);
                    const target = positions.get(edge.target);
                    if (!source || !target) {
                      return null;
                    }

                    const startX = source.x + nodeWidth;
                    const startY = source.y + nodeHeight / 2;
                    const endX = target.x;
                    const endY = target.y + nodeHeight / 2;
                    const controlOffset = Math.max((endX - startX) * 0.45, 60);
                    const stroke =
                      edge.kind === "rule"
                        ? "rgba(79,70,229,0.8)"
                        : edge.kind === "bundle"
                          ? "rgba(245,158,11,0.8)"
                          : edge.kind === "application"
                            ? "rgba(14,165,233,0.8)"
                            : "rgba(16,185,129,0.6)";

                    return (
                      <path
                        key={edge.id}
                        d={`M ${startX} ${startY} C ${startX + controlOffset} ${startY}, ${endX - controlOffset} ${endY}, ${endX} ${endY}`}
                        stroke={stroke}
                        strokeWidth={selectedNode && (edge.source === selectedNode.id || edge.target === selectedNode.id) ? 3 : 2}
                        fill="none"
                        strokeDasharray={edge.kind === "contains" ? "4 5" : undefined}
                        opacity={selectedNode && edge.source !== selectedNode.id && edge.target !== selectedNode.id ? 0.25 : 0.92}
                      />
                    );
                  })}
                </svg>

                {filteredNodes.map((node) => {
                  const position = positions.get(node.id);
                  if (!position) {
                    return null;
                  }

                  const isSelected = selectedNodeId === node.id;
                  return (
                    <button
                      key={node.id}
                      onClick={() => setSelectedNodeId(node.id)}
                      className={`absolute rounded-2xl border p-4 text-left shadow-sm transition-all ${nodeTone(node.kind)} ${
                        isSelected ? "ring-2 ring-primary shadow-xl scale-[1.02]" : "hover:-translate-y-0.5 hover:shadow-lg"
                      }`}
                      style={{ left: position.x, top: position.y, width: nodeWidth, minHeight: nodeHeight }}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="text-base font-semibold leading-tight text-foreground">{node.label}</p>
                          <p className="mt-1 text-[11px] uppercase tracking-[0.18em] text-muted">{node.kind}</p>
                        </div>
                        {node.project_id && (
                          <Badge variant="outline" className="border-border/80 bg-background/60 text-[10px]">
                            {node.project_id}
                          </Badge>
                        )}
                      </div>
                      <p className="mt-3 line-clamp-2 text-sm text-muted">
                        {node.description || "No extra description attached to this node yet."}
                      </p>
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Inspector</CardTitle>
            </CardHeader>
            {selectedNode ? (
              <div className="space-y-4">
                <div className={`rounded-2xl border p-4 ${nodeTone(selectedNode.kind)}`}>
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">{selectedNode.kind}</p>
                  <p className="mt-2 text-xl font-semibold text-foreground">{selectedNode.label}</p>
                  <p className="mt-2 text-sm text-muted">
                    {selectedNode.description || "This node does not have an extended description yet."}
                  </p>
                </div>

                <div>
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Metadata</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {Object.entries(selectedNode.meta || {}).map(([key, value]) => (
                      <Badge key={key} variant="secondary">
                        {key}: {value}
                      </Badge>
                    ))}
                    {!selectedNode.meta || Object.keys(selectedNode.meta).length === 0 ? (
                      <p className="text-sm text-muted">No extra metadata for this node.</p>
                    ) : null}
                  </div>
                </div>

                <div>
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Connected Edges</p>
                  <div className="mt-2 space-y-2">
                    {selectedEdges.map((edge) => (
                      <div key={edge.id} className="rounded-xl border border-border bg-surfaceHover p-3">
                        <div className="flex items-center justify-between gap-3">
                          <p className="text-sm font-medium text-foreground">{edge.label}</p>
                          <Badge variant="outline">{edge.kind}</Badge>
                        </div>
                        <p className="mt-1 text-xs text-muted">
                          {edge.source === selectedNode.id ? "Outgoing to" : "Incoming from"}{" "}
                          {filteredNodes.find((node) => node.id === (edge.source === selectedNode.id ? edge.target : edge.source))?.label || "hidden node"}
                        </p>
                      </div>
                    ))}
                    {selectedEdges.length === 0 && <p className="text-sm text-muted">No visible edges for this node in the current filter.</p>}
                  </div>
                </div>
              </div>
            ) : (
              <p className="text-sm text-muted">Select a node to inspect its relationships.</p>
            )}
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Legend</CardTitle>
            </CardHeader>
            <div className="space-y-3 text-sm">
              {laneOrder.map((kind) => (
                <div key={kind} className="flex items-center justify-between gap-3 rounded-xl border border-border bg-surfaceHover p-3">
                  <div>
                    <p className="font-medium text-foreground">{laneTitles[kind]}</p>
                    <p className="text-xs text-muted">Visual lane for {kind} entities</p>
                  </div>
                  <span className={`inline-flex rounded-full border px-3 py-1 text-xs font-semibold ${nodeTone(kind)}`}>
                    {kind}
                  </span>
                </div>
              ))}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
