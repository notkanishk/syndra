// @vitest-environment jsdom
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Modal } from "@/components/ui/Modal";

/**
 * `position: fixed` is fixed to the viewport only while no ancestor has a
 * transform, filter or backdrop-filter, and an ancestor animating `transform`
 * counts. A dialog rendered where it was written was fixed to the card it was
 * opened from: clipped by it, drawn over its rows. Every fixed overlay goes to
 * <body> through `ToBody` (or `createPortal`), so no ancestor can capture it.
 */

const SRC = join(import.meta.dirname, "../../..");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      if (entry !== "__tests__") walk(path, out);
    } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
      out.push(path);
    }
  }
  return out;
}

// A `fixed` utility in a class list, or a fixed inline style.
const FIXED = /["'`\s]fixed\s+(?:inset|top-|bottom-|left-|right-|z-)|position:\s*["']fixed["']/;

describe("every fixed overlay renders under <body>", () => {
  it("never leaves one where an ancestor's transform can capture it", () => {
    const offenders = walk(SRC)
      .filter((file) => {
        const src = readFileSync(file, "utf8");
        return FIXED.test(src) && !/<ToBody>|createPortal\(/.test(src);
      })
      .map((file) => relative(SRC, file));
    expect(offenders, "wrap the overlay in <ToBody> from components/ui/Modal").toEqual([]);
  });

  it("puts a dialog opened inside a transformed card under <body>", () => {
    render(
      <div data-testid="card" style={{ transform: "translateY(0)" }}>
        <Modal open onClose={() => {}} labelledBy="t">
          <h2 id="t">Reconcile accounts</h2>
        </Modal>
      </div>,
    );
    const dialog = screen.getByRole("dialog");
    expect(screen.getByTestId("card").contains(dialog)).toBe(false);
    expect(dialog.parentElement).toBe(document.body);
  });
});
