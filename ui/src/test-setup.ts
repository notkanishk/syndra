import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { createElement, type ComponentType } from "react";
import { afterEach, beforeEach, vi } from "vitest";

import { resetDisplayLocaleForTests, setDisplayLocale } from "@/lib/locale";
import { installMatchMedia, resetMediaQueries } from "@/test-utils/media";

// Every test runs on the same day: 9 Nov 2026, noon in the makerspace's zone,
// with the clock moving at real speed from there. Fixtures are written as
// dates in 2026 and the app (rightly) adds the year to a date outside the
// current one, so on the real 1 Jan 2027 thirty-five tests went red at once,
// and one on 31 Dec; a retry time fixed at 8 Nov 14:05 did the same on 9 Nov.
// A test that needs another "now" still sets it (vi.useFakeTimers +
// setSystemTime), and vi.useRealTimers comes back to this pinned clock.
const PINNED_NOW = Date.UTC(2026, 10, 9, 6, 30);
const RealDate = Date;
const realStart = RealDate.now();
class PinnedDate extends RealDate {
  constructor(...args: ConstructorParameters<DateConstructor> | []) {
    if (args.length === 0) super(PinnedDate.now());
    else super(...(args as [string | number | Date]));
  }
  static now(): number {
    return PINNED_NOW + (RealDate.now() - realStart);
  }
}
globalThis.Date = PinnedDate as DateConstructor;

// `next/dynamic` loads a component after first paint, which is right in a
// browser and means a test would have to await every dialog it opens. Here the
// load starts when the module does and `beforeEach` waits for it, so a lazily
// loaded component is there the moment a test renders it.
const dynamicLoads: Promise<unknown>[] = [];
vi.mock("next/dynamic", () => ({
  default: (load: () => Promise<ComponentType<never> | { default: ComponentType<never> }>) => {
    let Loaded: ComponentType<never> | undefined;
    dynamicLoads.push(
      load().then((m) => {
        Loaded = "default" in m ? m.default : m;
      }),
    );
    return (props: object) => (Loaded ? createElement(Loaded as ComponentType<object>, props) : null);
  },
}));
beforeEach(() => Promise.all(dynamicLoads));

// The forecast asks which roles wait; most surfaces under test have no list to
// read. Said once here, so a test about a waiting role overrides it locally.
vi.mock("@/lib/queries/useRoleWaits", () => ({ useRoleWaits: () => () => false }));

// jsdom implements no matchMedia. Installed once per file rather than per
// test, because components capture it during render; reset between tests so a
// query answered in one does not leak into the next. See test-utils/media.ts
// for why the default answer is `false` for everything.
installMatchMedia();

// Node 22 ships an experimental `localStorage` global, and this runner starts
// Node with `--localstorage-file` and no path, so that global exists with no
// working `setItem`. vitest then copies Node's globals onto the jsdom window,
// so BOTH `localStorage` and `window.localStorage` are the broken one and
// jsdom's own never gets a look in. In a browser these are the same working
// object, which is why `localStorage.setItem(…)` in application code is right
// and only the harness disagrees.
//
// Everything in this product that remembers a choice goes through it — the
// theme, the Basic/Advanced view, the drift chime — so none of them could be
// tested until this existed, and none of them were.
const memoryStorage = (): Storage => {
  const entries = new Map<string, string>();
  return {
    get length() {
      return entries.size;
    },
    // Array.from rather than spread: this file is type-checked by the Next
    // build, whose target predates iterating a Map without downlevelIteration.
    key: (index: number) => Array.from(entries.keys())[index] ?? null,
    getItem: (key: string) => entries.get(key) ?? null,
    setItem: (key: string, value: string) => void entries.set(key, String(value)),
    removeItem: (key: string) => void entries.delete(key),
    clear: () => entries.clear(),
  } as Storage;
};

if (typeof window !== "undefined") {
  const storage = memoryStorage();
  Object.defineProperty(window, "localStorage", { value: storage, configurable: true });
  Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true });
}

// jsdom implements no clipboard either, and copy rows now ask whether one
// exists BEFORE they offer to copy — so without this every test would see the
// insecure-origin fallback and assert against a browser nobody was describing.
// The default is a working clipboard, which is the secure origin most of the
// product assumes; a test that means the http LAN removes it deliberately.
function installClipboard() {
  if (typeof navigator === "undefined") return;
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: () => Promise.resolve() },
    configurable: true,
  });
}

installClipboard();

// Tests state their locale as the product learns it (the backend's LOCALE), not
// the machine's: fixtures read "2 Aug", which is en-GB.
beforeEach(() => {
  resetDisplayLocaleForTests();
  setDisplayLocale("en-GB");
});

beforeEach(() => {
  resetMediaQueries();
  installMatchMedia();
  installClipboard();
  if (typeof window !== "undefined") {
    window.localStorage.clear();
    window.sessionStorage.clear();
  }
});

// @testing-library/react auto-cleans only with vitest globals:true. We import
// explicitly, so wire cleanup() into afterEach so jsdom DOM doesn't leak
// across tests (would otherwise produce "multiple role=dialog" errors).
afterEach(() => {
  cleanup();
  if (typeof window !== "undefined") (window as { __motionInTests?: boolean }).__motionInTests = false;
});

// The tests' zone, as the backend would hand it over (test config; the product
// writes none). vitest.config.ts sets TZ to the same name for `new Date(y, m, d)` fixtures.
import { setMakerspaceZone } from "@/lib/time";
setMakerspaceZone(process.env.TZ);
