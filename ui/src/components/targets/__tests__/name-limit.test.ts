import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

import { NAME_MAX } from "@/components/targets/TargetName";

/**
 * A system's name limit is written in three places, and one decides it: the
 * schema's CHECK (migration 000091). The backend's refusal says the number to
 * the operator, and the Name field stops typing at it. Read from the sources
 * themselves, the way the proxy test reads the router, so none of the three
 * can move alone.
 */
describe("a system's name limit is one number", () => {
  const backend = resolve(__dirname, "../../../../../backend");
  const check = readFileSync(resolve(backend, "db/migrations/000091_target_name.up.sql"), "utf8");
  const refusal = readFileSync(resolve(backend, "internal/db/target_name.go"), "utf8");

  it("is the CHECK's, the refusal's and the field's", () => {
    const inCheck = /char_length\(name\) <= (\d+)/.exec(check)?.[1];
    const inRefusal = /ErrTargetNameInvalid = errors\.New\("A name is at most (\d+) characters\."\)/.exec(refusal)?.[1];
    expect(inCheck).toBeDefined();
    expect(inRefusal).toBeDefined();
    expect(Number(inCheck)).toBe(NAME_MAX);
    expect(Number(inRefusal)).toBe(NAME_MAX);
  });
});
