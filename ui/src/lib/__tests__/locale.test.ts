import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { formatShortDate, formatBytes } from "@/lib/format";
import {
  displayLocale,
  effectiveLocale,
  formatDecimal,
  resetDisplayLocaleForTests,
  runtimeLocale,
  setDisplayLocale,
  setFallbackLocale,
  validLocale,
} from "@/lib/locale";
import { compareNames, sortNames } from "@/lib/sort";
import { formatValue } from "@/lib/usage";
import { fetchMakerspaceFormat } from "@/lib/zone";

const runtimeDefault = () => new Intl.NumberFormat().resolvedOptions().locale;

describe("the display locale", () => {
  beforeEach(resetDisplayLocaleForTests);
  afterEach(() => vi.restoreAllMocks());

  it("validates by canonical form on the server, and refuses a malformed tag", () => {
    expect(validLocale("en-gb")).toBe("en-GB");
    expect(validLocale("th-TH-u-ca-gregory")).toBe("th-TH-u-ca-gregory");
    expect(validLocale("not a tag")).toBeUndefined();
    expect(validLocale("")).toBeUndefined();
    expect(validLocale(5)).toBeUndefined();
    for (const t of ["und", "und-US", "root", "x-private"]) expect(validLocale(t)).toBeUndefined(); // as the backend refuses
  });

  it("falls back to the server's runtime default, validated; never to a literal", () => {
    expect(runtimeLocale()).toBe(validLocale(runtimeDefault()));
    setFallbackLocale(runtimeLocale());
    expect(displayLocale()).toBe(runtimeLocale());
  });

  it("with no valid fallback either, is undefined: Intl's own default on both sides", () => {
    expect(displayLocale()).toBeUndefined();
    setFallbackLocale(undefined);
    expect(displayLocale()).toBeUndefined();
  });

  it("a failed first read does not learn the fallback: a later good read still takes effect", () => {
    setFallbackLocale("en-US"); // the server's default, handed down
    expect(effectiveLocale(undefined)).toBeUndefined(); // backend down: nothing configured, nothing sent
    expect(displayLocale()).toBe("en-US");
    expect(effectiveLocale("de-DE")).toBe("de-DE"); // the next render reads it
    expect(displayLocale()).toBe("de-DE");
  });

  it("is write-once: the first configured tag stays; empty changes nothing; a different one warns once", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    setDisplayLocale(undefined);
    setDisplayLocale("de-DE");
    setDisplayLocale("fr-FR");
    setDisplayLocale("it-IT");
    setDisplayLocale("de-DE");
    expect(displayLocale()).toBe("de-DE");
    expect(warn).toHaveBeenCalledTimes(1);
  });

  it("server and client agree: the configured tag and the fallback the layout hands down are what the browser holds", () => {
    setFallbackLocale("en-US");
    const configured = effectiveLocale("en-GB"); // the server, in the layout
    resetDisplayLocaleForTests(); // a fresh browser process
    setDisplayLocale(configured);
    setFallbackLocale("en-US"); // Providers
    expect(displayLocale()).toBe("en-GB");

    resetDisplayLocaleForTests();
    setFallbackLocale("sv-SE");
    expect(effectiveLocale(undefined)).toBeUndefined(); // nothing configured
    resetDisplayLocaleForTests();
    setDisplayLocale(undefined);
    setFallbackLocale("sv-SE");
    expect(displayLocale()).toBe("sv-SE");
  });

  it("is what dates, numbers and orders speak in", () => {
    const iso = "2026-08-02T12:00:00Z";
    setDisplayLocale("en-US");
    expect(formatShortDate(iso, new Date(iso))).toBe("Aug 2");
    expect(formatValue(1234.5, "count")).toBe("1,234.5");
    expect(formatDecimal(1.5)).toBe("1.5");
    expect(formatBytes(1536)).toBe("1.5 KiB");
    resetDisplayLocaleForTests();
    setDisplayLocale("de-DE");
    expect(formatValue(1234.5, "count")).toBe("1.234,5");
    expect(formatDecimal(1.5)).toBe("1,5");
    expect(formatBytes(1536)).toBe("1,5 KiB");
  });

  it("orders names in the locale: å after z in Swedish, with the As in English", () => {
    setDisplayLocale("en-GB");
    expect(sortNames(["Åsa", "Zoe", "Anna"])).toEqual(["Anna", "Åsa", "Zoe"]);
    expect(compareNames("Åsa", "Zoe")).toBeLessThan(0);
    resetDisplayLocaleForTests();
    setDisplayLocale("sv-SE");
    expect(sortNames(["Åsa", "Zoe", "Anna"])).toEqual(["Anna", "Zoe", "Åsa"]);
  });
});

describe("what the layout reads from /healthz", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });
  it("logs once when the backend's locale is dropped, not on every render", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal("fetch", vi.fn(async () => ({ json: async () => ({ locale: "not a tag" }) })));
    await fetchMakerspaceFormat();
    await fetchMakerspaceFormat();
    expect(warn).toHaveBeenCalledTimes(1);
    expect(String(warn.mock.calls[0][0])).toContain("not a tag");
  });
});
