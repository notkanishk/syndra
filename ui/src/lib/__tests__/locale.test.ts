import { beforeEach, describe, expect, it } from "vitest";

import { formatShortDate } from "@/lib/format";
import { displayLocale, effectiveLocale, resetDisplayLocaleForTests, setDisplayLocale, validLocale } from "@/lib/locale";
import { formatValue } from "@/lib/usage";

const runtimeDefault = () => new Intl.NumberFormat().resolvedOptions().locale;

describe("the display locale", () => {
  beforeEach(resetDisplayLocaleForTests);

  it("is the runtime's own default until one is learned, never a literal", () => {
    expect(displayLocale()).toBe(runtimeDefault());
    expect(effectiveLocale(undefined)).toBe(runtimeDefault());
  });

  it("validates: canonical form, malformed and unsupported refused", () => {
    expect(validLocale("en-gb")).toBe("en-GB");
    expect(validLocale("not a tag")).toBeUndefined();
    expect(validLocale("")).toBeUndefined();
    expect(validLocale(5)).toBeUndefined();
    expect(validLocale("xx-ZZ")).toBeUndefined(); // well formed, but no data for it
  });

  it("is write-once: the first valid tag stays; a bad or different one changes nothing", () => {
    setDisplayLocale("nonsense tag");
    expect(displayLocale()).toBe(runtimeDefault());
    setDisplayLocale("de-DE");
    setDisplayLocale("fr-FR");
    setDisplayLocale(undefined);
    expect(displayLocale()).toBe("de-DE");
  });

  it("server and client agree: the effective locale the layout hands down is the one the browser then holds", () => {
    const sent = effectiveLocale("en-GB"); // the server, in the layout
    resetDisplayLocaleForTests(); // a fresh browser process
    setDisplayLocale(sent); // Providers
    expect(displayLocale()).toBe(sent);

    resetDisplayLocaleForTests();
    const fallback = effectiveLocale(undefined); // nothing configured: the server's default, passed down
    resetDisplayLocaleForTests();
    setDisplayLocale(fallback);
    expect(displayLocale()).toBe(runtimeDefault());
  });

  it("is what the formatters speak in", () => {
    const iso = "2026-08-02T12:00:00Z";
    setDisplayLocale("en-US");
    expect(formatShortDate(iso, new Date(iso))).toBe("Aug 2");
    expect(formatValue(1234.5, "count")).toBe("1,234.5");
    resetDisplayLocaleForTests();
    setDisplayLocale("de-DE");
    expect(formatValue(1234.5, "count")).toBe("1.234,5");
    expect(formatShortDate(iso, new Date(iso))).toContain("2.");
  });
});
