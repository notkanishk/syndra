// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import * as copy from "@/components/actions/copy";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import type { MyTargetView, ReadView } from "@/lib/queries/useMyStorage";

/**
 * A member checking something out: the form, the preview, the confirm, each
 * outcome and refusal, the answer that is lost and asked after (never resent),
 * the action off while one is open, and the camera only where it exists.
 */
const mocks = vi.hoisted(() => ({ request: vi.fn() }));
vi.mock("@/lib/api-client", async (orig) => ({ ...(await orig<typeof import("@/lib/api-client")>()), request: mocks.request }));
import { ApiError, LostReply } from "@/lib/api-client";

const OP = "0b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70";
const POST = `/me/targets/parts/operations/item.checkout`;
const GET = `/me/targets/parts/operations/${OP}`;

const record = (over: Partial<ReadView> = {}): ReadView => ({
  operation: "item.checkout",
  shape: "records",
  readable: true,
  result: {
    records: [{ id: "item-0003", label: "Spool holder", category: "mk4-b", since: "2026-11-12T09:00:00Z", due: "2026-11-19T09:00:00Z" }],
    taken_at: "2026-11-12T09:00:00Z",
    current: true,
  },
  ...over,
});

const view = (over: Partial<MyTargetView> = {}): MyTargetView => ({
  target: "parts",
  name: "Parts shelf",
  entitled: true,
  account: { username: "rey_mysterio" },
  kind: "inventory",
  credential: { set: false },
  reachable: true,
  health: "online",
  actions: [{ id: "item.checkout", confirm: true, params: [{ name: "item_id", type: "string", required: true, format: "target_id" }] }],
  ...over,
});

const refusalOf = (status: number, error: string, details?: Record<string, string>) => new ApiError(status, { error, message: "server words", details }, "POST");

/** What the server says next, by request: a queue per "METHOD path". */
function server(script: Record<string, Array<unknown | Error>>) {
  mocks.request.mockImplementation(async (path: string, init?: { method?: string; body?: Record<string, unknown> }) => {
    const key = `${init?.method ?? "GET"} ${path}`;
    const queue = script[key];
    if (!queue?.length) throw new Error(`unscripted ${key}`);
    const next = queue.length > 1 ? queue.shift() : queue[0];
    if (next instanceof Error) throw next;
    return next;
  });
}
const sent = (method: string, path: string) => mocks.request.mock.calls.filter(([p, init]) => p === path && (init?.method ?? "GET") === method);
const bodies = (method: string, path: string) => sent(method, path).map(([, init]) => init.body);

function renderCard(v: MyTargetView = view()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  // The page's own read of the member's systems (useMyStorage), so a confirm can read it again.
  client.setQueryDefaults(["me", "targets"], { queryFn: () => mocks.request("/me/targets") });
  client.setQueryData(["me", "targets"], { targets: [v] });
  const ui = (x: MyTargetView) => (
    <QueryClientProvider client={client}>
      <MemberTargetCard view={x} untitled />
    </QueryClientProvider>
  );
  const r = render(ui(v));
  return { ...r, client, rerenderWith: (x: MyTargetView) => r.rerender(ui(x)) };
}

const open = () => fireEvent.click(screen.getByRole("button", { name: copy.actionLabel("item.checkout") }));
const typeId = (value = "item-0003") => fireEvent.change(screen.getByLabelText("Item id"), { target: { value } });
const press = (name: string) => fireEvent.click(screen.getByRole("button", { name }));
async function toPreview() {
  open();
  typeId();
  press(copy.PREVIEW_BUTTON);
  await screen.findByText(copy.PREVIEW_LEDE);
}

