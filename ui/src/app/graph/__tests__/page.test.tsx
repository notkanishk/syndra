// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AccessMapPage from "@/app/graph/page";
import type { TopologyGraph } from "@/lib/queries/useTopology";

const topology = vi.hoisted(() => ({
  data: undefined as TopologyGraph | undefined,
  isLoading: false,
  error: null as unknown,
}));

vi.mock("@/lib/queries/useTopology", () => ({
  useTopology: () => ({ ...topology, refetch: () => {} }),
}));

function renderMap() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <AccessMapPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  topology.data = {
    nodes: [
      { id: "p1", label: "Laser Lab", kind: "project" },
      { id: "p2", label: "Metal Shop", kind: "project" },
      { id: "r1", label: "trained", kind: "role", project_id: "p1" },
      { id: "b1", label: "Lab Tech", kind: "bundle" },
    ],
    edges: [
      { id: "e1", source: "p1", target: "r1", kind: "contains", label: "" },
      { id: "e2", source: "b1", target: "r1", kind: "bundle", label: "" },
    ],
  };
  topology.isLoading = false;
  topology.error = null;
});

describe("Access map", () => {
  it("opens on a browsable overview rather than a node you have to search for", () => {
    renderMap();

    // Every kind is listed, grouped, without typing anything.
    expect(screen.getByRole("button", { name: /Laser Lab/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Metal Shop/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Lab Tech/ })).toBeInTheDocument();

    // And nothing is focused yet: there is no "What gives this" column at the root.
    expect(screen.queryByText("What gives this")).not.toBeInTheDocument();
  });

  it("focuses a node when one is picked from the overview", () => {
    renderMap();
    fireEvent.click(screen.getByRole("button", { name: /Laser Lab/ }));

    expect(screen.getByText("What gives this")).toBeInTheDocument();
    expect(screen.getByText("What this gives")).toBeInTheDocument();
  });

  it("offers a way back to the root from a focused node", () => {
    renderMap();
    fireEvent.click(screen.getByRole("button", { name: /Laser Lab/ }));
    fireEvent.click(screen.getByRole("button", { name: "Everything" }));

    // Back at the overview: the focused-node columns are gone and the other
    // projects are pickable again.
    expect(screen.queryByText("What gives this")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Metal Shop/ })).toBeInTheDocument();
  });

  it("keeps a kind visible until it is explicitly switched off", () => {
    renderMap();

    // Toggling Projects off hides the project group but leaves the others.
    fireEvent.click(screen.getByRole("button", { name: /^Projects\s*2$/ }));
    expect(screen.queryByRole("button", { name: /Metal Shop/ })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /trained/ })).toBeInTheDocument();
  });

  it("says nothing is drawable rather than showing a blank panel", () => {
    topology.data = { nodes: [], edges: [] };
    renderMap();
    expect(screen.getByText("Nothing to show yet.")).toBeInTheDocument();
  });
});
