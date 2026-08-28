// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DegradedBanner } from "@/components/states/DegradedBanner";

const mode = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}));

vi.mock("@/lib/api-client", () => ({
  request: async () => mode.value,
}));

function renderBanner() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DegradedBanner />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mode.value = {
    directory: "zitadel",
    seed_active: false,
    seed_residue: 0,
    degraded: false,
  };
});

describe("Degraded banner", () => {
  it("says nothing when the directory is live and unseeded", async () => {
    renderBanner();
    await waitFor(() => {
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  it("says the numbers are fiction when the directory fell back", async () => {
    mode.value = { directory: "demo", degraded: true, zitadel_configured: true };
    renderBanner();
    expect(await screen.findByText("These numbers are not real.")).toBeInTheDocument();
  });

  it("warns when demo fixtures sit underneath a live directory", async () => {
    // The gap this closes: real people and projects, demo bundles and rules,
    // and — before this — no signal at all that the two were mixed.
    mode.value = {
      directory: "zitadel",
      seed_active: true,
      seed_residue: 12,
      degraded: false,
    };
    renderBanner();
    expect(await screen.findByText("12 items on these screens are sample data.")).toBeInTheDocument();
    expect(screen.getByText(/Set SYNDRA_SEED_DEMO=false first/)).toBeInTheDocument();
  });

  // The regression this whole field exists for. An operator sees demo data,
  // sets SYNDRA_SEED_DEMO=false, restarts — and the old banner keyed off
  // seed_active vanished, reading as confirmation the fix worked, while every
  // seeded row was still on screen.
  it("keeps warning after seeding is switched off but the rows remain", async () => {
    mode.value = {
      directory: "zitadel",
      seed_active: false,
      seed_residue: 31,
      degraded: false,
    };
    renderBanner();
    expect(await screen.findByText("31 items on these screens are sample data.")).toBeInTheDocument();
    expect(screen.getByText(/Sample data is switched off/)).toBeInTheDocument();
  });

  it("offers the backend's own reset command, not a hardcoded one", async () => {
    mode.value = {
      directory: "zitadel",
      seed_residue: 4,
      reset_command: "make reset-demo-data",
      degraded: false,
    };
    renderBanner();
    expect(await screen.findByText("make reset-demo-data")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Copy command: make reset-demo-data/ }),
    ).toBeInTheDocument();
    // The dry-run default is the safety property: reading the banner must not
    // leave an operator thinking the command deletes on sight.
    expect(screen.getByText(/prints what it would delete and stops/)).toBeInTheDocument();
  });

  it("prefers the harder warning when both are true", async () => {
    mode.value = {
      directory: "demo",
      seed_active: true,
      seed_residue: 12,
      degraded: true,
    };
    renderBanner();
    expect(await screen.findByText("These numbers are not real.")).toBeInTheDocument();
    expect(screen.queryByText(/are sample data\./)).not.toBeInTheDocument();
  });
});
