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

  // TrueNAS's user.delete removes the account and its .ssh folder only (middleware TS-25.10.7,
  // plugins/account.py do_delete): the home directory and its files stay, so no label may say they go.
  it("says a purge leaves the files", () => {
    expect(OPERATION_LABEL["account.purge"]).not.toMatch(/and its files/);
    expect(OPERATION_LABEL["account.purge"]).toMatch(/files stay/);
  });
});
