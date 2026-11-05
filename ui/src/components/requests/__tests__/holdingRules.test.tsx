// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApproveButton, HoldingNote, keepLength } from "@/components/requests/Holding";
import type { AccessRequest } from "@/lib/queries/useRequests";

const preview = vi.hoisted(() => ({
  rows: [] as string[],
  asked: [] as Array<{ enabled: boolean; keys: string[] }>,
  loading: false,
  error: false,
  refetching: false,
  refetch: vi.fn(),
}));

vi.mock("@/lib/queries/useRulePreview", () => ({
  useRulePreview: (_user: string, _project: string, keys: string[], enabled: boolean) => {
    preview.asked.push({ enabled, keys });
    return {
      data: enabled ? preview.rows.map((sentence) => ({ sentence })) : undefined,
      isLoading: enabled && preview.loading,
      isError: enabled && preview.error,
      isFetching: enabled && (preview.loading || preview.refetching),
      refetch: preview.refetch,
    };
  },
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: undefined, resolved: true }) }),
}));

function ask(overrides: Partial<AccessRequest> = {}): AccessRequest {
  return {
    id: "r1",
    requester_id: "u1",
    project_id: "p1",
    role_key: "operator",
    justification: "",
    status: "pending",
    created_at: "2026-11-01T00:00:00Z",
    holding: { kind: "none" },
    ...overrides,
  } as AccessRequest;
}

beforeEach(() => {
  preview.rows = [];
  preview.asked = [];
  preview.loading = false;
  preview.error = false;
  preview.refetching = false;
  preview.refetch = vi.fn();
});

// Audit C2: approving gives the role, and the automatic rules follow it, so
// the row the operator approves from names what they also give.
describe("approving, with automatic rules", () => {
  it("names what the rules also give before the approve", () => {
    preview.rows = ["Also gives Bench in Workshop, by the automatic rule from Operator."];
    render(<HoldingNote request={ask()} />);
    expect(
      screen.getByText("If you approve: Also gives Bench in Workshop, by the automatic rule from Operator."),
    ).toBeInTheDocument();
    expect(preview.asked.at(-1)).toEqual({ enabled: true, keys: ["operator"] });
  });

  it("says nothing when no rule follows, and does not ask about a role already held", () => {
    const { container } = render(<HoldingNote request={ask()} />);
    expect(container.textContent).toBe("");
    render(<HoldingNote request={ask({ holding: { kind: "for_good" } })} />);
    expect(preview.asked.at(-1)?.enabled).toBe(false);
  });
});

// Final verification N5: what the rules also give is part of the consent, so
// Approve waits for the preview and is held, with why, when it can't be read.
describe("approving, while the rule preview is not read", () => {
  it("holds Approve while the preview loads", () => {
    preview.loading = true;
    render(<ApproveButton request={ask()} onClick={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByText("Checking what automatic rules also give.")).toBeInTheDocument();
  });

  it("holds Approve when the preview fails, and says why, beside the row's own line", () => {
    preview.error = true;
    const click = vi.fn();
    render(
      <>
        <HoldingNote request={ask()} />
        <ApproveButton request={ask()} onClick={click} />
      </>,
    );
    const button = screen.getByRole("button", { name: "Approve" });
    expect(button).toBeDisabled();
    button.click();
    expect(click).not.toHaveBeenCalled();
    // The row says why and offers Try again; the button says it is held.
    expect(screen.getByText(/Syndra couldn.t read the automatic rules/)).toBeInTheDocument();
    expect(screen.getByText("Held until Syndra can read the automatic rules.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(preview.refetch).toHaveBeenCalledTimes(1);
  });

  it("is checking, not failed, while Try again reads the preview", () => {
    preview.error = true;
    preview.refetching = true;
    render(
      <>
        <HoldingNote request={ask()} />
        <ApproveButton request={ask()} onClick={vi.fn()} />
      </>,
    );
    expect(screen.queryByRole("button", { name: "Try again" })).toBeNull();
    expect(screen.getByRole("button", { name: "Approve" })).toBeDisabled();
    expect(screen.getByText("Checking what automatic rules also give.")).toBeInTheDocument();
  });

  it("lets Approve go once the preview is read", () => {
    const click = vi.fn();
    render(<ApproveButton request={ask()} onClick={click} />);
    screen.getByRole("button", { name: "Approve" }).click();
    expect(click).toHaveBeenCalledTimes(1);
  });

  it("still holds a role they already have, without asking the rules", () => {
    preview.error = true;
    render(<ApproveButton request={ask({ holding: { kind: "for_good" } })} onClick={vi.fn()} />);
    expect(screen.getByText("They already have this.")).toBeInTheDocument();
  });
});

// Audit S6: a term's last day is matched in the makerspace's day, not the
// browser's: an end at 00:00 IST on 27 Nov is 26 Nov in UTC.
describe("keepLength", () => {
  it("matches the term's last day in the makerspace's zone", () => {
    const zone = process.env.TZ;
    process.env.TZ = "UTC";
    try {
      const terms = [{ name: "Monsoon", start: "2026-08-03", end: "2026-11-27" }];
      expect(keepLength(ask({ kind: "extend", duration_days: 40, until: "2026-11-26T18:30:00Z" }), terms)).toBe(
        "To the end of next term",
      );
    } finally {
      process.env.TZ = zone;
    }
  });
});
