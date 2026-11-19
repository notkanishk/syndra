// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { PeopleOnTarget } from "@/components/targets/PeopleOnTarget";
import type { Standing } from "@/lib/queries/useTargets";

const people = [
  ["u1", "charlotte.l", "Charlotte Flair"],
  ["u2", "sami.h", "Sami Zayn"],
] as const;

function account(i: number) {
  const [id, username] = people[i % 2];
  return {
    subject_id: `${id}-${i}`,
    username: i < 2 ? username : `${username}${i}`,
    account_id: String(200000 + i),
    bound_at: "2026-10-15T10:00:00Z",
    // Only the first has a standing: the others still keep the Status column's place.
    standing: i === 0 ? state.standing : undefined,
  };
}

const state: { count: number; standing: Standing } = { count: 2, standing: { usable: true, waiting_on: "none" } };

// Who is a service account comes from one cached list; these rows are all people.
vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set<string>(),
}));

// A read is one object until it is read again, as the query cache's is; a list
// that is a new array of new rows on every render would be a read on every render.
const read: { count: number; data?: { bound: number; accounts: ReturnType<typeof account>[] } } = { count: -1 };
function inventory() {
  if (read.count !== state.count) {
    read.count = state.count;
    read.data = { bound: state.count, accounts: Array.from({ length: state.count }, (_, i) => account(i)) };
  }
  return read.data;
}

