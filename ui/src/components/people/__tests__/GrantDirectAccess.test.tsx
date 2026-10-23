// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GrantDirectAccess } from "@/components/people/GrantDirectAccess";
import { ApiError } from "@/lib/api-client";

/**
 * The direct-grant endpoint always enqueues to the outbox for the
 * operator-triggered drain — it never applies inline from this dialog. A
 * result reported as "given" here would tell the operator Zitadel has the
 * grant when only Syndra's own ledger does.
 */

const state = vi.hoisted(() => ({
  down: false,
  mutateAsync: vi.fn(),
  grants: [] as unknown[],
  queued: [] as string[],
  queuedAt: undefined as string | undefined,
  terms: undefined as Array<{ name: string; start: string; end: string }> | undefined,
}));

vi.mock("@/components/states/DegradedBanner", () => ({ useDegraded: () => state.down }));

vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({ data: state.terms }),
}));

vi.mock("@/lib/queries/useProjects", () => ({
  useProjects: () => ({
    data: [{ project: { id: "p1", name: "Printing Lab" } }, { project: { id: "p2", name: "Empty Lab" } }],
  }),
}));

vi.mock("@/lib/queries/useRoles", () => ({
  useGlobalRoleCatalog: () => ({
    data: [
      {
        project_id: "p1",
        role_key: "operator",
        project_name: "Printing Lab",
        display_name: "Operator",
        description: "Runs the printers",
        assigned_user_count: 3,
        observed_user_count: 4,
      },
      {
        project_id: "p1",
        role_key: "supervisor",
        project_name: "Printing Lab",
        display_name: "Supervisor",
        assigned_user_count: 1,
        observed_user_count: 1,
        waits_for_send: true,
      },
    ],
  }),
}));

vi.mock("@/lib/queries/useUsers", () => ({
  useCreateGrant: () => ({ mutateAsync: state.mutateAsync, isPending: false }),
  useUserGrants: () => ({ data: state.grants }),
  useUserAccess: () => ({
    data: {
      projects: [
        {
          project_id: "p1",
          observed_role_keys: [],
          source_roles: state.queued.map((key) => ({ role_key: key, reasons: [{ kind: "direct", description: "", queued: true, queued_at: state.queuedAt }] })),
          derived_roles: [],
        },
      ],
    },
  }),
}));

beforeEach(() => {
  state.down = false;
  state.grants = [];
  state.queued = [];
  state.queuedAt = undefined;
  state.terms = undefined;
  state.mutateAsync.mockReset();
});

function open() {
  return render(<GrantDirectAccess userId="u1" userName="Ada Lovelace" open onClose={vi.fn()} />);
}

function choose(role: string) {
  const dialog = screen.getByRole("dialog");
  if (!within(dialog).queryByRole("radio", { name: new RegExp(role) })) {
    fireEvent.click(within(dialog).getByRole("button", { name: /Printing Lab/ }));
  }
  fireEvent.click(within(dialog).getByRole("radio", { name: new RegExp(role) }));
}

async function grant(role = "Operator") {
  choose(role);
  fireEvent.change(screen.getByPlaceholderText(/Capstone build/), { target: { value: "test" } });
  fireEvent.click(screen.getByRole("button", { name: "Grant access" }));
  // "Close" only replaces "Grant access"/"Cancel" once an outcome (either kind)
  // is set — a wait condition that holds regardless of which message the
  // outcome carries.
  await screen.findByRole("button", { name: "Close" });
}