beforeEach(() => {
  mocks.request.mockReset();
  delete (window as { BarcodeDetector?: unknown }).BarcodeDetector;
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("preview, then confirm", () => {
  it("previews first (nothing confirmed), shows the record, then confirms once and says Done", async () => {
    server({
      [`POST ${POST}`]: [
        { state: "preview", outcome: "done", view: record() },
        { state: "done", operation: OP, view: record() },
      ],
    });
    renderCard();
    await toPreview();
    expect(bodies("POST", POST)).toEqual([{ params: { item_id: "item-0003" }, preview: true }]);
    expect(screen.getByText("Spool holder")).toBeInTheDocument();
    expect(screen.getByText(copy.previewNote("Parts shelf"))).toBeInTheDocument();

    press(copy.CONFIRM_BUTTON);
    await screen.findByText("Done");
    expect(bodies("POST", POST)).toEqual([
      { params: { item_id: "item-0003" }, preview: true },
      { params: { item_id: "item-0003" }, confirm: true },
    ]);
    expect(screen.getByText(`${copy.DONE}.`)).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByText(NOTHING_TO_DO)).toBeInTheDocument();
    expect(screen.getByText("Spool holder")).toBeInTheDocument();
  });

  it("trims what was typed, and will not preview with the field empty", () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    open();
    expect(screen.getByRole("button", { name: copy.PREVIEW_BUTTON })).toBeDisabled();
    typeId("  item-0003 ");
    press(copy.PREVIEW_BUTTON);
    return waitFor(() => expect(bodies("POST", POST)).toEqual([{ params: { item_id: "item-0003" }, preview: true }]));
  });

  it("a preview that says they already have it offers no Confirm", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "no_change", view: record() }] });
    renderCard();
    open();
    typeId();
    press(copy.PREVIEW_BUTTON);
    await screen.findByText(copy.PREVIEW_NO_CHANGE);
    expect(screen.queryByRole("button", { name: copy.CONFIRM_BUTTON })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("a preview the page cannot read says so, offers Back, and never a blind Confirm", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record({ readable: false, result: undefined }) }] });
    renderCard();
    open();
    typeId();
    press(copy.PREVIEW_BUTTON);
    await screen.findByText(copy.PREVIEW_UNREADABLE_TITLE);
    expect(screen.getByText(copy.PREVIEW_UNREADABLE_NEXT)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: copy.CONFIRM_BUTTON })).toBeNull();
    press("Back");
    expect(screen.getByLabelText("Item id")).toHaveValue("item-0003");
  });

  it("Back from the preview keeps what was typed", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    await toPreview();
    press("Back");
    expect(screen.getByLabelText("Item id")).toHaveValue("item-0003");
  });
});

describe("what a press leaves stale", () => {
  it("a confirm reads the member's systems again, a preview does not", async () => {
    server({
      [`POST ${POST}`]: [
        { state: "preview", outcome: "done", view: record() },
        { state: "done", operation: OP, view: record() },
      ],
    });
    const { client } = renderCard();
    const spy = vi.spyOn(client, "invalidateQueries");
    await toPreview();
    expect(spy).not.toHaveBeenCalled();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText("Done");
    await waitFor(() => expect(spy.mock.calls.map((c) => c[0]?.queryKey)).toContainEqual(["me", "targets"]));
  });
});

describe("a confirm whose reply never came back", () => {
  const LOST: Array<[string, () => Error]> = [
    ["a dropped connection", () => new LostReply(new TypeError("Failed to fetch"), "POST")],
    ["a proxy 502 after the send", () => refusalOf(502, "UPSTREAM_ERROR")],
  ];
  const confirms = () => bodies("POST", POST).filter((b) => (b as { confirm?: boolean }).confirm);
  const open_ = { operation: "item.checkout", id: OP, sent_at: "2026-11-12T09:00:03Z", state: "checking" as const };

  it.each(LOST)("%s: is sent once, finds the record the server holds, and goes to checking, never Not done", async (_n, lost) => {
    server({
      [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, lost()],
      "GET /me/targets": [{ targets: [view({ unsettled: open_ })] }],
      [`GET ${GET}`]: [{ operation: OP, sent_at: "x", state: "checking" }],
    });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText(`${copy.CHECKING}.`);
    expect(confirms()).toHaveLength(1);
    expect(sent("GET", "/me/targets")).toHaveLength(1);
    expect(screen.queryByText("Not done")).toBeNull();
    expect(document.body.textContent).not.toMatch(/Nothing was changed/);
    // The row agrees with the dialog: closed, it still says checking, and the action stays off.
    press("Close");
    expect(screen.getByText(`${copy.CHECKING}.`)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: copy.actionLabel("item.checkout") })).toBeDisabled();
  });

  it.each(LOST)("%s, and no record to be found: Not sure yet, with a step, never Not done", async (_n, lost) => {
    server({
      [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, lost()],
      "GET /me/targets": [{ targets: [view()] }],
    });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText(`${copy.UNSURE}.`);
    expect(screen.getByText("Not sure yet")).toBeInTheDocument();
    expect(screen.getByText(copy.UNSURE_NEXT)).toBeInTheDocument();
    expect(screen.queryByText("Not done")).toBeNull();
    expect(confirms()).toHaveLength(1);
  });

  it("a 5xx that is not a lost reply is still a plain refusal", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, refusalOf(503, "TARGET_UNREACHABLE")] });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText("Not done");
    expect(sent("GET", "/me/targets")).toHaveLength(0);
  });
});

