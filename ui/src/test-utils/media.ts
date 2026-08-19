/**
 * `window.matchMedia` for jsdom, which implements none of it.
 *
 * Three production call sites reach for it today — the theme provider, the
 * login ceremony and the drift chime — and every one of them writes
 * `window.matchMedia?.(…)`. That optional chaining is why the suite passes
 * without this file: in every test the call returns `undefined` and each site
 * silently takes its no-preference branch. So those branches are not merely
 * untested, they are the *only* branches the suite has ever run.
 *
 * The default here is `false` for every query, which is the same answer those
 * three sites were already inferring from `undefined`. Nothing about existing
 * behaviour moves; what changes is that the other branch becomes reachable.
 *
 * jsdom still loads no stylesheets and computes no layout, so this does NOT
 * make a CSS breakpoint observable — a `md:hidden` element is present and
 * visible in every test regardless of any width set here. Only code that
 * *asks* matchMedia a question can be asserted, which is the reason structural
 * decisions on touch (nav shape, dialog versus sheet) are made in JS rather
 * than inferred from a class.
 */

interface MediaListener {
  (event: MediaQueryListEvent): void;
}

/** Queries whose answer has been set for the current test. Default: false. */
const answers = new Map<string, boolean>();

/**
 * Live lists, so a change can notify the components already holding one.
 *
 * Arrays rather than Sets throughout: this file is type-checked by the Next
 * build, whose target predates iterating a Set without downlevelIteration.
 */
const lists: { media: string; list: MediaQueryList; listeners: MediaListener[] }[] = [];

/**
 * Answer `query` with `matches`, notifying anything already listening.
 *
 * Matching is exact on the query string rather than parsed: a test that asks
 * about `(prefers-reduced-motion: reduce)` and a component that asks about
 * `(prefers-reduced-motion:reduce)` are asking the same thing and would not
 * match here — which is a loud failure in a test rather than a quiet wrong
 * answer in a browser, and is the trade this file wants.
 */
export function setMediaQuery(query: string, matches: boolean): void {
  answers.set(query, matches);
  const event = { matches, media: query } as MediaQueryListEvent;
  lists.forEach((entry) => {
    if (entry.media !== query) return;
    (entry.list as { matches: boolean }).matches = matches;
    entry.listeners.forEach((listener) => listener(event));
    entry.list.onchange?.(event);
  });
}

/** Forget every answer and every listener. Called between tests. */
export function resetMediaQueries(): void {
  answers.clear();
  lists.length = 0;
}

/** Installs the stub on `window`. Idempotent. */
export function installMatchMedia(): void {
  if (typeof window === "undefined") return;

  window.matchMedia = (query: string): MediaQueryList => {
    const listeners: MediaListener[] = [];
    const list = {
      media: query,
      matches: answers.get(query) ?? false,
      onchange: null,
      addEventListener: (type: string, listener: MediaListener) => {
        if (type === "change") listeners.push(listener);
      },
      removeEventListener: (type: string, listener: MediaListener) => {
        const at = listeners.indexOf(listener);
        if (type === "change" && at !== -1) listeners.splice(at, 1);
      },
      // The deprecated pair, because Safari needed it long enough that
      // libraries still call it and a stub that omits it throws rather than
      // degrading.
      addListener: (listener: MediaListener) => listeners.push(listener),
      removeListener: (listener: MediaListener) => {
        const at = listeners.indexOf(listener);
        if (at !== -1) listeners.splice(at, 1);
      },
      dispatchEvent: () => true,
    } as unknown as MediaQueryList;

    lists.push({ media: query, list, listeners });
    return list;
  };
}
