// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UnconfirmedChanges } from "@/components/targets/UnconfirmedChanges";
import type { UnconfirmedOperation } from "@/lib/queries/useTargets";

// A-09 §3–4: a change the system never answered, settled by Say what happened.

const NAMES: Record<string, string> = { u9: "Sami Zayn", op1: "Kanishk Singh" };
vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig));
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{NAMES[id] ?? id}</span>,
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => ({ value: NAMES[id] ? { display_name: NAMES[id] } : undefined, resolved: true }),
  }),
}));

const answer = {
  sent: [] as Array<{ id: string; happened: boolean }>,
  isSuccess: false,
  variables: undefined as { id: string; happened: boolean } | undefined,
};
vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  return {
    ...actual,
    useAnswerUnconfirmed: () => ({
      mutate: (input: { id: string; happened: boolean }) => answer.sent.push(input),
      isPending: false,
      isSuccess: answer.isSuccess,
      variables: answer.variables,
      error: null,
    }),
  };
});

const PURGE: UnconfirmedOperation = {
  id: "0b0b0b0b-0000-4000-8000-000000000001",
  operation: "account.purge",
  subject_id: "u9",
  actor_id: "op1",
  about: "sami.zayn",
  sent_at: "2026-11-02T18:05:00Z",
};

function renderIt(operations: UnconfirmedOperation[]) {
  const client = new QueryClient();
  return render(
    <QueryClientProvider client={client}>
      <UnconfirmedChanges target="truenas" operations={operations} ready />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  answer.sent = [];
  answer.isSuccess = false;
  answer.variables = undefined;
});

describe("Waiting on you: changes nobody confirmed", () => {
  it("no rows: nothing shows, no card and no zero", () => {
    const { container } = renderIt([]);
    expect(container.textContent).toBe("");
  });

  it("says what was asked, that it is not confirmed, when and by whom, and one verb", () => {
    renderIt([PURGE]);
    expect(screen.getByText("Not confirmed")).toBeTruthy();
    expect(screen.getByText(/^Sent to TrueNAS on 2 Nov( 2026)? at \d\d:\d\d\. No answer came back, so it may or may not have happened\.$/)).toBeTruthy();
    expect(screen.getByText("Kanishk Singh").closest("span")!.parentElement!.textContent).toBe("Asked by Kanishk Singh");
    expect(screen.getByRole("button", { name: /^Say what happened/ })).toBeTruthy();
    // No id on the row: it belongs in Audit.
    expect(document.body.textContent).not.toContain(PURGE.id);
  });

  // one-path Open: a maintenance change is about the system, never "a change for <person>".
  it("words a maintenance change as the system's own state", () => {
    renderIt([{ ...PURGE, operation: "lifecycle.set", subject_id: "truenas", about: "read_only" }]);
    expect(document.body.textContent).toContain("Set TrueNAS to read-only");
    expect(document.body.textContent).not.toMatch(/a change for/i);
  });

  // addon-contract 6c: a physical act is settled at the desk, so the row says to ask there.
  it("words a checkout whose answer was lost: ask at the desk, the item and the person", () => {
    const checkout = { ...PURGE, operation: "item.checkout", about: "item-0003" };
    renderIt([checkout]);
    expect(document.body.textContent).toContain("Check out item-0003 to Sami Zayn");
    expect(document.body.textContent).not.toMatch(/a change for|account and files/i);
    fireEvent.click(screen.getByRole("button", { name: /^Say what happened/ }));
    expect(screen.getByRole("heading", { name: "Did item-0003 go out to Sami Zayn?" })).toBeTruthy();
    expect(screen.getByText("Ask at the desk whether item-0003 went out to Sami Zayn. If it did, it happened.")).toBeTruthy();
    // The same two answers as every other row.
    fireEvent.click(screen.getByText("It happened"));
    fireEvent.click(screen.getByRole("button", { name: "Record it as done" }));
    expect(answer.sent).toEqual([{ id: checkout.id, happened: true }]);
  });

  it("names Syndra when Syndra asked by itself", () => {
    renderIt([{ ...PURGE, actor_id: "system:reconcile" }]);
    expect(screen.getByText("Asked by Syndra, bringing accounts in line")).toBeTruthy();
  });

  it("asks the operator to look first, and records nothing until they choose", () => {
    renderIt([PURGE]);
    fireEvent.click(screen.getByRole("button", { name: /^Say what happened/ }));
    expect(screen.getByRole("heading", { name: "Did TrueNAS delete Sami Zayn's account and files?" })).toBeTruthy();
    expect(screen.getByText(/^Read TrueNAS again\. If sami\.zayn is gone from Accounts on TrueNAS, it happened\./)).toBeTruthy();
    expect(screen.getByText("Nothing is sent to TrueNAS. This records what you found, with your name.")).toBeTruthy();

    const confirm = screen.getByRole("button", { name: "Record what happened" });
    expect((confirm as HTMLButtonElement).disabled).toBe(true);

    fireEvent.click(screen.getByText("It didn't happen"));
    fireEvent.click(screen.getByRole("button", { name: "Record it as not done" }));
    expect(answer.sent).toEqual([{ id: PURGE.id, happened: false }]);

    fireEvent.click(screen.getByText("It happened"));
    expect(screen.getByRole("button", { name: "Record it as done" })).toBeTruthy();
  });

  it("the sheet becomes its result", () => {
    answer.isSuccess = true;
    answer.variables = { id: PURGE.id, happened: true };
    renderIt([PURGE]);
    fireEvent.click(screen.getByRole("button", { name: /^Say what happened/ }));
    expect(screen.getByText("Done")).toBeTruthy();
    expect(screen.getByText("TrueNAS deleted Sami Zayn's account and files.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Close" })).toBeTruthy();
  });
});