describe("each outcome of a confirm", () => {
  it("no_change: Nothing changed, they already have it, nothing to do", async () => {
    server({
      [`POST ${POST}`]: [
        { state: "preview", outcome: "done", view: record() },
        { state: "no_change", operation: OP, view: null },
      ],
    });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText("Nothing changed");
    expect(screen.getByText(`${copy.NO_CHANGE}.`)).toBeInTheDocument();
    expect(within(screen.getByRole("dialog")).getByText(NOTHING_TO_DO)).toBeInTheDocument();
  });
});

describe("each refusal says what is true and its one step", () => {
  const CASES: Array<[number, string, Record<string, string> | undefined]> = [
    [404, "UNKNOWN_RESOURCE", { param: "item_id" }],
    [403, "NOT_ENTITLED", { param: "item_id" }],
    [409, "RESOURCE_TAKEN", { param: "item_id" }],
    [409, "LIMIT_REACHED", { limit: "1", count: "1", unit: "items" }],
    [409, "CALL_UNSETTLED", undefined],
    [422, "ACTION_REFUSED", undefined],
    [409, "TARGET_PAUSED", undefined],
    [503, "TARGET_UNREACHABLE", undefined],
    [503, "CALL_UNSETTLED_UNKNOWN", undefined],
    [429, "TOO_MANY_ATTEMPTS", undefined],
    [429, "RATE_LIMITED", undefined],
  ];

  it.each(CASES)("a confirm refused %i %s", async (status, code, details) => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, refusalOf(status, code, details)] });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    const r = copy.refusal(code, details)!;
    await screen.findByText(new RegExp(r.message.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
    expect(screen.getByText(r.next)).toBeInTheDocument();
    expect(screen.getByText("Not done")).toBeInTheDocument();
    // Not the server's words, and never a status code.
    expect(document.body.textContent).not.toMatch(/server words|\b(404|403|409|422|503|429)\b/);
    // A refusal that already says nothing went out is not told "Nothing was changed." as well.
    if (/nothing went out/i.test(r.message)) expect(document.body.textContent).not.toMatch(/Nothing was changed/);
  });

  it.each([
    [404, "UNKNOWN_RESOURCE"],
    [409, "RESOURCE_TAKEN"],
    [503, "PREVIEW_UNANSWERED"],
  ] as const)("a preview refused %i %s keeps the form, with what was typed, and shows the step", async (status, code) => {
    server({ [`POST ${POST}`]: [refusalOf(status, code, { param: "item_id" })] });
    renderCard();
    open();
    typeId();
    press(copy.PREVIEW_BUTTON);
    const r = copy.refusal(code, { param: "item_id" })!;
    await screen.findByText(r.next);
    expect(screen.getByLabelText("Item id")).toHaveValue("item-0003");
    expect(screen.queryByRole("button", { name: copy.CONFIRM_BUTTON })).toBeNull();
    expect(bodies("POST", POST).every((b) => (b as { confirm?: boolean }).confirm === undefined)).toBe(true);
  });

  it("a refusal that leaves something to fix offers Back to the form", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, refusalOf(409, "RESOURCE_TAKEN", { param: "item_id" })] });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText(copy.refusal("RESOURCE_TAKEN")!.next);
    press("Back");
    expect(screen.getByLabelText("Item id")).toHaveValue("item-0003");
  });

  it("a code the page has no words for still ends on a step", async () => {
    server({ [`POST ${POST}`]: [refusalOf(400, "VALIDATION_FAILED")] });
    renderCard();
    open();
    typeId();
    press(copy.PREVIEW_BUTTON);
    await screen.findByText("Not done");
    expect(screen.getByRole("alert").textContent).toMatch(/Change what it says above, then try again\./);
  });
});

