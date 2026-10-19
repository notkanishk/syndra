// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DegradedBanner } from "@/components/states/DegradedBanner";
import { UiViewProvider } from "@/lib/ui-view";

const mode = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}));

const asked = vi.hoisted(() => ({ count: 0 }));

vi.mock("@/lib/api-client", () => ({
  request: async () => {
    asked.count++;
    return mode.value;
  },
}));

/** The server steps sit behind a disclosure, closed until asked for. */
function openServerSteps() {
  const toggle = screen.getByRole("button", { name: /For whoever runs the Syndra server/ });
  expect(toggle).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(toggle);
  expect(toggle).toHaveAttribute("aria-expanded", "true");
}

function renderBanner(isOperator = true) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UiViewProvider isOperator={isOperator}>
        <DegradedBanner />
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  asked.count = 0;
  mode.value = {
    directory: "zitadel",
    seed_active: false,
    seed_residue: 0,
    degraded: false,
  };
});

describe("Degraded banner", () => {
  // A member's proxy refuses /system/mode; asking every minute was a 403 a
  // minute on every member page.
  it("never asks for a member", async () => {
    mode.value = { ...mode.value, degraded: true };
    renderBanner(false);
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(asked.count).toBe(0);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("says nothing when the directory is live and unseeded", async () => {
    renderBanner();
    await waitFor(() => {
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  // N23: never sample data. With a last good read, the page shows it with
  // its age; with none, nothing, and the Notice says so.
  it("shows the last known read, with its age, when Zitadel stops answering", async () => {
    const at = new Date(Date.now() - 14 * 60_000).toISOString();
    mode.value = { directory: "zitadel", degraded: true, zitadel_configured: true, last_read_at: at };
    renderBanner();
    expect(await screen.findByText(/^Zitadel isn't answering\. This is what it held at /)).toBeInTheDocument();
    expect(screen.getByText(/Everything below is real, read/)).toBeInTheDocument();
    expect(screen.getByText(/14 min ago/)).toBeInTheDocument();
    expect(screen.queryByText(/not real|sample data/)).not.toBeInTheDocument();
  });

  it("shows nothing at all when Zitadel has never been read", async () => {
    mode.value = { directory: "unreachable", degraded: true, zitadel_configured: true, last_read_at: null };
    renderBanner();
    expect(
      await screen.findByText("Zitadel isn't answering, and Syndra hasn't read it yet."),
    ).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(/There is nothing to show until it answers/);
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
    expect(
      await screen.findByText("12 things on these screens are sample data."),
    ).toBeInTheDocument();
    // The operator-facing body says what is true for anyone reading it; the
    // server-side steps stay out of sight until asked for.
    expect(screen.getByText(/People and projects are real\./)).toBeInTheDocument();
    expect(screen.queryByText(/Set SYNDRA_SEED_DEMO=false first/)).not.toBeInTheDocument();
    openServerSteps();
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
    expect(
      await screen.findByText("31 things on these screens are sample data."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Sample data is switched off/)).not.toBeInTheDocument();
    openServerSteps();
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
    await screen.findByText("4 things on these screens are sample data.");
    expect(screen.queryByText("make reset-demo-data")).not.toBeInTheDocument();
    openServerSteps();
    expect(screen.getByText("make reset-demo-data")).toBeInTheDocument();
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
    expect(
      await screen.findByText("Zitadel isn't answering, and Syndra hasn't read it yet."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/are sample data\./)).not.toBeInTheDocument();
  });
});
