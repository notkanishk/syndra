// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import IdentityProviderPage from "@/app/zitadel/page";

const rotation = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
const health = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));

// The page reads rotation status through the raw request helper and health
// through its own hook, so both are stubbed at their respective seams.
vi.mock("@/lib/api-client", () => ({
  request: async () => rotation.value,
}));

vi.mock("@/lib/queries/useZitadel", () => ({
  useZitadelHealth: () => ({ data: health.value, isLoading: false, error: null, refetch: () => {} }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [], isLoading: false, error: null }),
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
  rotation.value = {
    key_installed: true,
    status: "ok",
    age_days: 12,
    threshold_days: 90,
    last_rotated_at: "2026-07-01T00:00:00Z",
    rotate_command: "make zitadel-actions-rotate-key",
  };
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
