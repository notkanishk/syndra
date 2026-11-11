// @vitest-environment jsdom
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import type { EntitlementField } from "@/lib/fields";
import type { MyTargetView, ReadView, RecordsResult } from "@/lib/queries/useMyStorage";

/**
 * A manifest and a binary: an inventory add-on the UI has never heard of, and
 * the member's card is right with no inventory code anywhere. The card is built
 * from the contract fixtures the way the backend builds it: the kind and fields
 * from the accepted manifest, the read from the add-on's own answer, its shape
 * from the registry's name for the operation's result schema.
 *
 * Read at test time, not imported (production code never imports fixtures).
 */
const contract = (...parts: string[]) => JSON.parse(readFileSync(resolve(process.cwd(), "..", "addons", "contract", "v2", ...parts), "utf8"));

const manifest = contract("fixtures", "manifest_inventory.json") as { kind: string; entitlement_schema: EntitlementField[]; operations: Array<{ id: string; scope: string; available: boolean }> };
const answer = contract("fixtures", "operation_response_records.json") as { operation: string; result: RecordsResult };
const registry = contract("registry", "operations.json") as { operations: Record<string, { result: string }> };

const offered = manifest.operations.find((o) => o.id === answer.operation && o.scope === "member" && o.available);
const read: ReadView = {
  operation: answer.operation,
  shape: registry.operations[answer.operation].result.replace(/^common\/|\.json$/g, "") as ReadView["shape"],
  readable: true,
  result: answer.result,
};

const view: MyTargetView = {
  target: "parts",
  name: "Parts shelf",
  entitled: true,
  account: { username: "rey_mysterio" },
  kind: manifest.kind,
  entitlement_schema: manifest.entitlement_schema,
  reads: [read],
  credential: { set: false },
  reachable: true,
  health: "online",
};

describe("a kind with no module of its own, from a manifest and a binary's answer", () => {
  it("starts from fixtures that say what the test says they do", () => {
    expect(manifest.kind).toBe("inventory");
    expect(offered, "holdings.list is offered to members").toBeDefined();
    expect(read.shape).toBe("records");
  });

  it("shows both items, the overdue one's due date, and its next step", () => {
    render(<MemberTargetCard view={view} />);
    expect(screen.getByText("Soldering iron, Hakko FX-888D")).toBeInTheDocument();
    expect(screen.getByText("Digital caliper, 150 mm")).toBeInTheDocument();
    expect(screen.getByText(/Due 3 Nov/)).toBeInTheDocument();
    expect(screen.getByText("Overdue since 3 November")).toBeInTheDocument();
    expect(screen.getByText("Bring it back to the front desk")).toBeInTheDocument();
  });

  it("is the same card with no reads: the account row, and no list", () => {
    render(<MemberTargetCard view={{ ...view, reads: undefined }} />);
    expect(screen.getByText("rey_mysterio")).toBeInTheDocument();
    expect(screen.queryByText("Digital caliper, 150 mm")).toBeNull();
  });
});
