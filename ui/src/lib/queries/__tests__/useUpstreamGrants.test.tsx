// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const respond = vi.fn();
vi.mock("@/lib/api-client", () => ({ request: (path: string) => respond(path) }));

import { useUpstreamGrants } from "@/lib/queries/useUpstream";

/**
 * `/zitadel/grants` now observes rather than lists Zitadel live (see
 * `internal/observe`), so each page of the response can carry `observed_at`
 * and `complete`. This is the aggregation half: one incomplete page must mark
 * the WHOLE fetched set truncated, the same meaning the aggregate-cap case
 * already carries, and the freshest `observed_at` seen must survive.
 */
function Probe() {
  const { data } = useUpstreamGrants();
  return (
    <span
      data-testid="grants"
      data-total={data?.total}
      data-truncated={String(data?.truncated)}
      data-observed-at={data?.observedAt ?? ""}
    />
  );
}

function mount() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <Probe />
    </QueryClientProvider>,
  );
}

beforeEach(() => respond.mockReset());
afterEach(() => vi.restoreAllMocks());

describe("useUpstreamGrants", () => {
  it("marks the aggregate truncated when a later page's own observation was incomplete", async () => {
    respond.mockImplementation((path: string) => {
      if (path?.includes("offset=0")) {
        return Promise.resolve({
          items: [{ id: "g1", userId: "u1", projectId: "p1", roleKeys: ["member"] }],
          total: 2,
          observed_at: "2026-09-11T03:00:00Z",
          complete: true,
        });
      }
      return Promise.resolve({
        items: [{ id: "g2", userId: "u2", projectId: "p1", roleKeys: ["member"] }],
        total: 2,
        observed_at: "2026-09-11T03:00:05Z",
        complete: false,
      });
    });

    const { findByTestId } = mount();
    const el = await findByTestId("grants");
    await waitFor(() => expect(el.dataset.total).toBe("2"));

    expect(el.dataset.truncated).toBe("true");
    expect(el.dataset.observedAt).toBe("2026-09-11T03:00:05Z");
  });

  it("is not truncated when every page observed completely", async () => {
    respond.mockResolvedValue({
      items: [{ id: "g1", userId: "u1", projectId: "p1", roleKeys: ["member"] }],
      total: 1,
      observed_at: "2026-09-11T03:00:00Z",
      complete: true,
    });

    const { findByTestId } = mount();
    const el = await findByTestId("grants");
    await waitFor(() => expect(el.dataset.total).toBe("1"));

    expect(el.dataset.truncated).toBe("false");
  });
});