vi.mock("@/lib/queries/useTargets", () => ({
  useTargetInventory: () => ({
    data: inventory(),
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useTargetHomes: () => ({ data: { accounts: [] }, isLoading: false, error: null }),
}));
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{id.startsWith("u1") ? "Charlotte Flair" : "Sami Zayn"}</span>,
  UserAvatar: ({ id }: { id: string }) => <span aria-hidden>{id.startsWith("u1") ? "AL" : "GH"}</span>,
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({
      value: { display_name: id.startsWith("u1") ? "Charlotte Flair" : "Sami Zayn" },
      resolved: true,
    }),
  }),
}));
vi.mock("@/components/kinds/storage/HomeLimit", () => ({
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

    const row = screen.getByText("Charlotte Flair").closest("[data-account]") as HTMLElement;
    expect(within(row).getByText("AL")).toBeInTheDocument();
    expect(within(row).getByText("charlotte.l")).toHaveClass("font-mono");
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
    expect(screen.getByRole("dialog")).toHaveTextContent("Hold for Charlotte Flair");
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

// How the fields stand: known (the TrueNAS fixture), being read, failed, or read with no schema.
const fields = vi.hoisted(() => ({ mode: "known" as "known" | "loading" | "failed" | "unread", noLimit: false }));
vi.mock("@/lib/queries/useFieldSchema", async () => {
  const m = (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock;
  return {
    ...m,
    // A system whose setup has no home limit field.
    useMappingFields: () => (fields.noLimit ? {} : m.useMappingFields()),
    useFieldSchemaState: () => {
      if (fields.mode === "known") return m.useFieldSchemaState();
      return { schema: undefined, loading: fields.mode === "loading", error: fields.mode === "failed" ? new Error("down") : null, refetch: () => {} };
    },
  };
});

// D-1 §5 #18, #39: the seat is the system's, never the rows'; Status is a column of its own.
describe("accounts table seats", () => {
  afterEach(() => {
    fields.noLimit = false;
  });

  it("puts every verb in the action seat, one width for every row and its heading, with or without a limit", () => {
    state.count = 2;
    for (const [noLimit, seat] of [[false, "wide"], [true, null]] as const) {
      fields.noLimit = noLimit;
      const { container, unmount } = render(<PeopleOnTarget target="truenas" />);
      const seats = [...container.querySelectorAll("[data-card-row] [data-action-seat]")];
      expect(seats).toHaveLength(2);
      for (const s of seats) {
        expect(s.getAttribute("data-seat")).toBe(seat);
        expect(within(s as HTMLElement).getByRole("button", { name: "Revoke" })).toBeInTheDocument();
        expect(within(s as HTMLElement).getByRole("button", { name: "Put on hold" })).toBeInTheDocument();
      }
      // The heading cell over the seat is the same width.
      expect(container.querySelector(".row-cols .row-seat")?.getAttribute("data-seat")).toBe(seat);
      unmount();
    }
  });

  it("holds the wide seat until the system's setup is read, so the seat never moves when it lands", () => {
    state.count = 2;
    fields.noLimit = true;
    for (const mode of ["loading", "failed"] as const) {
      fields.mode = mode;
      const { container, unmount } = render(<PeopleOnTarget target="truenas" />);
      const seats = [...container.querySelectorAll("[data-card-row] [data-action-seat]")];
      expect(seats).toHaveLength(2);
      for (const s of seats) expect(s).toHaveAttribute("data-seat", "wide");
      unmount();
    }
    fields.mode = "known";
  });

  it("has a Status column: heading, and the standing in its own cell, not under the username", () => {
    state.count = 2;
    render(<PeopleOnTarget target="truenas" />);

    const heading = screen.getByText("Status").closest("[data-col]") as HTMLElement;
    const row = screen.getByText("Charlotte Flair").closest("[data-account]") as HTMLElement;
    const standing = row.querySelector("[data-standing]") as HTMLElement;
    expect(standing).toHaveTextContent("Ready to use");
    // A healthy row with nothing to do is silent (D-1 Q5): no step, in the status column or under the row.
    expect(standing).not.toHaveTextContent("Nothing for you to do.");
    expect(row).not.toHaveTextContent("Nothing for you to do.");
    expect(row.querySelector("[data-row-detail]")).toBeNull();
    expect(standing.parentElement).not.toBe(within(row).getByText("charlotte.l"));
    // Heading and cells share one fixed seat; a row with no standing keeps the cell.
    const cell = standing.parentElement as HTMLElement;
    expect(cell.className).toBe(heading.className);
    const other = screen.getByText("Sami Zayn").closest("[data-account]") as HTMLElement;
    expect(other.querySelector("[data-standing]")).toBeNull();
    expect([...other.children].some((c) => c.className === heading.className)).toBe(true);
  });

  // Silence is for a healthy row only (D-1 Q5): a row that is not healthy still writes its step, and says
  // "Nothing for you to do." when that is the step, on the row's detail line.
  it.each([
    [{ usable: false, waiting_on: "member" }, "Ask them to do what it says above."],
    [{ usable: false }, "Nothing for you to do."],
    [{ usable: true, waiting_on: "member" }, "Nothing for you to do."],
    [{ usable: true, waiting_on: "none", next: "Sign in once to finish." }, "Sign in once to finish."],
  ] as Array<[Standing, string]>)("writes the step where there is one (%#)", (standing, step) => {
    state.standing = standing;
    read.count = -1;
    try {
      render(<PeopleOnTarget target="truenas" />);
      const row = screen.getByText("Charlotte Flair").closest("[data-account]") as HTMLElement;
      expect(row.querySelector("[data-row-detail] [data-standing-step]")).toHaveTextContent(step);
    } finally {
      state.standing = { usable: true, waiting_on: "none" };
      read.count = -1;
    }
  });
});

describe("Put on hold before the fields are known", () => {
  afterEach(() => {
    fields.mode = "known";
  });

  it("is off in all three states, each saying a different true thing", () => {
    state.count = 1;
    const said: string[] = [];
    for (const mode of ["loading", "failed", "unread"] as const) {
      fields.mode = mode;
      const { unmount } = render(<PeopleOnTarget target="truenas" />);
      expect((screen.getByRole("button", { name: "Put on hold" }) as HTMLButtonElement).disabled).toBe(true);
      said.push((screen.getByText(/TrueNAS's fields/).textContent ?? ""));
      unmount();
    }
    expect(said[0]).toMatch(/still reading/);
    expect(said[1]).toMatch(/could not read/);
    expect(said[2]).toMatch(/has not read/);
  });
});
