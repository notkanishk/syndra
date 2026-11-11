import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { OWN_READS } from "@/components/kinds/storage/state";

/**
 * The storage card draws the reads it names from `view.storage`; the generic
 * renderers draw the rest. A read named here that the contract does not answer
 * with the storage profile would vanish from both, so the list is held to the
 * registry the backend reads.
 */
const registry = JSON.parse(readFileSync(resolve(__dirname, "../../../../../../addons/contract/v2/registry/operations.json"), "utf8")) as {
  operations: Record<string, { result?: string }>;
};

describe("OWN_READS", () => {
  it("names at least one read", () => {
    expect(OWN_READS.length).toBeGreaterThan(0);
  });

  for (const id of OWN_READS) {
    it(`${id} is an operation in the registry whose result is the storage status profile`, () => {
      expect(registry.operations[id]?.result).toBe("profiles/storage/status.json");
    });
  }
});
