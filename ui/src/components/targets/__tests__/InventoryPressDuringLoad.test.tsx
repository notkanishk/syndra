// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * "Read again" pressed while the page's first load is still on its way, with the real hooks.
 * Cancelling a load that has no copy leaves the query idle with nothing and no error, so when the
 * read it was replaced by failed, the list drew "Nothing outside Syndra." about an account list
 * nobody had seen.
 */

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: undefined, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  return {
    ...actual,
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: () => ({ ...idle(), data: { reachable: true, lifecycle: "active" } }),
    useResolveLogFinding: inert,
    useReconcileTarget: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
  };
});
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/PeopleOnTarget", () => ({ PeopleOnTarget: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));

afterEach(() => vi.unstubAllGlobals());

// The real shape: a stopped pass answers 503 with the inventory as its body (handlers/inventory.go),
// through the real reader, so the card's words come from what the API sends and not from a mock.
describe("a stopped pass, as the API answers it", () => {
  it("shows the add-on's sentence from the 503 body, and never the empty list", async () => {
    const detail = "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.";
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).endsWith("/inventory")
          ? Promise.resolve(
              new Response(JSON.stringify({ target: "truenas", current: false, halted: true, reason: "read_refused", detail }), { status: 503 }),
            )
          : new Promise<Response>(() => {}),
      ),
    );
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <TargetOverview target="truenas" />
      </QueryClientProvider>,
    );
    expect(await screen.findByText(`The account list could not be read. ${detail}`)).toBeInTheDocument();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
  });

  it("says it once for the whole Accounts region, with Read again on the line, which reads afresh", async () => {
    const fetched = vi.fn((input: RequestInfo | URL) =>
      String(input).includes("/inventory")
        ? Promise.resolve(
            new Response(JSON.stringify({ target: "truenas", current: false, halted: true, reason: "target_unreachable" }), { status: 503 }),
          )
        : new Promise<Response>(() => {}),
    );
    vi.stubGlobal("fetch", fetched);
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <TargetOverview target="truenas" />
      </QueryClientProvider>,
    );
    const line = await screen.findByText(/^The account list could not be read\./);
    expect(screen.getAllByText(/The account list could not be read/)).toHaveLength(1);
    // Above both cards, not inside one: the region's line, before "Not created by Syndra".
    const card = screen.getByText("Not created by Syndra");
    expect(line.compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    fireEvent.click(within(line.parentElement!).getByRole("button", { name: "Read again" }));
    await waitFor(() => expect(fetched.mock.calls.some(([u]) => String(u).includes("fresh=true"))).toBe(true));
  });
});

describe("a press during the account list's first load", () => {
  it("shows why the fresh read failed, and never the empty list", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).includes("fresh=true")
          ? Promise.resolve(new Response(JSON.stringify({ error: "TARGET_UNREACHABLE", message: "TrueNAS did not answer" }), { status: 503 }))
          : new Promise<Response>(() => {}),
      ),
    );
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <TargetOverview target="truenas" />
      </QueryClientProvider>,
    );
    fireEvent.click(await screen.findByRole("button", { name: "Read again" }));
    expect(await screen.findByText("Not read")).toBeInTheDocument();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
  });
});
