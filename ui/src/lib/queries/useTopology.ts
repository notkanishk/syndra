"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

export type TopologyNodeKind = "application" | "bundle" | "project" | "role";

export interface TopologyNode {
  id: string;
  label: string;
  kind: TopologyNodeKind;
  project_id?: string;
  description?: string;
  meta?: Record<string, string>;
}

export interface TopologyEdge {
  id: string;
  source: string;
  target: string;
  kind: "application" | "bundle" | "contains" | "rule";
  label: string;
  meta?: Record<string, string>;
}

export interface TopologyGraph {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

const KEYS = {
  graph: ["topology", "graph"] as const,
};

/** Full topology graph for /graph. Cached at the QueryClient defaults. */
export function useTopology() {
  return useQuery({
    queryKey: KEYS.graph,
    queryFn: async (): Promise<TopologyGraph> => {
      const data = await request<TopologyGraph | { nodes?: unknown; edges?: unknown }>(
        "/topology",
      );
      const nodes = Array.isArray(data?.nodes) ? (data.nodes as TopologyNode[]) : [];
      const edges = Array.isArray(data?.edges) ? (data.edges as TopologyEdge[]) : [];
      return { nodes, edges };
    },
  });
}

export const topologyQueryKeys = KEYS;
