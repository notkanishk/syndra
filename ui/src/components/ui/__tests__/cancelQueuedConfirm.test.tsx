// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CancelQueuedConfirm } from "@/components/ui/CancelQueuedConfirm";
import { confirmCancellations, type QueuedChange } from "@/lib/cancel-confirm";

vi.mock("@/components/names", () => ({
  ProjectName: ({ id }: { id: string }) => <span>{id === "pNas" ? "TrueNAS" : id}</span>,
  UserName: ({ id }: { id: string }) => <span>{({ u1: "test user", "op-2": "Sam" } as Record<string, string>)[id] ?? id}</span>,
}));

const queued: QueuedChange = {
  id: "q1",
  op_type: "revoke",
  user_id: "u1",
  project_id: "pNas",
  role_keys: ["community"],
  source: "direct",
  initiated_by: "op-2",
  reason: "left the course",
  queued_at: "2026-10-15T09:00:00Z",
};

describe("the question before cancelling a queued change", () => {
  it("names what, for whom, who queued it and why; keeping it is first and is the answer to Escape", async () => {
    render(<CancelQueuedConfirm />);
    let answer: Promise<boolean> = Promise.resolve(true);
    act(() => {
      answer = confirmCancellations([queued]);
    });
    expect(screen.getByText("This cancels a change that hasn't been sent")).toBeTruthy();
    const row = screen.getByText("Removing").closest("li")!;
    expect(row.textContent).toContain("TrueNAS / community for test user");
    expect(row.textContent).toContain("Queued by Sam");
    expect(row.textContent).toContain("left the course");
    const buttons = screen.getAllByRole("button").filter((b) => /Keep it|Cancel it/.test(b.textContent ?? ""));
    expect(buttons[0].textContent).toBe("Keep it");

    fireEvent.keyDown(window, { key: "Escape" });
    await expect(answer).resolves.toBe(false);
    expect(screen.queryByText("This cancels a change that hasn't been sent")).toBeNull();
  });

  it("going ahead answers yes", async () => {
    render(<CancelQueuedConfirm />);
    let answer: Promise<boolean> = Promise.resolve(false);
    act(() => {
      answer = confirmCancellations([queued, { ...queued, id: "q2" }]);
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel them and go ahead" }));
    await expect(answer).resolves.toBe(true);
  });
});
