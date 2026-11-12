// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { StatsRetention } from "@/components/targets/StatsRetention";
import { targetStats } from "@/components/usage/__tests__/fixtures";
import * as copy from "@/components/usage/copy";
import { makeProxyFetch, respondWith } from "@/test-utils/proxyFetch";

afterEach(() => vi.unstubAllGlobals());

const PUT = "/api/proxy/targets/gym/stats-retention";

function mount(retention: { subject_days: number; set: boolean }, put: (body: unknown) => unknown = (b) => ({ target: "gym", subject_days: (b as { days: number | null }).days ?? 400, set: (b as { days: number | null }).days !== null })) {
  const stub = makeProxyFetch();
  stub.register("GET", "/api/proxy/targets/gym/stats", () => targetStats({ retention }));
  stub.register("PUT", PUT, ({ body }) => put(body));
  vi.stubGlobal("fetch", stub.fetchImpl);
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <StatsRetention target="gym" />
    </QueryClientProvider>,
  );
  return stub;
}
const puts = (stub: ReturnType<typeof makeProxyFetch>) => stub.calls.filter((c) => c.method === "PUT");
const field = async () => (await screen.findByLabelText(copy.RETENTION_FIELD)) as HTMLInputElement;
const type = (input: HTMLInputElement, value: string) => fireEvent.change(input, { target: { value } });

describe("the retention control", () => {
  it("is a card with its number field (1 to 400) and the current state as its hint: the one statement", async () => {
    mount({ subject_days: 400, set: false });
    const input = await field();
    expect(screen.getByText(copy.RETENTION_TITLE)).toBeInTheDocument();
    expect(input).toHaveAttribute("type", "number");
    expect(input).toHaveAttribute("min", "1");
    expect(input).toHaveAttribute("max", "400");
    expect(input).toHaveValue(400);
    expect(screen.getByText(copy.retentionHint(400, false))).toBeInTheDocument();
    // Nothing shortened, so nothing to go back from.
    expect(screen.queryByRole("button", { name: copy.RETENTION_FULL })).toBeNull();
    expect(screen.getByRole("button", { name: copy.RETENTION_SAVE })).toBeDisabled();
  });

  it("says nothing where the add-on reports no statistics", async () => {
    const stub = makeProxyFetch();
    stub.register("GET", "/api/proxy/targets/gym/stats", () => targetStats({ usage: { reported: false } }));
    vi.stubGlobal("fetch", stub.fetchImpl);
    const { container } = render(
      <QueryClientProvider client={new QueryClient()}>
        <StatsRetention target="gym" />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(stub.calls).toHaveLength(1));
    expect(container).toBeEmptyDOMElement();
  });

  it("asks before shortening, in a dialog, since it deletes history for good, and sends nothing until told to", async () => {
    const stub = mount({ subject_days: 400, set: false });
    type(await field(), "90");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    const dialog = await screen.findByRole("dialog", { name: copy.retentionAskTitle(90) });
    expect(within(dialog).getByText(copy.retentionAsk(90))).toBeInTheDocument();
    expect(puts(stub)).toHaveLength(0);
    // The destructive confirm names the number, in the destructive fill; the way out is "Keep them".
    const yes = within(dialog).getByRole("button", { name: copy.retentionAskYes(90) });
    expect(yes.className).toContain("bg-danger");
    expect(within(dialog).getByRole("button", { name: copy.RETENTION_ASK_NO })).toBeInTheDocument();
    fireEvent.click(yes);
    expect(await screen.findByText(copy.retentionSaved(90))).toBeInTheDocument();
    expect(puts(stub).map((c) => c.body)).toEqual([{ days: 90 }]);
  });

  it("lets the operator keep the numbers after all", async () => {
    const stub = mount({ subject_days: 400, set: false });
    type(await field(), "90");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    fireEvent.click(await screen.findByRole("button", { name: copy.RETENTION_ASK_NO }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts(stub)).toHaveLength(0);
  });

  it("does not ask before lengthening: nothing that exists is deleted", async () => {
    const stub = mount({ subject_days: 90, set: true });
    type(await field(), "200");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    expect(await screen.findByText(copy.retentionSaved(200))).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(puts(stub).map((c) => c.body)).toEqual([{ days: 200 }]);
  });

  it("goes back to the full 400 days with null, only offered when a shorter value is set, and does not ask", async () => {
    const stub = mount({ subject_days: 90, set: true });
    await field();
    expect(screen.getByText(copy.retentionHint(90, true))).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_FULL }));
    expect(await screen.findByText(copy.retentionSaved(400))).toBeInTheDocument();
    expect(puts(stub).map((c) => c.body)).toEqual([{ days: null }]);
  });

  it("shows the result as an applied outcome, with its step", async () => {
    mount({ subject_days: 90, set: true });
    type(await field(), "120");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    expect(await screen.findByText(copy.RETENTION_WORD)).toBeInTheDocument();
    expect(screen.getByText(copy.NOTHING_TO_DO)).toBeInTheDocument();
  });

  it("shows a 422 in the backend's words and keeps the number typed, with no result", async () => {
    const message = "Per-person statistics are kept between 1 and 400 days.";
    const stub = mount({ subject_days: 90, set: true }, () => respondWith(422, { error: "VALIDATION_FAILED", message, details: { days: "out_of_range" } }));
    const input = await field();
    type(input, "500");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(puts(stub)).toHaveLength(1);
    expect(screen.queryByText(copy.RETENTION_WORD)).toBeNull();
    expect(input).toHaveValue(500);
  });

  it("never asks to delete for a number the backend refuses: 0 is sent as typed and its 422 is the only answer", async () => {
    const message = "Per-person statistics are kept between 1 and 400 days.";
    const stub = mount({ subject_days: 400, set: false }, () => respondWith(422, { error: "VALIDATION_FAILED", message, details: { days: "out_of_range" } }));
    type(await field(), "0");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(puts(stub).map((c) => c.body)).toEqual([{ days: 0 }]);
  });

  it("closes the dialog on a refusal and shows it, never silently retrying; typing again clears it", async () => {
    const stub = mount({ subject_days: 400, set: false }, () => respondWith(422, { error: "VALIDATION_FAILED", message: "no", details: {} }));
    const input = await field();
    type(input, "90");
    fireEvent.click(screen.getByRole("button", { name: copy.RETENTION_SAVE }));
    fireEvent.click(await screen.findByRole("button", { name: copy.retentionAskYes(90) }));
    await screen.findByRole("alert");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(puts(stub)).toHaveLength(1);
    // An old refusal is not about the number typed next.
    type(input, "91");
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });
});
