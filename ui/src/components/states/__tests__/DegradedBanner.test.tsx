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
  mode.value = { directory: "zitadel", seed_active: false, degraded: false };
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

  it("warns when demo fixtures are seeded underneath a live directory", async () => {
    // The gap this closes: real people and projects, demo bundles and rules,
    // and — before this — no signal at all that the two were mixed.
    mode.value = { directory: "zitadel", seed_active: true, degraded: false };
    renderBanner();
    expect(
      await screen.findByText("Demo data is seeded into this deployment."),
    ).toBeInTheDocument();
  });

  it("prefers the harder warning when both are true", async () => {
    mode.value = { directory: "demo", seed_active: true, degraded: true };
    renderBanner();
    expect(await screen.findByText("These numbers are not real.")).toBeInTheDocument();
    expect(
      screen.queryByText("Demo data is seeded into this deployment."),
    ).not.toBeInTheDocument();
  });
});
