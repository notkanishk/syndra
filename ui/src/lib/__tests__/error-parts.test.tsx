// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ErrorState } from "@/components/states";
import { ApiError } from "@/lib/api-client";
import { NETWORK_FAULT, describeFailure } from "@/lib/outcome";
import { UiViewProvider } from "@/lib/ui-view";

function show(error: unknown, fine?: string) {
  const { container } = render(
    <QueryClientProvider client={new QueryClient()}>
      <UiViewProvider isOperator>
        <ErrorState title="Couldn't load people." error={error} onRetry={() => {}} fine={fine} />
      </UiViewProvider>
    </QueryClientProvider>,
  );
  return container.textContent ?? "";
}

describe("an error carries the copy guide's four parts", () => {
  const withId = new ApiError(502, { error: "ZITADEL_ERROR", message: "Zitadel didn't answer", details: { request_id: "req-123" } });

  it("says what failed, what did not, the world, the next step and the reference", () => {
    const text = show(withId, "Syndra itself is fine");
    expect(text).toContain("Zitadel didn't answer.");
    expect(text).toContain("Syndra itself is fine.");
    expect(text).toContain("Nothing was changed.");
    expect(text).toContain(
      "Try again in a minute. If it keeps failing, ask the person who runs the Syndra server and quote the reference below.",
    );
    expect(text).toContain("Quote this if you ask for help.");
    expect(text).toContain("req-123");
  });

  it("does not point at a reference that does not exist", () => {
    const text = show(new ApiError(500, { message: "It broke" }));
    expect(text).not.toContain("quote the reference");
    expect(text).not.toContain("Quote this");
    expect(text).toContain("Try again in a minute.");
  });

  it("carries no em dash or exclamation mark", () => {
    expect(show(withId)).not.toMatch(/[—!]/);
  });
});

describe("fine is never said beside a failure that is Syndra's own", () => {
  const FINE = "Syndra itself is fine";
  const zitadelDown = new ApiError(502, { error: "ZITADEL_ERROR", message: "Zitadel gave an answer Syndra could not use." });

  it("says it when Syndra answered and Zitadel did not", () => {
    expect(show(zitadelDown, FINE)).toContain("Syndra itself is fine.");
  });
  it.each([
    ["a network fault", new TypeError("Failed to fetch")],
    ["Zitadel not set up", new ApiError(502, { error: "ZITADEL_ERROR", message: "Zitadel is not set up — Syndra has no connection configured to it" })],
    ["Syndra's own 500", new ApiError(500, { error: "INTERNAL_ERROR", message: "Something went wrong inside Syndra." })],
    ["a plain Error", new Error("boom")],
  ])("drops it for %s", (_n, err) => {
    expect(show(err, FINE)).not.toContain("Syndra itself is fine");
  });
  it("never sits beside the network sentence", () => {
    const text = show(new TypeError("Failed to fetch"), FINE);
    expect(text).toContain(NETWORK_FAULT);
    expect(text).not.toContain("is fine");
  });
});

describe("a raw network error never surfaces", () => {
  it.each(["Failed to fetch", "NetworkError when attempting to fetch resource.", "Load failed", "Network request failed"])(
    "maps %s",
    (raw) => {
      expect(describeFailure(new TypeError(raw))).toBe(NETWORK_FAULT);
      expect(describeFailure(new Error(raw))).toBe(NETWORK_FAULT);
      expect(show(new TypeError(raw))).not.toContain(raw);
    },
  );

  it("says Syndra's server didn't answer", () => {
    expect(NETWORK_FAULT).toBe("Syndra's server didn't answer.");
  });

  it("still passes an authored message through", () => {
    expect(describeFailure(new Error("It may have been removed in Zitadel."))).toBe("It may have been removed in Zitadel.");
  });
});
