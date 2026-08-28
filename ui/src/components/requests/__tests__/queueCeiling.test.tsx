// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import type { AccessRequest } from "@/lib/queries/useRequests";

const queue = vi.hoisted(() => ({ data: [] as AccessRequest[] }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsAdmin: () => ({ ...queue, isLoading: false, error: null, refetch: () => {} }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({ data: [], isLoading: false, error: null }),
}));

vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: () => null,
}));

function request(at: number): AccessRequest {
  return {
    id: `r${at}`,
    requester_id: `u${at}`,
    project_id: "pLaser",
    role_key: "trained",
    justification: "Needs the laser for a build.",
    status: "pending",
    created_at: "2026-07-22T06:00:00Z",
  };
}

function renderQueue(count: number) {
  queue.data = Array.from({ length: count }, (_, at) => request(at));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator userId="op1" />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
}

beforeEach(() => {
  queue.data = [];
});

/**
 * `/requests/bulk-decision` refuses past 500. The bar has been able to say so
 * since the selection work and this queue was not telling it — so an operator
 * select-alled, tapped, and met the refusal afterwards, which is the one
 * sequence the bar exists to prevent.
 */
describe("the request queue states its ceiling before the tap", () => {
  // 501 requests in the queue, 25 rows on screen. The ceiling is about the
  // SELECTION and not about what is rendered, which is the whole reason the
  // cap could be added without touching this assertion — and the reason the
  // test below exists to hold those two apart.
  it("names the limit over 500", () => {
    renderQueue(501);
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 501 requests/ }));

    expect(screen.getByText(/you can change at most 500 requests at once/)).toBeTruthy();
  });

  it("leaves a selection inside the ceiling alone", () => {
    renderQueue(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 3 requests/ }));

    expect(screen.queryByText(/you can change at most/)).toBeNull();
  });
});

/**
 * The cap is on rendering and on nothing else.
 *
 * This screen used to render every row it was given, so a long queue paid its
 * whole cost on arrival — and there was no point at which a reader could tell
 * how much more there was. The drift queue and the people list both cap at a
 * page and offer the next one; this one did not.
 *
 * What must not follow is "select all" quietly coming to mean "select the
 * page". The bar's ceiling message counts the pending queue, so a cap that
 * narrowed the selection would change what that number is about without
 * saying so — and an operator would tap it believing they had the queue.
 */
describe("the queue has a visible end", () => {
  it("renders a page rather than the whole queue", () => {
    renderQueue(60);

    // A page, not sixty.
    expect(screen.getAllByRole("checkbox", { name: /Select this request/ }).length).toBe(25);
    expect(screen.getByRole("button", { name: /Load next 25/ })).toBeTruthy();
    expect(screen.getByText("35 more")).toBeTruthy();
  });

  it("still selects the whole queue, not the page", () => {
    renderQueue(60);
    fireEvent.click(screen.getByRole("checkbox", { name: /Select these 60 requests/ }));

    // Sixty is under the ceiling, so no warning — and the count is the queue's.
    expect(screen.queryByText(/you can change at most/)).toBeNull();
    // Sixty, not twenty-five: the bar counts the queue and the page is only
    // what is drawn.
    expect(document.body.textContent).toMatch(/60 requests selected/);
  });

  it("shows the next page on request", () => {
    renderQueue(60);
    fireEvent.click(screen.getByRole("button", { name: /Load next 25/ }));

    expect(screen.getAllByRole("checkbox", { name: /Select this request/ }).length).toBe(50);
    expect(screen.getByText("10 more")).toBeTruthy();
  });
});
