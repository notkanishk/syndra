// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, request } from "@/lib/api-client";
import { outcomeFromError } from "@/lib/outcome";

/**
 * Offline is its own state, distinct from `degraded`.
 *
 * Degraded means the API answered and answered badly — the directory fell
 * back and every name on screen is fiction. Offline means nothing answered:
 * what is on screen is true, it is just not being kept up to date. An
 * operator who reads "Syndra can't reach the provider" while standing in a
 * workshop with no wifi goes looking for a broken server.
 */
function setOnline(online: boolean) {
  Object.defineProperty(navigator, "onLine", { value: online, configurable: true });
}

afterEach(() => {
  setOnline(true);
  vi.restoreAllMocks();
});

describe("a write attempted with no network", () => {
  it("is refused before it is sent, so nothing can half-land", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    setOnline(false);

    await expect(request("/users/u_1/grants", { method: "POST", body: {} })).rejects.toThrow();
    expect(fetchSpy, "nothing may reach the wire").not.toHaveBeenCalled();
  });

  // Refused, not failed: nothing was attempted, so the state the operator is
  // looking at is the state that still holds. A `fetch` rejection on a dead
  // network carries no status and would read as the opposite.
  it("reads as a refusal rather than a failure", async () => {
    setOnline(false);
    const error = await request("/x", { method: "DELETE" }).catch((e) => e);
    const outcome = outcomeFromError(error);

    expect(error).toBeInstanceOf(ApiError);
    expect(outcome.kind).toBe("refused");
  });

  // There is no client-side queue by design — one would be a second ledger
  // nobody can inspect — so the copy must not promise that it will send later.
  it("does not promise to send it later", async () => {
    setOnline(false);
    const outcome = outcomeFromError(await request("/x", { method: "POST" }).catch((e) => e));

    expect(outcome.detail).toContain("Nothing was sent");
    expect(outcome.detail?.toLowerCase()).not.toContain("will send");
  });

  // A read that fails while offline fails harmlessly, and every list already
  // has an error state. Blocking it here would replace one honest failure
  // with another.
  it("lets a read through, because a failed read is already handled", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValue(new TypeError("Failed to fetch"));
    setOnline(false);

    await expect(request("/users")).rejects.toThrow();
    expect(fetchSpy).toHaveBeenCalled();
  });
});