describe("a confirm whose answer is lost", () => {
  async function toChecking(states: Array<unknown | Error>) {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    server({
      [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, { state: "checking", operation: OP, sent_at: "2026-11-12T09:00:03Z" }],
      [`GET ${GET}`]: states,
    });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText(`${copy.CHECKING}.`);
  }
  const polls = () => sent("GET", GET).length;
  const wait = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

  it("says it is checking, asks every 5 s, stops when settled, and shows the record", async () => {
    await toChecking([
      { operation: OP, sent_at: "x", state: "checking" },
      { operation: OP, sent_at: "x", state: "checking" },
      { operation: OP, sent_at: "x", state: "done", view: record() },
    ]);
    expect(screen.getByText(copy.CHECKING_NEXT)).toBeInTheDocument();
    await waitFor(() => expect(polls()).toBe(1));
    await wait(4000);
    expect(polls()).toBe(1);
    await wait(1100);
    expect(polls()).toBe(2);
    await wait(5100);
    await screen.findByText("Done");
    expect(screen.getByText("Spool holder")).toBeInTheDocument();
    const settledAt = polls();
    await wait(30_000);
    expect(polls()).toBe(settledAt);
  });

  it("never sends the confirm again, while checking or after", async () => {
    await toChecking([
      { operation: OP, sent_at: "x", state: "checking" },
      { operation: OP, sent_at: "x", state: "not_done" },
    ]);
    await wait(12_000);
    await screen.findByText("Not done");
    expect(screen.getByText(copy.NOT_DONE_NEXT)).toBeInTheDocument();
    expect(bodies("POST", POST).filter((b) => (b as { confirm?: boolean }).confirm)).toHaveLength(1);
  });

  it("an hour in doubt: says staff have been told, and stops asking", async () => {
    await toChecking([{ operation: OP, sent_at: "x", state: "ask_desk" }]);
    await screen.findByText(`${copy.ASK_DESK}.`);
    expect(screen.getByText(copy.ASK_DESK_NEXT)).toBeInTheDocument();
    expect(screen.getByText(copy.ASK_DESK_DETAIL)).toBeInTheDocument();
    const n = polls();
    await wait(20_000);
    expect(polls()).toBe(n);
  });

  it("keeps asking when an ask does not get through, and says so", async () => {
    await toChecking([new Error("down"), { operation: OP, sent_at: "x", state: "no_change" }]);
    await screen.findByText(copy.CHECKING_LATE);
    await wait(5100);
    await screen.findByText("Nothing changed");
  });

  it("their last one still open: shows that one and asks after it, sending nothing", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    server({
      [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, refusalOf(409, "CALL_UNSETTLED", { operation: OP, sent_at: "2026-11-12T09:00:03Z" })],
      [`GET ${GET}`]: [{ operation: OP, sent_at: "x", state: "checking" }],
    });
    renderCard();
    await toPreview();
    press(copy.CONFIRM_BUTTON);
    await screen.findByText(`${copy.CHECKING}.`);
    await waitFor(() => expect(polls()).toBe(1));
    expect(bodies("POST", POST).filter((b) => (b as { confirm?: boolean }).confirm)).toHaveLength(1);
  });

  it("closing the dialog leaves the row checking, and the action off", async () => {
    await toChecking([{ operation: OP, sent_at: "x", state: "checking" }]);
    press("Close");
    expect(screen.getByText(`${copy.CHECKING}.`)).toBeInTheDocument();
    // The list has not caught up yet; the page's own poll is enough to keep the action off.
    expect(screen.getByRole("button", { name: copy.actionLabel("item.checkout") })).toBeDisabled();
    expect(screen.getByText(copy.UNSETTLED_REASON)).toBeInTheDocument();
  });
});

