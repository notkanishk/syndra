import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { expect, it } from "vitest";

const SRC = join(import.meta.dirname, "../../..");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry !== "__tests__" && entry !== "node_modules") walk(full, out);
    } else if (/\.tsx$/.test(entry)) out.push(full);
  }
  return out;
}

/**
 * A-22 G24: two buttons in a pair never share a verb. A footer whose act
 * begins with "Cancel" (Cancel giving it, Cancel the ...) passes a dismiss
 * that is not the default "Cancel": Keep it, Keep it waiting, Keep them.
 */
it("a footer whose act cancels never also dismisses with Cancel", () => {
  const offenders: string[] = [];
  for (const file of walk(join(SRC, "components")).concat(walk(join(SRC, "app")))) {
    const source = readFileSync(file, "utf8");
    for (const m of source.matchAll(/<ModalFooter\b[\s\S]*?<\/ModalFooter>/g)) {
      const block = m[0];
      const act = /(>|\{\s*|["`])\s*Cancel (giving|removing|the|it|all)\b/.test(block.replace(/cancel=\{\{[\s\S]*?\}\}/, ""));
      const dismissCancel = /cancel=\{\{/.test(block) && !/label:/.test(block.match(/cancel=\{\{[\s\S]*?\}\}/)![0]);
      if (act && dismissCancel) offenders.push(file.slice(SRC.length + 1));
    }
  }
  expect(offenders).toEqual([]);
});
