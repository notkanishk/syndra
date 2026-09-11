// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import IdentityProviderPage from "@/app/zitadel/page";

const rotation = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const health = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const projects = vi.hoisted(() => ({ value: [] as unknown[] }));

// The page reads rotation status through the raw request helper and health
// through its own hook, so both are stubbed at their respective seams.
vi.mock("@/lib/api-client", () => ({
  request: async () => rotation.value,
}));

vi.mock("@/lib/queries/useZitadel", () => ({
  useZitadelHealth: () => ({ data: health.value, isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: projects.value, isLoading: false, error: null }),
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <IdentityProviderPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  health.value = { status: "ok", mode: "live", latency_ms: 21, domain: "auth.example.org" };
  projects.value = [];
  rotation.value = {
    key_installed: true,
    status: "ok",
    age_days: 12,
    threshold_days: 90,
    last_rotated_at: "2026-07-01T00:00:00Z",
    rotate_command: "make zitadel-actions-rotate-key",
  };
});

/**
 * (1) One page, one fact about reachability — not two cards computing it
 * differently.
 *
 * The "Connection" tile used to key off `mode` alone, which reads "live" for
 * both a healthy connection and a live one whose last call just failed — so a
 * real outage showed "Connected" here directly under "Unreachable" in the
 * banner above it. These assert the tile always agrees with the banner's
 * verdict, for every reachability state the backend can report.
 */
describe("Identity provider · one verdict about reachability", () => {
  it("calls it 'Not connected' everywhere when Zitadel was never set up", async () => {
    health.value = {
      status: "disabled",
      mode: "local-policy-only",
      error: "Management client not initialized — check ZITADEL_DOMAIN, ZITADEL_MACHINE_KEY_PATH, and backend startup logs",
    };
    renderPage();
    expect(await screen.findAllByText(/Not connected/)).not.toHaveLength(0);
    expect(screen.queryByText(/^Unreachable/)).not.toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
  });

  it("calls it 'Unreachable' everywhere when Zitadel is configured but did not answer — never 'Connected'", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    renderPage();
    expect(await screen.findAllByText(/Unreachable/)).not.toHaveLength(0);
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
    expect(screen.queryByText(/^Not connected/)).not.toBeInTheDocument();
  });
});

/**
 * (4) The project count names its own source, and stops claiming a live read
 * it did not make.
 *
 * `health.data` used to stay undefined for every non-"ok" status (a separate
 * bug: the health request threw instead of returning the body), so `live`
 * was always false and this tile always fell back to Syndra's cached count
 * — reading as a live Zitadel number even when Zitadel had just answered.
 */
describe("Identity provider · projects tile names its source", () => {
  it("reports Zitadel's own count, live, once the connection is healthy", async () => {
    health.value = { status: "ok", mode: "live", projects_total: 6 };
    projects.value = [{ id: "cached-only" }];
    renderPage();
    expect(await screen.findByText("6")).toBeInTheDocument();
    expect(screen.getByText("As Zitadel reported them just now.")).toBeInTheDocument();
  });

  it("falls back to Syndra's remembered count, and says so, when Zitadel can't be asked", async () => {
    health.value = { status: "error", mode: "live", error: "dial tcp: connection refused" };
    projects.value = [{ id: "p1" }, { id: "p2" }, { id: "p3" }];
    renderPage();
    expect(await screen.findByText("3")).toBeInTheDocument();
    expect(
      screen.getByText("As Syndra last remembered them — Zitadel cannot be asked right now."),
    ).toBeInTheDocument();
  });
});

/**
 * (6) The tile headline reads as a fact while there is nothing to do, and
 * only turns into an instruction once the same status crosses its threshold
 * — "Replace within 53 days" beside a body saying "Nothing to do" was two
 * readings of one number disagreeing.
 */
describe("Identity provider · signing key tile is truthful and calm", () => {
  it("gives the age as a fact, not an instruction, while within the limit", async () => {
    rotation.value = {
      key_installed: true,
      status: "ok",
      age_days: 12,
      threshold_days: 90,
      last_rotated_at: "2026-07-01T00:00:00Z",
    };
    renderPage();
    expect(await screen.findByText("12 days old")).toBeInTheDocument();
    expect(screen.queryByText(/Replace within/)).not.toBeInTheDocument();
    expect(screen.getByText(/Replace by .* · nothing to do/)).toBeInTheDocument();
  });

  it("only turns into an instruction once the threshold is actually crossed", async () => {
    rotation.value = {
      key_installed: true,
      status: "warn",
      age_days: 85,
      threshold_days: 90,
      last_rotated_at: "2026-01-01T00:00:00Z",
    };
    renderPage();
    expect(await screen.findByText(/Replace within 5 days/)).toBeInTheDocument();
    expect(screen.queryByText(/nothing to do/)).not.toBeInTheDocument();
  });
});

describe("Identity provider · signing key", () => {
  it("shows the rotate command the backend reported, with a copy control", async () => {
    renderPage();
    expect(await screen.findByText("make zitadel-actions-rotate-key")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Copy command: make zitadel-actions-rotate-key/ }),
    ).toBeInTheDocument();
  });

  // The command alone is half the job: rotation that Zitadel accepts and the
  // backend never picks up leaves every Action call failing verification.
  it("names the env swap and the restart that must follow it", async () => {
    renderPage();
    expect(await screen.findByText(/ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT/)).toBeInTheDocument();
    expect(screen.getByText(/docker compose restart backend/)).toBeInTheDocument();
    expect(screen.getByText(/access\s+details Syndra normally adds at sign-in are missing/)).toBeInTheDocument();
  });

  it("does not offer to rotate the key itself", async () => {
    renderPage();
    await screen.findByText("make zitadel-actions-rotate-key");
    expect(screen.queryByRole("button", { name: /^Rotate/ })).not.toBeInTheDocument();
  });

  it("states the age against the threshold in words", async () => {
    renderPage();
    expect(
      await screen.findByText(/12 days old, within the 90-day limit/),
    ).toBeInTheDocument();
  });

  it("calls a stale key stale rather than reporting a number", async () => {
    rotation.value = {
      key_installed: true,
      status: "stale",
      age_days: 220,
      threshold_days: 90,
      rotate_command: "make zitadel-actions-rotate-key",
    };
    renderPage();
    expect(await screen.findByText(/more than twice the 90-day limit/)).toBeInTheDocument();
  });

  // "disabled" reads like "not set up yet" and means every inbound Action
  // request is being trusted unchecked. The copy has to say the second thing,
  // and the command offered has to be register, not rotate — there is nothing
  // to rotate.
  it("treats a missing key as verification being off, and offers register", async () => {
    rotation.value = {
      key_installed: false,
      status: "disabled",
      threshold_days: 90,
      rotate_command: "make zitadel-actions-rotate-key",
    };
    renderPage();
    expect(await screen.findByText(/accepts every request that claims to come from Zitadel without checking/)).toBeInTheDocument();
    expect(screen.getByText("make zitadel-actions-register")).toBeInTheDocument();
    expect(screen.queryByText("make zitadel-actions-rotate-key")).not.toBeInTheDocument();
  });
});
