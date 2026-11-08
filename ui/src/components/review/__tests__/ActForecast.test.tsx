// @vitest-environment jsdom
import { QueryClient, QueryClientProvider, useMutation } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HoldsDueForReview } from "@/components/review/HoldsDueForReview";
import { ApiError } from "@/lib/api-client";
import { FORECAST } from "@/lib/outcome";
import type { Hold } from "@/lib/queries/useHolds";

/**
 * A-19 P2, the first 100ms in the future tense, through a real mutation so
 * `isPending` is React Query's and not a stub's. A hold's lift is the surface:
 * it uses the same shared seam (ActionForecast, Crossfade, ActionOutcome) the
 * Requests, Home, Expiring and person pages use.
 */
const hold: Hold = {
  id: "h1",
  subject_id: "u1",
  target: "truenas",
  field: "enabled",
  value: "true",
  direction: "deny",
  actor_id: "op",
  reason: "safety review",
  review_date: "2026-10-01T00:00:00Z",
  created_at: "2026-09-01T00:00:00Z",
};

const wire = vi.hoisted(() => ({
  settle: null as null | { resolve: () => void; reject: (e: unknown) => void },
  cacheWrites: 0,
}));

vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));
vi.mock("@/lib/queries/useHolds", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useHolds")>()),
  useHoldsDueForReview: () => ({
    data: { due_for_review: [hold], count: 1 },
    dataUpdatedAt: Date.now(),
    isLoading: false,
    error: null,
    refetch: vi.fn(),
  }),
  useLiftHold: () =>
    useMutation({
      mutationFn: () =>
        new Promise<void>((resolve, reject) => {
          wire.settle = { resolve, reject };
        }),
    }),
}));
vi.mock("@/components/names", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));

function mount() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const setQueryData = client.setQueryData.bind(client) as (...a: unknown[]) => unknown;
  client.setQueryData = ((...args: unknown[]) => {
    wire.cacheWrites += 1;
    return setQueryData(...args);
  }) as typeof client.setQueryData;
  return render(
    <QueryClientProvider client={client}>
      <HoldsDueForReview />
    </QueryClientProvider>,
  );
}

const lift = () => screen.getByRole("button", { name: /Lift the hold on their TrueNAS account/ });

beforeEach(() => {
  wire.settle = null;
  wire.cacheWrites = 0;
});

describe("the first 100ms, in the future tense (A-19 P2)", () => {
  it("forecasts in the seat in the same tick the write goes in flight, with a breathing amber dot and no bloom", async () => {
    const { container } = mount();
    fireEvent.click(lift());

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toBe(FORECAST.lift);
    expect(seat.querySelector(".breathe.bg-warn")).not.toBeNull();
    // Nothing has happened yet, so nothing celebrates or says it did.
    expect(container.querySelector(".bloom")).toBeNull();
    expect(seat.textContent).not.toMatch(/^Lifted/);
    // The seat is the forecast; the buttons went in the same frame.
    expect(screen.queryByRole("button", { name: /Lift the hold/ })).toBeNull();
    // Words, not counts: nothing was written to the cache ahead of the server.
    expect(wire.cacheWrites).toBe(0);
  });

  it("crossfades to the true word on the answer, and only the true word blooms", async () => {
    const { container } = mount();
    fireEvent.click(lift());
    await screen.findByText(FORECAST.lift);

    await act(async () => wire.settle!.resolve());

    const seat = await screen.findByRole("status");
    expect(seat.textContent).toContain("Lifted");
    expect(seat.textContent).not.toContain(FORECAST.lift);
    expect(seat.querySelector(".bloom")).not.toBeNull();
    expect(container.querySelectorAll(".bloom")).toHaveLength(1);
  });

  it("shows the honest error, not a rollback flash: forecast gone, error said, buttons back", async () => {
    const { container } = mount();
    fireEvent.click(lift());
    await screen.findByText(FORECAST.lift);

    await act(async () =>
      wire.settle!.reject(new ApiError(500, { message: "boom", details: { request_id: "req_9" } })),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Not lifted");
    expect(alert.textContent).toContain("Nothing was changed.");
    expect(screen.queryByRole("status")).toBeNull();
    expect(container.querySelector(".bloom")).toBeNull();
    expect(screen.getByRole("button", { name: "Try again" })).toBeTruthy();
  });
});