describe("GrantDirectAccess · choosing the role", () => {
  it("is the role picker in single choice, with the operator's notes", () => {
    open();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    const row = screen.getByRole("radio", { name: /Operator/ }).closest("label")!;
    expect(within(row).getByText("operator")).toBeInTheDocument();
    expect(within(row).getByText("4 people")).toBeInTheDocument();
    expect(within(row).getByText("Runs the printers")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Waits for you" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Empty Lab/ })).toHaveTextContent("Nothing yet");
  });

  it("says a role already held with no end changes nothing, and will not send it", () => {
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: null }];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Has this")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Grant access" })).toBeDisabled();
  });

  it("a role given and not yet sent says so, with the way to send it", () => {
    state.queued = ["operator"];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
  });

  it("says when it was given, from the time the read carries, beside the way to send it", () => {
    state.queued = ["operator"];
    state.queuedAt = new Date(Date.now() - 2 * 3_600_000).toISOString();
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Given 2h ago")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeInTheDocument();
  });

  it("a role given with an end but not yet sent is on its way, not held until a date", () => {
    // Found on the dev box: an approved request's grant has an end and waits in the
    // outbox; the row said "Has this until 31 Dec · Extend" for access not yet in Zitadel.
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: "2026-12-18T12:00:00Z" }];
    state.queued = ["operator"];
    state.queuedAt = new Date(Date.now() - 60_000).toISOString();
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText(/^Given/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toBeInTheDocument();
    expect(screen.queryByText(/Has this until/)).toBeNull();
    expect(screen.queryByRole("link", { name: /Extend/ })).toBeNull();
  });

  it("a role held with an end is listed, not pickable, and offers Extend", () => {
    state.grants = [{ id: "g1", project_id: "p1", role_key: "operator", expires_at: "2026-12-18T12:00:00Z" }];
    open();
    fireEvent.click(screen.getByRole("button", { name: /Printing Lab/ }));
    expect(screen.getByText("Has this until 18 Dec")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Extend/ })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Operator/ })).toBeDisabled();
  });

  it("says before the give that a waiting role waits, and a plain one doesn't", () => {
    open();
    choose("Operator");
    expect(screen.getByText("Ada can use it as soon as you give it.")).toBeInTheDocument();
    choose("Supervisor");
    expect(
      screen.getByText("Supervisor always waits for you, so Ada gets it once you send it."),
    ).toBeInTheDocument();
  });

  it("names the project and role, never the bare id", () => {
    open();
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.queryByText("p1")).not.toBeInTheDocument();
  });
});

describe("GrantDirectAccess · End of term", () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ["Date"], now: new Date(2026, 8, 1, 12) }));
  afterEach(() => vi.useRealTimers());

  it("is the end of the term holding today, from the term setting", () => {
    state.terms = [{ name: "Monsoon", start: "2026-08-03", end: "2026-11-27" }];
    open();
    fireEvent.click(screen.getByRole("button", { name: "End of term" }));
    expect(screen.getByText("27 Nov")).toBeInTheDocument();
  });

  it("falls back to 18 December when the setting can't be read", () => {
    open();
    fireEvent.click(screen.getByRole("button", { name: "End of term" }));
    expect(screen.getByText("18 Dec")).toBeInTheDocument();
  });
});

describe("GrantDirectAccess · its result", () => {
  it("a role that always waits is Given, amber, with why and what that means for them", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "pending", wait_reason: "role_waits" });
    open();
    await grant("Supervisor");

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending");
    expect(screen.getByText("Supervisor always waits for you, so Ada can't use it yet.")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
  });

  it("a send that stopped is queued with the stop named, and goes when sent", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "pending", wait_reason: "halted:zitadel_offline" });
    open();
    await grant();

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(
      screen.getByText(/^Zitadel didn't answer at \d\d:\d\d, so it goes when you send it\.$/),
    ).toBeInTheDocument();
  });

  it("reports Given, in their first name, when nothing waits", async () => {
    state.mutateAsync.mockResolvedValue({ outbox_id: "o1", status: "applied" });
    open();
    await grant();

    expect(screen.getByText("Given")).toBeInTheDocument();
    expect(screen.getByText("Ada can use Operator on their own now.")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Grant access" })).not.toBeInTheDocument();
  });

  it("a refusal is Not given, and keeps the form", async () => {
    state.mutateAsync.mockRejectedValue(new ApiError(409, { error: "ALREADY_HELD", message: "They already have this." }));
    open();
    choose("Operator");
    fireEvent.click(screen.getByRole("button", { name: "Grant access" }));
    expect(await screen.findByText("Not given")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Grant access" })).toBeEnabled();
  });
});

// A-07 §9: a person's page stays on while Zitadel is down, and says before the
// operator acts that the change will wait.
describe("GrantDirectAccess · while Zitadel is not answering", () => {
  it("says before the grant that it will wait, instead of promising it works at once", () => {
    state.down = true;
    open();
    choose("Operator");
    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText("Zitadel isn't answering, so this will wait for you to send it once it does."),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(/as soon as you give it/)).not.toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Grant access" })).toBeEnabled();
  });
});
