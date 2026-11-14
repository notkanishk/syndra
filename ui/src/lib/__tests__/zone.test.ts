import { afterEach, describe, expect, it, vi } from "vitest";

import { fetchMakerspaceFormat } from "@/lib/zone";

const answer = (body: unknown) => vi.stubGlobal("fetch", vi.fn(async () => ({ json: async () => body })));

describe("the zone the layout reads from the backend", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("is the backend's time_zone, healthy or not", async () => {
    answer({ status: "ok", time_zone: "Europe/London" });
    expect(await fetchMakerspaceFormat()).toEqual({ zone: "Europe/London", locale: undefined });
  });

  it("is undefined for a name Intl cannot read: UTC, labelled, never the bad name", async () => {
    answer({ status: "ok", time_zone: "Not/AZone" });
    expect(await fetchMakerspaceFormat()).toEqual({ zone: undefined, locale: undefined });
    answer({ status: "ok", time_zone: 5 });
    expect(await fetchMakerspaceFormat()).toEqual({ zone: undefined, locale: undefined });
  });

  it("is undefined, never a guessed zone, when it cannot be read", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("down"); }));
    expect(await fetchMakerspaceFormat()).toEqual({ zone: undefined, locale: undefined });
    answer({ status: "ok" });
    expect(await fetchMakerspaceFormat()).toEqual({ zone: undefined, locale: undefined });
  });

  it("carries the backend's locale beside the zone, from the same one fetch", async () => {
    answer({ status: "ok", time_zone: "Europe/London", locale: "en-gb" });
    const fetched = vi.mocked(fetch);
    expect(await fetchMakerspaceFormat()).toEqual({ zone: "Europe/London", locale: "en-GB" });
    expect(fetched).toHaveBeenCalledTimes(1);
  });

  it("drops a locale the runtime cannot use, and keeps the zone", async () => {
    answer({ status: "ok", time_zone: "Europe/London", locale: "not a tag" });
    expect(await fetchMakerspaceFormat()).toEqual({ zone: "Europe/London", locale: undefined });
    answer({ status: "ok", locale: 5 });
    expect((await fetchMakerspaceFormat()).locale).toBeUndefined();
  });
});
