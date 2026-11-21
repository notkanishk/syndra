// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TargetOverview } from "@/components/targets/TargetOverview";

/**
 * The Accounts region's line, with the real hooks and the API's own shapes: a read with nothing to
 * give is a 503 TARGET_NOT_READ (design §13.3), never a body read as a list, and Read again on the
 * line reads afresh, waits out a cooldown with focus kept, and hands focus on only when it lands.
 */

vi.mock("@/lib/queries/useTargetSystemHealth", () => ({
  useTargetSystemHealth: () => ({ data: undefined, isLoading: false, error: null }),
}));
vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  const idle = () => ({ data: undefined, isLoading: false, isError: false, error: null, refetch: vi.fn() });
  const inert = () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false, error: null, data: undefined, reset: vi.fn() });
  const health = actual.healthReplyOf({ read: { state: "now", taken_at: new Date().toISOString() }, reachable: true, lifecycle: "active" });
  return {
    ...actual,
    useTargets: () => ({ ...idle(), data: [] }),
    useUnconfirmedOperations: () => ({ data: [], isLoading: false, error: null }),
    useMergeFindings: () => ({ ...idle(), data: [] }),
    useTargetHealth: () => ({ ...idle(), data: health }),
    useResolveLogFinding: inert,
    useReconcileTarget: inert,
    useSetLifecycle: inert,
    useReleaseBinding: inert,
    useAdoptAccount: inert,
  };
});
vi.mock("@/components/targets/MappingManagement", () => ({ MappingManagement: () => null }));
vi.mock("@/components/targets/PeopleOnTarget", () => ({ PeopleOnTarget: () => null }));
vi.mock("@/components/targets/DormantAccounts", () => ({ DormantAccounts: () => null }));
vi.mock("@/components/targets/MergeFindings", () => ({ MergeFindings: () => null }));

afterEach(() => vi.unstubAllGlobals());

const ADDON_DOWN = "Syndra can't reach TrueNAS's add-on. Check that its container is running. Syndra keeps trying.";
const notRead = (reason = "addon_unreachable", message = ADDON_DOWN, wait = 30) =>
  new Response(JSON.stringify({ error: "TARGET_NOT_READ", message, details: { reason, cause: "no_answer" } }), {
    status: 503,
    headers: { "Retry-After": String(wait) },
  });
const listed = () =>
  new Response(JSON.stringify({ target: "truenas", bound: 0, accounts: [], unmanaged: [], read: { state: "now", taken_at: new Date().toISOString() } }));
const never = () => new Promise<Response>(() => {});

function renderPage(client = new QueryClient({ defaultOptions: { queries: { retry: false } } })) {
  return render(
    <QueryClientProvider client={client}>
      <TargetOverview target="truenas" />
    </QueryClientProvider>,
  );
}

