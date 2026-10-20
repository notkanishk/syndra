// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";

const people = [
  ["u1", "ada.l", "Ada Lovelace"],
  ["u2", "grace.h", "Grace Hopper"],
] as const;

function account(i: number) {
  const [id, username] = people[i % 2];
  return { subject_id: `${id}-${i}`, username: i < 2 ? username : `${username}${i}`, account_uid: 200000 + i, bound_at: "2026-10-15T10:00:00Z" };
}

const state = { count: 2 };

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: { bound: state.count, accounts: Array.from({ length: state.count }, (_, i) => account(i)) },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id.startsWith("u1") ? "Ada Lovelace" : "Grace Hopper"}</span>,
  UserAvatar: ({ id }: { id: string }) => <span aria-hidden>{id.startsWith("u1") ? "AL" : "GH"}</span>,
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({
      value: { display_name: id.startsWith("u1") ? "Ada Lovelace" : "Grace Hopper" },
      resolved: true,
    }),
  }),
}));
vi.mock("@/components/targets/HomeLimit", () => ({
  HomeLine: () => null,
  LimitDialog: () => null,
  limitVerb: () => null,
}));
vi.mock("@/components/review/HoldDialog", () => ({
  HoldDialog: ({ subjectName }: { subjectName: string }) => <div role="dialog">Hold for {subjectName}</div>,
}));

describe("accounts Syndra manages, as the board's table", () => {
  it("heads the columns: person, account on the system, since", () => {
    state.count = 2;
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText("Person")).toBeInTheDocument();
    expect(screen.getByText("Account on TrueNAS")).toBeInTheDocument();
    expect(screen.getByText("Since")).toBeInTheDocument();
  });

  it("names each person with their initials beside the name, and the account name in mono", () => {
    state.count = 2;
    render(<PeopleOnTarget target="truenas" />);

    const row = screen.getByText("Ada Lovelace").closest("[data-account]") as HTMLElement;
    expect(within(row).getByText("AL")).toBeInTheDocument();
    expect(within(row).getByText("ada.l")).toHaveClass("font-mono");
    expect(screen.getByText("GH")).toBeInTheDocument();
    expect(screen.queryByText(/200001|200000/)).not.toBeInTheDocument();
  });

  it("dates the account the way a date is spoken, not as 'N days ago'", () => {
    state.count = 1;
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByText("15 Oct")).toBeInTheDocument();
    expect(screen.queryByText(/ago/)).toBeNull();
  });

  it("keeps the row's verbs: put on hold opens the hold, revoke is a button", () => {
    state.count = 1;
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getByRole("button", { name: "Revoke" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Put on hold" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("Hold for Ada Lovelace");
  });

  it("shows ten, then more on request", () => {
    state.count = 12;
    render(<PeopleOnTarget target="truenas" />);

    expect(screen.getAllByRole("button", { name: "Revoke" })).toHaveLength(10);
    expect(screen.getByText(/Showing 10 of 12/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Show 2 more/ }));
    expect(screen.getAllByRole("button", { name: "Revoke" })).toHaveLength(12);
  });
});
