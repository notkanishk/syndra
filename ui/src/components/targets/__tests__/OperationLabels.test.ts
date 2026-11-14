// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import { OPERATION_LABEL } from "@/components/targets/TargetOverview";

const registry = JSON.parse(
  readFileSync(resolve(__dirname, "../../../../../addons/contract/v2/registry/operations.json"), "utf8"),
) as { operations: Record<string, unknown> };

describe("the operations list", () => {
  // The page says "Something TrueNAS can do that Syndra has no name for" for an id without a
  // label; password.set shipped like that. Every id the registry can offer has its words.
  for (const id of Object.keys(registry.operations)) {
    it(`${id} has a label`, () => {
      expect(OPERATION_LABEL[id]).toBeTruthy();
    });
  }

  // Every system shows these words, and the manifest gives an operation no description of its own, so
  // a label names no system and says nothing of what one does with files (TrueNAS keeps the home
  // folder on user.delete, middleware TS-25.10.7 plugins/account.py do_delete: that is its fact).
  it("makes no claim about any one system", () => {
    for (const [id, label] of Object.entries(OPERATION_LABEL)) {
      expect(label, id).not.toMatch(/TrueNAS|\bfiles?\b/i);
    }
    expect(OPERATION_LABEL["account.purge"]).toBe("Deletes an account");
  });
});
