// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EventSentence, describeSends } from "@/components/audit/EventLine";
import { truenasName } from "@/lib/__fixtures__/field-schema-mock";
import type { AuditEntry, AuditOperation } from "@/lib/queries/useAudit";

// A-13 §4, "In the record": one change, its sends listed faint, the second
// marked "Sent again automatically" with both times and which was confirmed.

vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);
vi.mock("@/components/names", () => ({ UserName: ({ id }: { id: string }) => <span>{id}</span> }));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: undefined, resolved: true }) }),
}));

// Today, in local time: "at 09:41 today" is only true of today.
const at = (hms: string) => {
  const [h, m, sec] = hms.split(":").map(Number);
  const d = new Date();
  d.setHours(h, m, sec, 0);
  return d.toISOString();
};

function send(op: Partial<AuditOperation>): AuditEntry {
  return {
    id: "a1",
    actor_id: "u1",
    target_id: "u1",
    action: "addon.truenas.password.set.dispatched",
    resource_id: "op1",
    created_at: at("09:41:02"),
    operation: { operation: "password.set", status: "indeterminate", attempt: 1, sent_at: at("09:41:02"), ...op },
  };
}

describe("a password change in the record", () => {
  it("confirmed on the second send", () => {
    const { headline, sends } = describeSends(
      send({ retry: { id: "op2", status: "succeeded", sent_at: at("09:41:14") } }),
      truenasName,
    );
    expect(headline).toBe("Storage password changed on TrueNAS");
    expect(sends).toBe(
      "Sent at 09:41:02 with no answer. Sent again automatically at 09:41:14, and TrueNAS confirmed that one.",
    );
  });

  it("never confirmed, then settled by a later change", () => {
    const { headline, sends } = describeSends(
      send({
        retry: { id: "op2", status: "indeterminate", sent_at: at("09:41:14") },
        resolution: "superseded",
        resolved_at: at("11:52:00"),
      }),
      truenasName,
    );
    expect(headline).toBe("Storage password change on TrueNAS, not confirmed");
    expect(sends).toBe(
      "Sent at 09:41:02 with no answer. Sent again automatically at 09:41:14 with no answer. Settled by their change at 11:52, which TrueNAS confirmed.",
    );
  });

  it("the change that settles it says so", () => {
    const { headline, sends } = describeSends(send({ status: "succeeded", supersedes: [at("09:41:02")] }), truenasName);
    expect(headline).toBe("Storage password changed on TrueNAS");
    expect(sends).toBe("TrueNAS confirmed it at once. It also settles the change at 09:41 today.");
  });

  it("a plain confirmed change needs no sends line", () => {
    expect(describeSends(send({ status: "succeeded" }), truenasName).sends).toBe("");
  });

  it("a resend's own row, in Audit, says it was sent again automatically", () => {
    render(<EventSentence entry={send({ attempt: 2, retry_of: "op1", status: "succeeded", sent_at: at("09:41:14") })} />);
    expect(screen.getByText("Storage password change on TrueNAS, sent again automatically")).toBeTruthy();
    expect(screen.getByText("Sent again automatically at 09:41:14, and TrueNAS confirmed that one.")).toBeTruthy();
  });

  it("an operator's answer reads as what they said", () => {
    render(
      <EventSentence
        entry={{
          id: "a2", actor_id: "op1", target_id: "u9", action: "addon.truenas.account.purge.answered",
          resource_id: "op1", created_at: at("12:00:00"), details: { happened: false },
        }}
      />,
    );
    expect(document.body.textContent).toBe("Said it didn't happen · a delete of an account and its files TrueNAS never answered");
  });
});
