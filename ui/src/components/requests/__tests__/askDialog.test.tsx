// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";
import { ApiError } from "@/lib/api-client";

// Ask for access, from where it is asked: the places come from /catalog, the
// one list a member may read, and an "Ask for this" link opens the dialog
// already filled in, for a member and for an operator asking for themselves.

const search = vi.hoisted(() => ({ value: "" }));
const mine = vi.hoisted(() => ({ data: [] as Array<Record<string, unknown>> }));
const held = vi.hoisted(() => ({
  grants: [] as Array<{ id: string; project_id: string; role_key: string; expires_at?: string | null }>,
  observed: [] as string[],
  queued: [] as string[],
}));
const create = vi.hoisted(() => ({
  calls: 0,
  last: null as Record<string, unknown> | null,
  fail: null as unknown,
}));
const termsData = vi.hoisted(() => ({
  terms: [
    { name: "Spring", start: "2026-01-01", end: "2026-05-31" },
    { name: "Monsoon", start: "2026-08-01", end: "2026-12-31" },
    { name: "Spring", start: "2027-01-01", end: "2027-05-31" },
  ] as Array<{ name: string; start: string; end: string }> | undefined,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(search.value),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsMine: () => ({ data: mine.data, isLoading: false, error: null, refetch: () => {} }),
  useRequestsAdmin: () => ({ data: mine.data, isLoading: false, error: null, refetch: () => {} }),
  useCreateRequest: () => ({
    mutateAsync: async (input: Record<string, unknown>) => {
      create.calls++;
      create.last = input;
      if (create.fail) throw create.fail;
      return { id: "r-new" };
    },
    isPending: false,
  }),
  useWithdrawRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useTerms", () => ({
  useTerms: () => ({ data: termsData.terms, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useMyChanges", () => ({
  useMyChanges: () => ({ data: [] }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({
    data: [
      {
        id: "pLaser",
        name: "Laser Lab",
        kind: "",
        description: "",
        roles: [
          { key: "trained", label: "Trained operator", description: "Runs the cutter" },
          { key: "viewer", label: "Viewer", description: "" },
        ],
      },
      { id: "pWood", name: "Woodshop", kind: "", description: "", roles: [{ key: "bandsaw", label: "Bandsaw", description: "" }] },
      { id: "pNet", name: "Network storage", kind: "", description: "", roles: [] },
    ],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserGrants: () => ({ data: held.grants }),
  useUserAccess: () => ({
    data: {
      projects: [
        {
          project_id: "pLaser",
          observed_role_keys: held.observed,
          source_roles: held.queued.map((key) => ({ role_key: key, reasons: [{ kind: "direct", description: "", queued: true }] })),
          derived_roles: [],
        },
      ],
    },
  }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

function renderAs(isOperator: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator={isOperator} userId="u1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  search.value = "";
  mine.data = [];
  held.grants = [];
  held.observed = [];
  held.queued = [];
  create.calls = 0;
  create.last = null;
  create.fail = null;
  termsData.terms = [
    { name: "Spring", start: "2026-01-01", end: "2026-05-31" },
    { name: "Monsoon", start: "2026-08-01", end: "2026-12-31" },
    { name: "Spring", start: "2027-01-01", end: "2027-05-31" },
  ];
});

const dialog = () => screen.getByRole("dialog");
const sendButton = () => screen.getByRole("button", { name: "Send request" });

describe("Ask for access", () => {
  it("offers every place in the catalogue as a fold, the one a link named open first", () => {
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(within(dialog()).getByRole("button", { name: /Laser Lab/ })).toHaveAttribute("aria-expanded", "true");
    expect(within(dialog()).getByRole("button", { name: /Woodshop/ })).toHaveAttribute("aria-expanded", "false");
    expect(within(dialog()).getByRole("button", { name: /Network storage/ })).toHaveTextContent("Nothing yet");
    expect(within(dialog()).getByRole("radio", { name: /Trained operator/ })).toBeChecked();
    expect(within(dialog()).getByRole("button", { name: "Remove Trained operator" })).toBeInTheDocument();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("opens over the queue for an operator asking for themselves, and says they decide", () => {
    search.value = "project=pLaser&role=trained";
    renderAs(true);
    expect(screen.getByRole("heading", { name: "Ask for access" })).toBeInTheDocument();
    expect(within(dialog()).getByRole("radio", { name: /Trained operator/ })).toBeChecked();
    expect(
      screen.getByText("You'll decide this yourself, unless another operator does first."),
    ).toBeInTheDocument();
  });

  it("opens empty from a plain Ask for access link, with Send off and what to do", () => {
    search.value = "ask";
    renderAs(false);
    expect(within(dialog()).queryByRole("button", { name: /^Remove/ })).not.toBeInTheDocument();
    expect(screen.getAllByText("Choose a role to ask for.").length).toBeGreaterThan(0);
    expect(sendButton()).toBeDisabled();
  });

  it("stays shut on a plain visit", () => {
    renderAs(true);
    expect(screen.queryByRole("heading", { name: "Ask for access" })).not.toBeInTheDocument();
  });

  it("a place with nothing in it says so, and nothing can be sent", () => {
    search.value = "project=pNet";
    renderAs(false);
    expect(
      screen.getByText("Nothing is set up here yet, so there's nothing to ask for. Makerspace staff can set it up."),
    ).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
  });
});

// Found on production, 15 Oct: test_user asked five times, from one dialog,
// for a role they already held. A request is for a role someone lacks, one per
// role (owner, 15 Oct).
describe("Ask for access · only what they lack, once", () => {
  it("arriving on a role they have: outlined, a sentence in place of the hint, nothing to fill in", () => {
    held.observed = ["trained"];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(
      screen.getByText("You already have Laser Lab · Trained operator, with no end date, so there's nothing to ask for."),
    ).toBeInTheDocument();
    expect(within(dialog()).getByText("You have this")).toBeInTheDocument();
    expect(within(dialog()).getByRole("radio", { name: /Trained operator/ })).toBeDisabled();
    expect(screen.queryByText("How long do you need it?")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Why?")).not.toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
    expect(screen.getByText("Choose another role to ask for.")).toBeInTheDocument();
  });

  it("choosing another role brings the form back", () => {
    held.observed = ["trained"];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    fireEvent.click(within(dialog()).getByRole("radio", { name: /Viewer/ }));
    expect(screen.getByText("How long do you need it?")).toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
    expect(screen.queryByText(/with no end date, so there's nothing to ask for/)).not.toBeInTheDocument();
  });

  it("arriving on one they've already asked for says it's waiting", () => {
    mine.data = [{ id: "r1", requester_id: "u1", project_id: "pLaser", role_key: "trained", status: "pending", justification: "", created_at: "2026-10-15T12:00:00Z" }];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(
      screen.getByText("You've already asked for Laser Lab · Trained operator. It's waiting, and the answer appears on this page."),
    ).toBeInTheDocument();
    expect(within(dialog()).getByText(/^Asked /)).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
  });

  it("arriving on one given and not yet sent says it's on its way", () => {
    held.queued = ["trained"];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(
      screen.getByText(
        "Makerspace staff gave you Trained operator. It starts working once they send it, so there's nothing to ask for.",
      ),
    ).toBeInTheDocument();
    expect(within(dialog()).getByText("On its way")).toBeInTheDocument();
    expect(sendButton()).toBeDisabled();
  });

  it("becomes its result once sent: Asked, who decides, and only Close left", async () => {
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    fireEvent.click(screen.getByRole("button", { name: "A month" }));
    fireEvent.click(sendButton());
    expect(await screen.findByText("Asked")).toBeInTheDocument();
    expect(screen.getByText("Trained operator on Laser Lab, for a month.")).toBeInTheDocument();
    expect(
      screen.getByText("Makerspace staff decide. The answer appears on this page, and nothing else is needed from you."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send request" })).not.toBeInTheDocument();
    expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(create.calls).toBe(1);
    expect(create.last).toMatchObject({ kind: "access", project_id: "pLaser", role_key: "trained", duration_days: 30 });
  });

  it("a race with another tab is Not sent, neutral, and says the request is waiting", async () => {
    create.fail = new ApiError(409, { error: "ALREADY_ASKED", message: "You've already asked." });
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    fireEvent.click(sendButton());
    expect(await screen.findByText("Not sent")).toBeInTheDocument();
    expect(
      screen.getByText("You already asked for Trained operator on Laser Lab, from another tab. That request is waiting."),
    ).toBeInTheDocument();
    expect(screen.getByText("Nothing was changed. Its answer appears on this page.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("another refusal keeps the form, so it can be tried again", async () => {
    create.fail = new ApiError(409, { error: "ALREADY_HELD", message: "You already have this." });
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    fireEvent.click(sendButton());
    expect(await screen.findByText("Not sent")).toBeInTheDocument();
    expect(sendButton()).toBeEnabled();
  });
});

describe("Ask for access · asking to keep a role that ends", () => {
  const ends = () => {
    held.grants = [{ id: "g1", project_id: "pLaser", role_key: "trained", expires_at: new Date(2026, 11, 18, 12).toISOString() }];
    search.value = "project=pLaser&role=trained&extend=1";
  };

  it("changes the form: the sentence, How much longer, the hint and the footer", () => {
    ends();
    renderAs(false);
    expect(screen.getByRole("heading", { name: "Ask for access" })).toBeInTheDocument();
    expect(
      screen.getByText("You have this until 18 Dec. Ask to keep it longer, and makerspace staff decide."),
    ).toBeInTheDocument();
    expect(screen.getByText("How much longer?", { selector: "label" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Another month" })).toBeChecked();
    expect(screen.getByText(/^If it[’']s approved, you keep it to 17 Jan 2027 instead of 18 Dec\.$/)).toBeInTheDocument();
    expect(screen.getByText("Makerspace staff decide. Until they do, it still ends 18 Dec.")).toBeInTheDocument();
    expect(screen.queryByText("How long do you need it?")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Send request" })).not.toBeInTheDocument();
  });

  it("To the end of next term counts the days from the current end, and moves the hint", () => {
    ends();
    renderAs(false);
    fireEvent.click(screen.getByRole("radio", { name: "To the end of next term" }));
    expect(screen.getByText(/^If it[’']s approved, you keep it to 31 May 2027 instead of 18 Dec\.$/)).toBeInTheDocument();
  });

  it("To the end of next term is off, with its reason, when there is no next term", () => {
    ends();
    termsData.terms = [{ name: "Monsoon", start: "2026-08-01", end: "2026-12-31" }];
    renderAs(false);
    const option = screen.getByRole("radio", { name: "To the end of next term" });
    expect(option).toBeDisabled();
    expect(option).toHaveAttribute("title", "There's no next term set yet.");
  });

  it("sends an extension for the days chosen, and says what it asked for", async () => {
    ends();
    renderAs(false);
    fireEvent.click(screen.getByRole("button", { name: "Ask to keep it" }));
    expect(await screen.findByText("Asked")).toBeInTheDocument();
    expect(screen.getByText("To keep Laser Lab · Trained operator until 17 Jan 2027.")).toBeInTheDocument();
    expect(
      screen.getByText("Until makerspace staff decide, it still ends 18 Dec. The answer appears on Requests."),
    ).toBeInTheDocument();
    expect(create.last).toMatchObject({ kind: "extend", project_id: "pLaser", role_key: "trained", duration_days: 30 });
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("a second ask for the same keep is refused as already waiting", async () => {
    ends();
    create.fail = new ApiError(409, { error: "ALREADY_ASKED", message: "x" });
    renderAs(false);
    fireEvent.click(screen.getByRole("button", { name: "Ask to keep it" }));
    expect(await screen.findByText("Not sent")).toBeInTheDocument();
    expect(
      screen.getByText("You've already asked to keep this. It's waiting, and the answer appears on this page."),
    ).toBeInTheDocument();
  });

  it("a request already waiting turns the arrival into the sentence, with Send off", () => {
    ends();
    mine.data = [{ id: "r1", requester_id: "u1", project_id: "pLaser", role_key: "trained", status: "pending", justification: "", created_at: "2026-10-15T12:00:00Z", kind: "extend" }];
    renderAs(false);
    expect(screen.getByText(/You've already asked for Laser Lab · Trained operator/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ask to keep it" })).not.toBeInTheDocument();
  });

  it("stays an ordinary ask for a role they lack, even from ?extend=1", async () => {
    search.value = "project=pLaser&role=trained&extend=1";
    renderAs(false);
    fireEvent.click(sendButton());
    await screen.findByText("Asked");
    expect(create.last).toMatchObject({ kind: "access" });
  });
});

describe("The queue · a request for a role they have since come to hold", () => {
  it("says so, and can't be approved", () => {
    mine.data = [
      {
        id: "r1", requester_id: "u9", project_id: "pLaser", role_key: "trained", status: "pending",
        justification: "again", created_at: "2026-10-15T12:00:00Z", holding: { kind: "for_good" },
      },
    ];
    renderAs(true);
    expect(screen.getByText(/Already has this, with no end/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Approve/ })).toBeDisabled();
  });
});

describe("The queue · how long an ask is for", () => {
  it("says the length and the date approving now would end it", () => {
    mine.data = [
      {
        id: "r1", requester_id: "u9", project_id: "pLaser", role_key: "trained", status: "pending",
        justification: "capstone", created_at: "2026-10-15T12:00:00Z", duration_days: 30,
      },
    ];
    renderAs(true);
    expect(screen.getByText(/^For a month · until \d{1,2} [A-Z][a-z]{2}( \d{4})?$/)).toBeInTheDocument();
    expect(screen.queryByText(/from when it's approved/)).not.toBeInTheDocument();
  });
});

describe("The queue · a request to keep a role longer", () => {
  const extendRow = (holding: Record<string, unknown>) => [
    {
      id: "r2", requester_id: "u9", project_id: "pLaser", role_key: "trained", status: "pending",
      justification: "capstone", created_at: "2026-10-15T12:00:00Z", kind: "extend", duration_days: 30, holding,
    },
  ];

  it("reads as an extension, and can be approved while they hold it until a date", () => {
    mine.data = extendRow({ kind: "until", until: "2026-11-03T12:00:00Z" });
    renderAs(true);
    expect(screen.getByText("Asks to keep it longer · for 30 more days")).toBeInTheDocument();
    expect(screen.getByText("Has it until 3 Nov.")).toBeInTheDocument();
    expect(screen.queryByText(/Already has this/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Approve/ })).toBeEnabled();
  });

  it("can't be approved once they hold it with no end", () => {
    mine.data = extendRow({ kind: "for_good" });
    renderAs(true);
    expect(screen.getByText(/Already has this, with no end/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Approve/ })).toBeDisabled();
  });

  it("says it ended while it waited", () => {
    mine.data = extendRow({ kind: "none" });
    renderAs(true);
    expect(
      screen.getByText("It ended while this waited. Approving gives it back for the time asked."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Approve/ })).toBeEnabled();
  });
});

// Found on production, 15 Oct: Close, then Ask for access again, reopened on
// the last result until a reload. Each opening starts clean.
describe("Ask for access · each opening starts clean", () => {
  it("opens on the form again after Close", async () => {
    search.value = "ask";
    renderAs(false);
    fireEvent.click(within(dialog()).getByRole("button", { name: /Laser Lab/ }));
    fireEvent.click(within(dialog()).getByRole("radio", { name: /Trained operator/ }));
    fireEvent.click(sendButton());
    fireEvent.click(await screen.findByRole("button", { name: "Close" }));
    fireEvent.click(screen.getAllByRole("button", { name: "Ask for access" })[0]);
    expect(screen.getByRole("searchbox", { name: "Search roles" })).toBeInTheDocument();
    expect(screen.queryByText("Asked")).not.toBeInTheDocument();
    expect(within(dialog()).queryByRole("button", { name: /^Remove/ })).not.toBeInTheDocument();
  });
});