describe("while a call is unsettled", () => {
  const unsettled = (state: "checking" | "ask_desk") => ({ operation: "item.checkout", id: OP, sent_at: "2026-11-12T09:00:03Z", state });

  it("the action is off, says why, and the page asks after the call without sending anything", async () => {
    server({ [`GET ${GET}`]: [{ operation: OP, sent_at: "x", state: "checking" }] });
    renderCard(view({ unsettled: unsettled("checking") }));
    const button = screen.getByRole("button", { name: copy.actionLabel("item.checkout") });
    expect(button).toBeDisabled();
    expect(screen.getByText(copy.UNSETTLED_REASON)).toBeInTheDocument();
    await screen.findByText(`${copy.CHECKING}.`);
    fireEvent.click(button);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(sent("POST", POST)).toHaveLength(0);
  });

  it("an unsettled call of another action leaves this one on", () => {
    server({});
    renderCard(view({ unsettled: { ...unsettled("checking"), operation: "item.return" } }));
    expect(screen.getByRole("button", { name: copy.actionLabel("item.checkout") })).toBeEnabled();
  });

  it("a settled call keeps its result on the row after the server stops listing it, and the action is back", async () => {
    server({ [`GET ${GET}`]: [{ operation: OP, sent_at: "x", state: "not_done" }] });
    const { rerenderWith } = renderCard(view({ unsettled: unsettled("checking") }));
    await screen.findByText("Not done");
    rerenderWith(view());
    expect(screen.getByText(copy.NOT_DONE_NEXT)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: copy.actionLabel("item.checkout") })).toBeEnabled();
  });

  it("an hour in doubt: the reason under the off button ends on the desk and no longer says being checked", async () => {
    server({ [`GET ${GET}`]: [{ operation: OP, sent_at: "x", state: "ask_desk" }] });
    renderCard(view({ unsettled: unsettled("ask_desk") }));
    expect(screen.getByText(copy.UNSETTLED_DESK_REASON)).toBeInTheDocument();
    expect(screen.queryByText(copy.UNSETTLED_REASON)).toBeNull();
    expect(screen.getByRole("button", { name: copy.actionLabel("item.checkout") })).toBeDisabled();
  });

  it("a system with no account lists no action", () => {
    renderCard(view({ actions: undefined, account: undefined }));
    expect(screen.queryByRole("button", { name: copy.actionLabel("item.checkout") })).toBeNull();
  });
});

describe("one press, one send", () => {
  const confirmPosts = () => bodies("POST", POST).filter((b) => (b as { confirm?: boolean }).confirm);

  it("two clicks on Confirm in the same tick send one confirm", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }, { state: "done", operation: OP, view: record() }] });
    renderCard();
    await toPreview();
    const button = screen.getByRole("button", { name: copy.CONFIRM_BUTTON });
    act(() => {
      fireEvent.click(button);
      fireEvent.click(button);
    });
    await screen.findByText("Done");
    expect(confirmPosts()).toHaveLength(1);
  });

  it("two clicks on the preview button in the same tick send one preview", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    open();
    typeId();
    const button = screen.getByRole("button", { name: copy.PREVIEW_BUTTON });
    act(() => {
      fireEvent.click(button);
      fireEvent.click(button);
    });
    await screen.findByText(copy.PREVIEW_LEDE);
    expect(bodies("POST", POST)).toHaveLength(1);
  });

  it("can be sent again once a send has settled (Back, then another preview)", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    await toPreview();
    press("Back");
    press(copy.PREVIEW_BUTTON);
    await waitFor(() => expect(bodies("POST", POST)).toHaveLength(2));
  });

  it("the Confirm button is not the preview button: focus does not carry a second Enter onto Confirm", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    open();
    typeId();
    const previewButton = screen.getByRole("button", { name: copy.PREVIEW_BUTTON });
    previewButton.focus();
    fireEvent.click(previewButton);
    await screen.findByText(copy.PREVIEW_LEDE);
    const confirm = screen.getByRole("button", { name: copy.CONFIRM_BUTTON });
    expect(previewButton.isConnected).toBe(false);
    expect(document.activeElement).not.toBe(confirm);
    expect(confirmPosts()).toHaveLength(0);
  });
});

