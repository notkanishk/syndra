import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { STOPPED_NEXT } from "@/lib/drain-outcome";
import { ADVANCED_NAV, type NavEntry } from "@/lib/nav";
import { MAY_HAVE_GONE_THROUGH, SEND_IT, SENDS_ITSELF_NEXT } from "@/lib/outcome";

/**
 * Re-audit, 6 Nov 2026: a next step that sends the reader to a place that
 * doesn't exist is a false next step ("Find them in Audit" for a give-up Audit
 * never named; "Open Zitadel › Users to read it again" for a read that page
 * never makes). This guard holds the class: every "A › B" the product says
 * names a real place, and every place a shared next step names is one.
 */
const SRC = join(__dirname, "..", "..");
const APP = join(SRC, "app");

function leaves(entries: NavEntry[]): Array<{ group?: string; label: string; href: string }> {
  return entries.flatMap((e) =>
    e.kind === "group" ? e.children.map((c) => ({ group: e.label, label: c.label, href: c.href })) : [{ label: e.label, href: e.href }],
  );
}
const NAV = leaves(ADVANCED_NAV);
const LABELS = new Set(NAV.map((l) => l.label));

const slug = (s: string) => s.toLowerCase().replace(/\s+/g, "-");

/** "A › B" is a group and its row, or a row and a page under it. */
function resolvesExactly(a: string, b: string): boolean {
  if (NAV.some((l) => l.group === a && l.label === b)) return true;
  const row = NAV.find((l) => l.label === a);
  return Boolean(row && existsSync(join(APP, ...row.href.split("/").filter(Boolean), slug(b))));
}

/** The match runs on into the sentence ("Roles does"): any leading run of B's words will do. */
function resolves(a: string, b: string): boolean {
  const words = b.split(" ");
  return words.some((_, i) => resolvesExactly(a, words.slice(0, i + 1).join(" ")));
}

/** Ours: a crumb that starts at one of the product's own places. Another app's menus ("Go › Connect to Server") are not. */
const GROUPS = new Set(NAV.flatMap((l) => (l.group ? [l.group] : [])));
const ours = (a: string) => LABELS.has(a) || GROUPS.has(a);

function sources(dir: string, out: string[] = []): string[] {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) {
      if (name !== "__tests__" && name !== "node_modules") sources(p, out);
    } else if (/\.tsx?$/.test(p) && !/\.test\.tsx?$/.test(p)) out.push(p);
  }
  return out;
}

/** What a reader sees: string literals and JSX text, never comments. */
function shown(text: string): string {
  return text
    .split("\n")
    .filter((line) => !/^\s*(\/\/|\*|\/\*|\{\/\*)/.test(line))
    .join("\n");
}

describe("every breadcrumb the product says is a real place", () => {
  const crumbs: Array<[string, string, string]> = [];
  for (const file of sources(SRC)) {
    for (const m of shown(readFileSync(file, "utf8")).matchAll(/([A-Z][A-Za-z]+(?: [a-z]+)*) › ([A-Z][A-Za-z]+(?: [a-z]+)*)/g)) {
      if (ours(m[1])) crumbs.push([file.slice(SRC.length + 1), m[1], m[2]]);
    }
  }
  it("finds the breadcrumbs", () => expect(crumbs.length).toBeGreaterThan(5));
  for (const [file, a, b] of crumbs) {
    it(`${file}: ${a} › ${b}`, () => expect(resolves(a, b)).toBe(true));
  }
});

describe("the shared next steps name places that exist", () => {
  it("Send, and Home, are real places", () => {
    expect(SEND_IT).toContain("Pending changes");
    expect(LABELS.has("Pending changes")).toBe(true);
    expect(SENDS_ITSELF_NEXT).toContain("on Home");
    expect(LABELS.has("Home")).toBe(true);
  });

  it("every place a partial write sends the reader to is a place", () => {
    for (const where of Object.values(MAY_HAVE_GONE_THROUGH)) {
      const named = [...LABELS].some((label) => where.includes(label)) || /page|›/.test(where);
      expect(named, where).toBe(true);
      for (const m of where.matchAll(/([A-Z][A-Za-z]+(?: [a-z]+)*) › ([A-Z][A-Za-z]+(?: [a-z]+)*)/g)) {
        expect(resolves(m[1], m[2]), where).toBe(true);
      }
    }
  });

  it("the give-up step names sections the pages really have", () => {
    expect(STOPPED_NEXT).toMatch(/Syndra stopped sending, on Pending changes/);
    const pending = readFileSync(join(APP, "governance", "pending", "page.tsx"), "utf8");
    expect(pending).toContain('title="Syndra stopped sending"');
    expect(STOPPED_NEXT).toContain("Drift");
    expect(LABELS.has("Drift")).toBe(true);
  });
});
