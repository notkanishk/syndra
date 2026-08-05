// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const chime = vi.fn();
vi.mock("@/lib/driftChime", () => ({ playDriftChime: () => chime() }));

const respond = vi.fn();
vi.mock("@/lib/api-client", () => ({ request: () => respond() }));

import { useIndicators } from "@/lib/queries/useIndicators";

/**
 * The rail is backed by `placeholderData`, which hands out four fabricated
 * zeros before the first payload lands so a failed poll can never blank a
 * badge. Nothing downstream can tell those zeros from real ones, which makes
 * "did this number change?" a question about readings rather than values.
 */
function Probe() {
  const { data, isPlaceholderData } = useIndicators(true);
  return (
    <span data-testid="drift" data-placeholder={String(isPlaceholderData)}>
      {data?.drift}
    </span>
  );
}

function payload(drift: number) {
  return {
    pending_requests: 0,
    expiring_grants: 0,
    pending_propagation: 0,
    drift,
    zitadel_reachable: true,
  };
}

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <Probe />
    </QueryClientProvider>,
  );
  return { client, ...view };
}

beforeEach(() => {
  chime.mockClear();
  respond.mockReset();
});
afterEach(() => vi.restoreAllMocks());

describe("useIndicators — the drift chime", () => {
  it("stays silent when the first real payload arrives over the placeholder", async () => {
    respond.mockResolvedValue(payload(12));
    const { getByTestId } = mount();

    // The placeholder is in place first, and it reads zero.
    expect(getByTestId("drift").dataset.placeholder).toBe("true");

    await waitFor(() => expect(getByTestId("drift").textContent).toBe("12"));

    // Twelve unexplained grants arriving is not a rise from zero — nobody ever
    // saw that zero. A chime here would sound on every single page load, which
    // is exactly the training-out the chime exists to avoid.
    expect(chime, "the first reading is an arrival, not a rise").not.toHaveBeenCalled();
  });

  it("sounds when drift rises between two real readings", async () => {
    respond.mockResolvedValueOnce(payload(3)).mockResolvedValue(payload(5));
    const { client, getByTestId } = mount();

    await waitFor(() => expect(getByTestId("drift").textContent).toBe("3"));
    expect(chime).not.toHaveBeenCalled();

    await client.invalidateQueries({ queryKey: ["governance", "indicators"] });
    await waitFor(() => expect(getByTestId("drift").textContent).toBe("5"));

    expect(chime).toHaveBeenCalledTimes(1);
  });

  it("says nothing when drift holds steady or falls", async () => {
    respond
      .mockResolvedValueOnce(payload(7))
      .mockResolvedValueOnce(payload(7))
      .mockResolvedValue(payload(2));
    const { client, getByTestId } = mount();

    await waitFor(() => expect(getByTestId("drift").textContent).toBe("7"));
    await client.invalidateQueries({ queryKey: ["governance", "indicators"] });
    await client.invalidateQueries({ queryKey: ["governance", "indicators"] });
    await waitFor(() => expect(getByTestId("drift").textContent).toBe("2"));

    // A count that stays at seven is not news, and a queue that drained is
    // good news nobody needs to be interrupted for.
    expect(chime).not.toHaveBeenCalled();
  });
});
