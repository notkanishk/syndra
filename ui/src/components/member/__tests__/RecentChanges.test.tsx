// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RecentChanges, dayLabel } from "@/components/member/RecentChanges";
import type { MyChange } from "@/lib/queries/useMyChanges";

vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id === "op" ? "Kanishk Singh" : id}</span>,
  BundleName: ({ id }: { id: string }) => <span>{id === "b1" ? "Lab Tech" : id}</span>,
}));

const AT = "2026-10-02T09:00:00Z";
const roleName = (_p: string, key: string) => ({ op: "Laser cutter · Operator", sup: "Laser cutter · Supervisor" })[key] ?? key;

function row(change: Partial<MyChange>) {
  const items = [{ kind: "given", at: AT, project_id: "p1", role_key: "op", via: null, ...change } as MyChange];
  const { container } = render(<RecentChanges items={items} roleName={roleName} />);
  return container.querySelector(".row-divider") as HTMLElement;
}

/** Each kind: a dot and a word, the sentence with the role in bold, the note quoted with its decider. */
describe("Recent changes rows", () => {
  it("Approved, with its end and the decider's note", () => {
    const r = row({ kind: "request_approved", by: "op", note: "Book the evening slots.", until: "2026-10-20T09:00:00Z" });
    expect(r.textContent).toContain("Approved");
    expect(r.textContent).toContain("Your request for Laser cutter · Operator was approved, until 20 Oct.");
    expect(r.querySelector("strong")!.textContent).toBe("Laser cutter · Operator");
    expect(r.textContent).toContain("“Book the evening slots.” — Kanishk Singh");
    expect(r.querySelector(".bg-healthy")).toBeTruthy();
  });

  it("Not approved, with Ask again prefilled", () => {
    const r = row({ kind: "request_declined", role_key: "sup", by: "op", note: "Do the induction first." });
    expect(r.textContent).toContain("Not approved");
    expect(r.textContent).toContain("Your request for Laser cutter · Supervisor was not approved.");
    const again = screen.getByText("Ask again").closest("a")!;
    expect(again.getAttribute("href")).toBe("/requests?project=p1&role=sup");
  });

  it("Withdrawn", () => {
    const r = row({ kind: "request_withdrawn" });
    expect(r.textContent).toContain("Withdrawn");
    expect(r.textContent).toContain("You withdrew your request for Laser cutter · Operator.");
    expect(r.textContent).not.toContain("was withdrawn");
    expect(screen.queryByText("Ask again")).toBeNull();
  });

  it("a request to keep a role: Kept with its end, Not approved with what still holds, Ask again keeps it", () => {
    const kept = row({ kind: "request_approved", request_kind: "extend", until: "2026-12-18T12:00:00Z" });
    expect(kept.textContent).toContain("Kept");
    expect(kept.textContent).toContain(
      "You keep Laser cutter · Operator until 18 Dec. Your request to keep it was approved.",
    );
    expect(row({ kind: "request_approved", request_kind: "extend" }).textContent).toContain(
      "You keep Laser cutter · Operator. Your request to keep it was approved.",
    );
    expect(row({ kind: "request_withdrawn", request_kind: "extend" }).textContent).toContain(
      "You withdrew your request to keep Laser cutter · Operator.",
    );
    const r = row({ kind: "request_declined", request_kind: "extend" });
    expect(r.textContent).toContain("Not approved");
    expect(r.textContent).toContain("Your request to keep Laser cutter · Operator was not approved.");
    expect(r.textContent).not.toContain("still ends");
    expect(screen.getAllByText("Ask again").at(-1)!.closest("a")!.getAttribute("href")).toBe(
      "/requests?project=p1&role=op&extend=1",
    );
  });

  // The request remembers the end they had and the end approving set (6.37), so the
  // row says it from there and not from a grant that may have moved since.
  it("Kept says the end it now has instead of the one it had", () => {
    const r = row({
      kind: "request_approved",
      request_kind: "extend",
      ended_before: "2099-12-18T12:00:00Z",
      ends_after: "2100-01-18T12:00:00Z",
    });
    expect(r.textContent).toContain(
      "You keep Laser cutter · Operator. Your request to keep it was approved. It now ends 18 Jan 2100 instead of 18 Dec 2099.",
    );
  });

  it("Not approved says the end it still has, from the request, even when the page knows another", () => {
    const items = [
      { kind: "request_declined", at: AT, project_id: "p1", role_key: "op", via: null, request_kind: "extend", ended_before: "2099-12-18T12:00:00Z" } as MyChange,
    ];
    const { container } = render(
      <RecentChanges items={items} roleName={roleName} endOf={() => "2099-01-01T12:00:00Z"} />,
    );
    expect(container.textContent).toContain("was not approved. It still ends 18 Dec 2099.");
    expect(container.textContent).not.toContain("1 Jan");
  });

  it("a declined request to keep says what still ends, when the page knows it", () => {
    const items = [{ kind: "request_declined", at: AT, project_id: "p1", role_key: "op", via: null, request_kind: "extend" } as MyChange];
    const { container } = render(
      <RecentChanges items={items} roleName={roleName} endOf={() => "2099-12-18T12:00:00Z"} />,
    );
    expect(container.textContent).toContain(
      "Your request to keep Laser cutter · Operator was not approved. It still ends 18 Dec 2099.",
    );
  });

  it("Given, by hand, through a bundle, and automatically", () => {
    expect(row({ by: "op" }).textContent).toContain("Laser cutter · Operator was given to you.");
    expect(row({ via: { bundle_id: "b1" } }).textContent).toContain(
      "Laser cutter · Operator was given to you, because you’re in Lab Tech.",
    );
    // A rule, with no trigger role the page can name.
    expect(row({ via: { rule_id: "r1" } }).textContent).toContain(
      "Laser cutter · Operator was given automatically by a makerspace rule.",
    );
  });

  it("names the role a rule follows, when the page can tell", () => {
    const items = [{ kind: "given", at: AT, project_id: "p1", role_key: "op", via: { rule_id: "r1" } } as MyChange];
    const { container } = render(
      <RecentChanges items={items} roleName={roleName} triggerRole={() => "Door access · Member"} />,
    );
    const r = container.querySelector(".row-divider") as HTMLElement;
    expect(r.textContent).toContain("Laser cutter · Operator came automatically, because you have Door access · Member.");
    expect(r.textContent).not.toContain("makerspace rule");
  });

  it("Revoked, in the danger tone", () => {
    const r = row({ kind: "taken_away", note: "Studio bookings close for the term." });
    expect(r.textContent).toContain("Revoked");
    expect(r.textContent).toContain("Laser cutter · Operator was revoked.");
    expect(r.querySelector(".bg-danger")).toBeTruthy();
    // No decider, so the note is quoted without a name.
    expect(r.textContent).not.toContain("—");
  });

  it("the date sits in its own seat", () => {
    const r = row({});
    const seat = r.lastElementChild as HTMLElement;
    expect(seat.className).toContain("w-[110px]");
    expect(seat.textContent).toBe("2 Oct");
    expect(dayLabel(new Date().toISOString())).toBe("Today");
  });
});

/** N19: an approval not on the account yet, said in the member's words. */
describe("Recent changes: approved, not there yet", () => {
  it("waits for staff to check the role", () => {
    const r = row({ kind: "request_approved", role_key: "sup", by: "op", waiting: true, wait_reason: "role_waits" });
    expect(r.querySelector(".bg-warn")).toBeTruthy();
    expect(r.textContent).toContain(
      "Your request for Laser cutter · Supervisor was approved. It isn't on your account yet: makerspace staff check this role before it reaches you, and nothing more is needed from you.",
    );
    expect(r.textContent).not.toMatch(/Waiting for you to send/);
  });

  it("says Syndra couldn't reach the sign-in system when the send halted", () => {
    const r = row({ kind: "request_approved", role_key: "sup", waiting: true, wait_reason: "halted:zitadel_offline" });
    expect(r.textContent).toContain(
      "It isn't on your account yet: Syndra couldn't reach the sign-in system when it was approved; staff will send it.",
    );
    expect(r.textContent).not.toContain("zitadel_offline");
  });

  it("links to all your requests", () => {
    render(<RecentChanges items={[]} roleName={roleName} />);
    expect(screen.getByText("All your requests →").closest("a")!.getAttribute("href")).toBe("/requests");
  });
});
