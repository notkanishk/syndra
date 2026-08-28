// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { WithdrawnAccess } from "@/components/review/WithdrawnAccess";

/**
 * The page is two buckets, and which one is empty IS the answer.
 *
 * They used to render only when non-empty, so a revocation going terminal
 * inserted a red card above the queue somebody was reading. Structure moving in
 * response to data, on the page whose whole job is to say what has not happened
 * yet.
 */

function row(id: string, spent: boolean) {
  return {
    id,
    op_type: "revoke",
    user_id: `u-${id}`,
    project_id: "pLab",
    role_keys: ["maker"],
    status: spent ? "spent" : "queued",
    attempts: spent ? 9 : 3,
    created_at: "2026-08-20T00:00:00Z",
    target: "truenas",
    age_seconds: 4000,
    spent,
  };
}

const rows = {
  queuedOnly: [row("r1", false)],
  both: [row("r1", false), row("r2", true)],
};

let payload: unknown = { revocations: rows.queuedOnly };

vi.mock("@/lib/api-client", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api-client")>("@/lib/api-client");
  return { ...actual, request: vi.fn(async () => payload) };
});

async function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <WithdrawnAccess />
    </QueryClientProvider>,
  );
  await screen.findByText("Still on the way");
}

function headings() {
  return screen
    .getAllByText(/Given up|Still on the way/)
    .map((el) => el.textContent!.trim());
}

describe("what has not gone away yet", () => {
  it("keeps both buckets, in one order, whatever either holds", async () => {
    payload = { revocations: rows.queuedOnly };
    await renderPage();

    // The empty one is present, and says what its emptiness means — which is
    // the good news on this page, not the absence of news.
    expect(screen.getByText("Given up")).toBeInTheDocument();
    expect(screen.getByText(/Nothing has been given up on/)).toBeInTheDocument();
    expect(headings(), "terminal first, always").toEqual([
      "Given up",
      "Still on the way",
    ]);
  });

  it("does not insert a card above the queue when one goes terminal", async () => {
    payload = { revocations: rows.both };
    await renderPage();

    // Same order, same seats: the queue an operator was reading has not moved.
    expect(headings()).toEqual(["Given up", "Still on the way"]);
    expect(screen.queryByText(/Nothing has been given up on/)).toBeNull();
  });
});