describe("the camera", () => {
  const track = { stop: vi.fn() };
  const stream = { getTracks: () => [track] };
  const getUserMedia = vi.fn();
  let detect: ReturnType<typeof vi.fn>;
  let formats: string[];
  const install = () =>
    ((window as unknown as { BarcodeDetector: unknown }).BarcodeDetector = class {
      static getSupportedFormats = () => Promise.resolve(formats);
      detect = detect;
    });

  beforeEach(() => {
    track.stop.mockReset();
    getUserMedia.mockReset().mockResolvedValue(stream);
    Object.defineProperty(navigator, "mediaDevices", { value: { getUserMedia }, configurable: true });
    HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined);
    detect = vi.fn().mockResolvedValue([]);
    formats = ["qr_code", "code_128"];
    install();
  });

  // The offer is decided after mount, so wait for it where it is expected and flush where it is not.
  const scanButton = () => screen.queryByRole("button", { name: copy.SCAN_BUTTON });
  const offered = () => screen.findByRole("button", { name: copy.SCAN_BUTTON });
  const settle = () => act(async () => {});

  it("is offered only where the browser has BarcodeDetector, and the typed field is always there", async () => {
    delete (window as { BarcodeDetector?: unknown }).BarcodeDetector;
    renderCard();
    open();
    await settle();
    expect(scanButton()).toBeNull();
    expect(screen.getByLabelText("Item id")).toBeInTheDocument();
    cleanup();
    install();
    renderCard();
    open();
    expect(await offered()).toBeInTheDocument();
  });

  it("is named by its visible words (WCAG 2.5.3): no aria-label of its own", async () => {
    renderCard();
    open();
    expect(await offered()).not.toHaveAttribute("aria-label");
  });

  it("is not offered where BarcodeDetector reads no formats, or none a label uses", async () => {
    for (const none of [[], ["pdf417", "aztec"]]) {
      formats = none;
      renderCard();
      open();
      await settle();
      expect(scanButton()).toBeNull();
      cleanup();
    }
  });

  it("is not offered when a detector cannot say what it reads", async () => {
    (window as unknown as { BarcodeDetector: unknown }).BarcodeDetector = class {};
    renderCard();
    open();
    await settle();
    expect(scanButton()).toBeNull();
  });

  it("is not offered without a camera to open", async () => {
    Object.defineProperty(navigator, "mediaDevices", { value: undefined, configurable: true });
    renderCard();
    open();
    await settle();
    expect(scanButton()).toBeNull();
  });

  it("the field is a typing field for a wedge scanner: off autocomplete, text input mode, Go key", () => {
    renderCard();
    open();
    const input = screen.getByLabelText("Item id");
    expect(input).toHaveAttribute("autocomplete", "off");
    expect(input).toHaveAttribute("inputmode", "text");
    expect(input).toHaveAttribute("enterkeyhint", "go");
  });

  it("a wedge scanner types the id and presses Enter: that previews", async () => {
    server({ [`POST ${POST}`]: [{ state: "preview", outcome: "done", view: record() }] });
    renderCard();
    open();
    typeId("item-0003");
    fireEvent.submit(screen.getByLabelText("Item id").closest("form")!);
    await screen.findByText(copy.PREVIEW_LEDE);
  });

  it("asks for the camera only when pressed, not when the dialog opens", async () => {
    renderCard();
    open();
    expect(getUserMedia).not.toHaveBeenCalled();
    fireEvent.click(await offered());
    await waitFor(() => expect(getUserMedia).toHaveBeenCalledTimes(1));
  });

  it("fills the field with the first label read and stops the stream", async () => {
    detect.mockResolvedValue([{ rawValue: "item-0042" }]);
    renderCard();
    open();
    fireEvent.click(await offered());
    await waitFor(() => expect(screen.getByLabelText("Item id")).toHaveValue("item-0042"));
    expect(track.stop).toHaveBeenCalled();
    // Back on the form, not a second dialog.
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });

  it("stops the stream when closed without a read", async () => {
    renderCard();
    open();
    fireEvent.click(await offered());
    await waitFor(() => expect(detect).toHaveBeenCalled());
    expect(track.stop).not.toHaveBeenCalled();
    press("Close");
    expect(track.stop).toHaveBeenCalled();
    expect(screen.getByLabelText("Item id")).toBeInTheDocument();
  });

  it("stops a stream that arrives after it was closed", async () => {
    let arrive!: (s: unknown) => void;
    getUserMedia.mockReturnValue(new Promise((r) => (arrive = r)));
    renderCard();
    open();
    fireEvent.click(await offered());
    press("Close");
    await act(async () => arrive(stream));
    expect(track.stop).toHaveBeenCalled();
  });

  it("a camera that is refused says to type instead, and the field still works", async () => {
    getUserMedia.mockRejectedValue(new DOMException("no", "NotAllowedError"));
    renderCard();
    open();
    fireEvent.click(await offered());
    await screen.findByText(copy.SCAN_DENIED);
    press("Close");
    expect(screen.getByLabelText("Item id")).toBeInTheDocument();
  });
});