describe("an account list with nothing to give, as the API answers it", () => {
  it("says the backend's sentence once, above both cards, and never the empty list", async () => {
    const detail = "Syndra's records of who has which account on TrueNAS can't be read. Check the add-on's storage, then try again.";
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => (String(input).endsWith("/inventory") ? Promise.resolve(notRead("read_refused", detail)) : never())));
    renderPage();
    const line = await screen.findByText(`The account list could not be read. ${detail}`);
    expect(screen.getAllByText(/The account list could not be read/)).toHaveLength(1);
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    // Above both cards, not inside one: the region's line, before "Not created by Syndra".
    expect(line.compareDocumentPosition(screen.getByText("Not created by Syndra")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("draws neither an empty list nor Read again while the first load is on its way", () => {
    vi.stubGlobal("fetch", vi.fn(never));
    renderPage();
    expect(screen.queryByText("Nothing outside Syndra.")).toBeNull();
    expect(screen.queryByRole("button", { name: "Read again" })).toBeNull();
  });
});

describe("Read again on the region's line", () => {
  it("says Reading while it reads, then reads afresh", async () => {
    let answer: (r: Response) => void = () => {};
    const fetched = vi.fn((input: RequestInfo | URL) =>
      String(input).includes("fresh=true")
        ? new Promise<Response>((resolve) => (answer = resolve))
        : String(input).includes("/inventory")
          ? Promise.resolve(notRead())
          : never(),
    );
    vi.stubGlobal("fetch", fetched);
    renderPage();
    const line = await screen.findByText(/^The account list could not be read\./);
    fireEvent.click(within(line.parentElement!).getByRole("button", { name: "Read again" }));
    expect(await within(line.parentElement!).findByRole("button", { name: "Reading" })).toHaveAttribute("aria-disabled", "true");
    expect(fetched.mock.calls.some(([u]) => String(u).includes("fresh=true"))).toBe(true);
    answer(notRead());
    expect(await within(line.parentElement!).findByRole("button", { name: "Read again" })).toBeInTheDocument();
  });

  it("keeps its status region quiet while nothing waits", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => (String(input).includes("/inventory") ? Promise.resolve(notRead()) : never())));
    renderPage();
    const line = await screen.findByText(/^The account list could not be read\./);
    expect(within(line.parentElement!).getByRole("status").textContent).toBe("");
  });

  it("inside the cooldown says the one wait wording, keeps focus on the waiting button, and says the wait once", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).includes("fresh=true")
          ? Promise.resolve(notRead("cooldown", "TrueNAS was read moments ago. Read it again in 30 s.", 30))
          : String(input).includes("/inventory")
            ? Promise.resolve(notRead())
            : never(),
      ),
    );
    renderPage();
    const line = await screen.findByText(/^The account list could not be read\./);
    const button = within(line.parentElement!).getByRole("button", { name: "Read again" });
    button.focus();
    fireEvent.click(button);
    const said = await screen.findAllByText(/^Truenas was read moments ago; you can read again in \d+ s$/i);
    const countdown = said.find((el) => el.id)!;
    const waiting = within(countdown.parentElement!).getByRole("button", { name: "Read again" });
    expect(waiting.getAttribute("aria-describedby")).toBe(countdown.id);
    // Off without leaving the tab order: focus stays where the press was.
    expect(waiting).toHaveAttribute("aria-disabled", "true");
    expect(waiting).not.toBeDisabled();
    expect(document.activeElement).toBe(waiting);
    expect(within(countdown.parentElement!).getByRole("status").textContent).toMatch(/you can read again in \d+ s$/);
  });

  it("leaves focus alone when a press failed and a later read lands by itself", async () => {
    let answered = false;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).includes("fresh=true")
          ? Promise.resolve(new Response(JSON.stringify({ error: "INVENTORY_ERROR", message: "Syndra's server had a problem." }), { status: 500 }))
          : String(input).includes("/inventory")
            ? Promise.resolve(answered ? listed() : notRead())
            : never(),
      ),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderPage(client);
    const line = await screen.findByText(/^The account list could not be read\./);
    fireEvent.click(within(line.parentElement!).getByRole("button", { name: "Read again" }));
    await screen.findByText("Not read");
    answered = true;
    await client.refetchQueries({ queryKey: ["targets", "truenas", "inventory"] });
    await waitFor(() => expect(screen.queryByText(/^The account list could not be read\./)).toBeNull());
    expect(document.activeElement).toBe(document.body);
  });

  // A press that settles with nothing again (a none, not a failure) spends the focus intent too.
  it("leaves focus alone when a press came back with nothing and a later read lands by itself", async () => {
    let answered = false;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).includes("fresh=true")
          ? Promise.resolve(notRead())
          : String(input).includes("/inventory")
            ? Promise.resolve(answered ? listed() : notRead())
            : never(),
      ),
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    renderPage(client);
    const line = await screen.findByText(/^The account list could not be read\./);
    fireEvent.click(within(line.parentElement!).getByRole("button", { name: "Read again" }));
    await waitFor(() => expect(within(screen.getByText(/^The account list could not be read\./).parentElement!).getByRole("button", { name: "Read again" })).toBeInTheDocument());
    (document.activeElement as HTMLElement | null)?.blur();
    answered = true;
    await client.refetchQueries({ queryKey: ["targets", "truenas", "inventory"] });
    await waitFor(() => expect(screen.queryByText(/^The account list could not be read\./)).toBeNull());
    expect(document.activeElement).toBe(document.body);
  });

  it("hands focus to the region's heading when a read lands and takes the line away", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) =>
        String(input).includes("fresh=true") ? Promise.resolve(listed()) : String(input).includes("/inventory") ? Promise.resolve(notRead()) : never(),
      ),
    );
    renderPage();
    const line = await screen.findByText(/^The account list could not be read\./);
    const button = within(line.parentElement!).getByRole("button", { name: "Read again" });
    button.focus();
    fireEvent.click(button);
    await waitFor(() => expect(screen.queryByText(/^The account list could not be read\./)).toBeNull());
    const heading = document.getElementById("accounts-heading")!;
    expect(document.activeElement).toBe(heading);
    // Focusable for that moment only (GOV.UK Frontend's setFocus): never a tab stop at rest.
    expect(heading.getAttribute("tabindex")).toBe("-1");
    heading.blur();
    expect(heading.hasAttribute("tabindex")).toBe(false);
  });
});

describe("a whole account list that holds nothing", () => {
  it("counts 0 on the card, a count that was taken", async () => {
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL) => (String(input).includes("/inventory") ? Promise.resolve(listed()) : never())));
    renderPage();
    await screen.findByText("Nothing outside Syndra.");
    const header = screen.getByText("Not created by Syndra").parentElement!;
    expect(within(header).getByTestId("card-count").textContent).toBe("0");
  });
});
