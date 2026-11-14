import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * Tests and tsc run the installed React; production's App Router pages run
 * the React Next vendors in `next/dist/compiled` (Next 15.5: a 19.2 canary).
 * So the installed React, its DOM and their types stay on the vendored
 * major.minor, and React's majors and minors move only with Next's (slice 7q):
 * a test passing on a React production never runs proves nothing.
 */
const UI = join(__dirname, "..", "..", "..");

function minorOf(version: string): string {
  const m = /^(\d+\.\d+)\./.exec(version);
  if (!m) throw new Error(`not a version: ${version}`);
  return m[1];
}

describe("React parity with Next's vendored React", () => {
  const vendored = /exports\.version = "([^"]+)"/.exec(
    readFileSync(join(UI, "node_modules/next/dist/compiled/react/cjs/react.production.js"), "utf8"),
  )?.[1];
  const pkg = JSON.parse(readFileSync(join(UI, "package.json"), "utf8"));
  const pinned: Record<string, string> = {
    react: pkg.dependencies.react,
    "react-dom": pkg.dependencies["react-dom"],
    "@types/react": pkg.devDependencies["@types/react"],
    "@types/react-dom": pkg.devDependencies["@types/react-dom"],
  };

  it("finds the React Next vendors", () => {
    expect(vendored).toMatch(/^\d+\.\d+\.\d+/);
  });

  for (const [name, version] of Object.entries(pinned)) {
    it(`${name} is on the vendored minor`, () => {
      expect(minorOf(version)).toBe(minorOf(vendored!));
    });
  }
});
