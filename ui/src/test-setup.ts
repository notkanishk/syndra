import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach, beforeEach } from "vitest";

import { installMatchMedia, resetMediaQueries } from "@/test-utils/media";

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

beforeEach(() => {
  resetMediaQueries();
  installMatchMedia();
  installClipboard();
  if (typeof window !== "undefined") window.localStorage.clear();
});

// @testing-library/react auto-cleans only with vitest globals:true. We import
// explicitly, so wire cleanup() into afterEach so jsdom DOM doesn't leak
// across tests (would otherwise produce "multiple role=dialog" errors).
afterEach(() => {
  cleanup();
});
