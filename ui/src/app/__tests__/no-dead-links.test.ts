import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * Every internal link points at a page that exists.
 *
 * The dashboard's health row offered "Zitadel" at `/system` — a path with no
 * page under it, so the tile 404'd — and "Waiting to be sent" at
 * `/operations`, which is a real page about the wrong thing: the feed of what
 * arrived FROM Zitadel, not the queue of what Syndra is waiting to send TO it.
 * The second is the more expensive kind, because it looks like it worked.
 *
 * Both survived every test in the product, because nothing compared an href
 * against the router. This does: it reads the App Router tree and asserts that
 * each literal internal href in the source resolves to a `page.tsx`.
 *
 * It cannot see a link that resolves and means the wrong thing — that one is
 * still a person's job. What it removes is the class where the destination does
 * not exist at all.
 */

const APP = join(import.meta.dirname, "..");
const SRC = join(APP, "..");

/** Every routable path in the App Router tree, with dynamic segments kept. */
function routes(dir: string, prefix = ""): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (entry === "page.tsx") {
      found.push(prefix === "" ? "/" : prefix);
      continue;
    }
    if (!statSync(full).isDirectory()) continue;
    // Route groups `(name)` do not appear in the URL; private `_dirs` and test
    // directories are not routes at all.
    if (entry.startsWith("_") || entry === "__tests__" || entry === "api") continue;
    if (entry.startsWith("(") && entry.endsWith(")")) {
      found.push(...routes(full, prefix));
      continue;
    }
    found.push(...routes(full, `${prefix}/${entry}`));
  }
  return found;
}

function sources(dir: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === "node_modules" || entry === "__tests__") continue;
      found.push(...sources(full));
    } else if (entry.endsWith(".tsx") || entry.endsWith(".ts")) {
      found.push(full);
    }
  }
  return found;
}

/** Whether `path` is served, allowing for `[id]`-style segments. */
function served(path: string, all: string[]): boolean {
  const parts = path.split("/").filter(Boolean);
  return all.some((route) => {
    const segs = route.split("/").filter(Boolean);
    if (segs.length !== parts.length) return false;
    return segs.every((seg, i) => (seg.startsWith("[") ? true : seg === parts[i]));
  });
}

describe("internal links", () => {
  it("all point at a page that exists", () => {
    const all = routes(APP);
    expect(all.length).toBeGreaterThan(5);

    const dead: string[] = [];
    for (const file of sources(SRC)) {
      const text = readFileSync(file, "utf8");
      // Literal hrefs only. A template or a variable is not something a static
      // check can resolve, and guessing at one would produce a guard that
      // fails on correct code.
      for (const [, href] of text.matchAll(/href=["'](\/[^"'`${}?#]*)["']/g)) {
        const path = href.replace(/\/$/, "") || "/";
        if (!served(path, all)) {
          dead.push(`${file.replace(SRC, "src")} → ${href}`);
        }
      }
    }
    expect(dead, "these links go nowhere").toEqual([]);
  });
});
