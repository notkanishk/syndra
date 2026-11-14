import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchMakerspaceZone } from "@/lib/zone";

const answer = (body: unknown) => vi.stubGlobal("fetch", vi.fn(async () => ({ json: async () => body })));

describe("the zone the layout reads from the backend", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("is the backend's time_zone, healthy or not", async () => {
    answer({ status: "ok", time_zone: "Europe/London" });
    expect(await fetchMakerspaceZone()).toBe("Europe/London");
  });

  it("degrades to the backend's own default when it cannot be read", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("down"); }));
    expect(await fetchMakerspaceZone()).toBe("Asia/Kolkata");
    answer({ status: "ok" });
    expect(await fetchMakerspaceZone()).toBe("Asia/Kolkata");
  });
});
