// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { KEPT, PushedStep, useCancelStep, useForetell, type CancelStepOptions } from "@/components/ui/CancelQueuedStep";
import { Modal } from "@/components/ui/Modal";
import { WouldCancelQueued } from "@/lib/api-client";
import type { QueuedChange } from "@/lib/cancel-confirm";
import { motionInTests } from "@/test-utils/motion";

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: (id: string) => {
      const names: Record<string, string> = { u1: "Rohan Mehta", "op-2": "Sam Iyer", "op-3": "Lena Ortiz" };
      return names[id] ? { value: { display_name: names[id], email: "" }, resolved: true } : { value: undefined, resolved: true };
    },
    resolveProject: (id: string) => ({ value: id === "pNas" ? { name: "Network storage" } : undefined, resolved: true }),
    resolveRole: (_p: string, key: string) => ({ value: key === "member" ? { display_name: "Member" } : undefined, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

const hoursAgo = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();

function change(over: Partial<QueuedChange> = {}): QueuedChange {
  return {
    id: "00000000-0000-0000-0000-000000000001",
    op_type: "revoke",
    user_id: "u1",
    project_id: "pNas",
    role_keys: ["member"],
    source: "direct",
    initiated_by: "op-2",
    said: "left the course",
    queued_at: hoursAgo(2),
    ...over,
  };
}

const idOf = (n: number) => `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`;
const refusal = (cancels: QueuedChange[]) => new WouldCancelQueued(cancels, "POST", "/users/u1/grants");

/** A host sheet: a form with a field, a Send button, and the step pushed inside it. */
function Host({
  write,
  options,
  onResult,
}: {
  write: (cancelIds?: string[]) => Promise<unknown>;
  options?: Partial<CancelStepOptions>;
  onResult?: (result: unknown) => void;
}) {
  const step = useCancelStep({ host: "Give Rohan Mehta a role", goAhead: "give Member", ...options });
  const [done, setDone] = useState("");
  return (
    <Modal open onClose={() => undefined} size="md" labelledBy="host-title">
      <PushedStep step={step}>
        <h2 id="host-title">Give a role</h2>
        <input aria-label="Reason" defaultValue="" />
        <button
          onClick={async () => {
            try {
              const result = await step.run(write);
              onResult?.(result);
              setDone(result === KEPT ? "kept" : "sent");
            } catch {
              setDone("failed");
            }
          }}
        >
          Send
        </button>
        <div data-testid="footer-note">{step.note}</div>
        <div data-testid="result">{done}</div>
        <div data-testid="after">{step.after}</div>
      </PushedStep>
    </Modal>
  );
}

afterEach(() => {
  cleanup();
});

function refusedOnce(...lists: QueuedChange[][]) {
  const write = vi.fn();
  for (const list of lists) write.mockRejectedValueOnce(refusal(list));
  write.mockResolvedValue({ status: "applied" });
  return write;
}

describe("the step pushed inside the host sheet", () => {
  it("one removal: names the kind, the person today, who queued it and why; Keep it is first and focused", async () => {
    render(<Host write={refusedOnce([change()])} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    expect(await screen.findByRole("heading", { name: "Cancel removing Member from Rohan?" })).toBeInTheDocument();
    expect(screen.getByText("Give Rohan Mehta a role")).toBeInTheDocument(); // the chip
    expect(screen.getByText("It hasn't reached Zitadel, so Rohan still has Member today.")).toBeInTheDocument();
    expect(screen.getByText("Removing").closest("p")).toHaveTextContent("Removing Network storage · Member from Rohan Mehta");
    expect(screen.getByText(/Queued by Sam Iyer/)).toHaveTextContent("Queued by Sam Iyer · 2h ago");
    expect(screen.getByText("“left the course”")).toBeInTheDocument();
    expect(screen.getByText("Nothing changes until you choose.")).toBeInTheDocument();

    const keep = screen.getByRole("button", { name: "Keep it waiting" });
    const confirm = screen.getByRole("button", { name: "Cancel removing it and give Member" });
    expect(keep.compareDocumentPosition(confirm) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(keep).toHaveFocus();
  });

  it("a waiting grant is Giving ... to, and the person doesn't have it yet", async () => {
    render(<Host write={refusedOnce([change({ op_type: "add" })])} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByRole("heading", { name: "Cancel giving Member to Rohan?" })).toBeInTheDocument();
    expect(screen.getByText("Giving").closest("p")).toHaveTextContent("Giving Network storage · Member to Rohan Mehta");
    expect(screen.getByText("It hasn't reached Zitadel, so Rohan doesn't have Member yet.")).toBeInTheDocument();
  });

  it("Keep it puts the form back exactly as it was, says nothing changed, and never sends again", async () => {
    const write = refusedOnce([change()]);
    const onResult = vi.fn();
    render(<Host write={write} onResult={onResult} />);
    fireEvent.change(screen.getByLabelText("Reason"), { target: { value: "capstone" } });
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep it waiting" }));

    await waitFor(() => expect(onResult).toHaveBeenCalledWith(KEPT));
    expect(write).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument());
    expect(screen.getByLabelText("Reason")).toHaveValue("capstone");
    const note = screen.getByTestId("footer-note");
    expect(note).toHaveTextContent("Nothing changed. Sam's removal is still waiting.");
    expect(within(note).getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute("href", "/governance/pending?person=u1");
    expect(write).toHaveBeenCalledWith(undefined);
  });

  it("Escape is Keep it, and the host stays open", async () => {
    const onResult = vi.fn();
    render(<Host write={refusedOnce([change()])} onResult={onResult} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByRole("button", { name: "Keep it waiting" });
    fireEvent.keyDown(document.body, { key: "Escape" });
    await waitFor(() => expect(onResult).toHaveBeenCalledWith(KEPT));
    expect(screen.getByLabelText("Reason")).toBeInTheDocument();
  });

  it("going ahead sends the same write again naming exactly the ids shown, then says what was cancelled", async () => {
    const write = refusedOnce([change()]);
    render(<Host write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel removing it and give Member" }));

    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("sent"));
    expect(write).toHaveBeenCalledTimes(2);
    expect(write.mock.calls.map((c) => c[0])).toEqual([undefined, [idOf(1)]]);
    expect(screen.getByTestId("after")).toHaveTextContent(
      "The removal Sam queued 2h ago is cancelled, so nothing goes to Zitadel. Change history records both.",
    );
    await waitFor(() => expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument());
  });

  it("several queued together share one who-line and one quote, and the title and confirm carry the count", async () => {
    const same = (n: number) => change({ id: idOf(n), op_type: "add", role_keys: ["member"] });
    render(<Host write={refusedOnce([same(1), same(2), same(3)])} options={{ goAhead: "remove", tone: "danger" }} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByRole("heading", { name: "This cancels 3 changes that are waiting" })).toBeInTheDocument();
    expect(screen.getAllByText(/Queued by Sam Iyer/)).toHaveLength(1);
    expect(screen.getAllByText("“left the course”")).toHaveLength(1);
    expect(screen.getAllByText(/Giving/)).toHaveLength(3);
    expect(screen.getByRole("button", { name: "Cancel all 3 and remove" })).toBeInTheDocument();
    expect(screen.getByText(/None of it has reached Zitadel/)).toBeInTheDocument();
  });

  it("changes by different people or for different reasons are separate groups", async () => {
    render(
      <Host
        write={refusedOnce([
          change({ id: idOf(1) }),
          change({ id: idOf(2), initiated_by: "op-3", said: "duplicate" }),
        ])}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByRole("heading", { name: "This cancels 2 removals that are waiting" });
    expect(screen.getAllByText(/Queued by/)).toHaveLength(2);
    expect(screen.getByText("“duplicate”")).toBeInTheDocument();
  });

  it("past five it shows five, then the rest through the shared paging row", async () => {
    const many = Array.from({ length: 7 }, (_, i) => change({ id: idOf(i + 1), role_keys: [`r${i + 1}`] }));
    render(<Host write={refusedOnce(many)} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByRole("heading", { name: "This cancels 7 removals that are waiting" });
    expect(screen.getAllByText(/Removing/)).toHaveLength(5);
    expect(screen.getByText(/Showing 5 of 7/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Show 2 more" }));
    expect(screen.getAllByText(/Removing/)).toHaveLength(7);
    expect(screen.getByText("Showing all 7")).toBeInTheDocument();
  });

  it("no reason: says so in faint and tells you whom to ask", async () => {
    render(<Host write={refusedOnce([change({ said: undefined })])} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    expect(await screen.findByText("Sam gave no reason.")).toBeInTheDocument();
    expect(screen.getByText(/If you're unsure why Sam queued it, keep it and ask Sam\./)).toBeInTheDocument();
  });

  it("a longer refusal after going ahead: the new one is first, labelled, counted, and nothing was changed", async () => {
    const first = change({ id: idOf(1) });
    const added = change({ id: idOf(2), initiated_by: "op-3", initiated_by_name: "Lena Ortiz", said: "wrong person", queued_at: hoursAgo(0), new: true });
    const write = refusedOnce([first], [first, added]);
    render(<Host write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel removing it and give Member" }));

    expect(await screen.findByText(/One more since you looked\. Lena queued it just now, so it's added at the top and nothing was changed\./)).toBeInTheDocument();
    expect(screen.getByText("New since you looked")).toBeInTheDocument();
    const lines = screen.getAllByText(/Removing/);
    expect(lines).toHaveLength(2);
    expect(screen.getByText("New since you looked").closest("section")).toContainElement(screen.getByText("“wrong person”"));
    expect(screen.getByRole("heading", { name: "This cancels 2 removals that are waiting" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel all 2 and give Member" })).toBeInTheDocument();
    expect(write).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole("button", { name: "Cancel all 2 and give Member" }));
    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("sent"));
    expect(write.mock.calls.map((c) => c[0])).toEqual([undefined, [idOf(1)], [idOf(1), idOf(2)]]);
  });

  it("an error that is not the refusal passes through and closes the step", async () => {
    const write = vi.fn().mockRejectedValue(new Error("boom"));
    const onResult = vi.fn();
    render(<Host write={write} onResult={onResult} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("failed"));
    expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument();
    expect(onResult).not.toHaveBeenCalled();
  });

  it("the form fades first, then the step takes the sheet (the form stays mounted behind)", async () => {
    vi.useFakeTimers();
    motionInTests();
    vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined }));
    try {
      render(<Host write={refusedOnce([change()])} />);
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Send" }));
      });
      expect(screen.getByLabelText("Reason").closest("[data-step-form]")).not.toHaveAttribute("hidden");
      expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument();
      await act(async () => {
        vi.advanceTimersByTime(130);
      });
      expect(screen.getByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).toBeInTheDocument();
      expect(screen.getByLabelText("Reason", { selector: "input" }).closest("[data-step-form]")).toHaveAttribute("hidden");
    } finally {
      vi.unstubAllGlobals();
      vi.useRealTimers();
    }
  });
});

describe("the pushed step's height", () => {
  const rect = (open: boolean) => ({ open, view: open ? <p>the step</p> : null }) as never;

  it("ends on its own transition, not a child's, and lets go of the listener", async () => {
    vi.useFakeTimers();
    motionInTests();
    vi.stubGlobal("matchMedia", () => ({ matches: false, addEventListener: () => undefined, removeEventListener: () => undefined }));
    // jsdom has no layout: the form is 50 tall, the step 200.
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", { configurable: true, get: () => 50 });
    Object.defineProperty(HTMLElement.prototype, "scrollHeight", { configurable: true, get: () => 200 });
    try {
      const view = render(<PushedStep step={rect(false)}>form</PushedStep>);
      view.rerender(<PushedStep step={rect(true)}>form</PushedStep>);
      await act(async () => void vi.advanceTimersByTime(130));
      const box = view.container.firstElementChild as HTMLElement;
      expect(box.style.height).toBe("200px");

      // A fade inside the step finishing is not the height finishing.
      fireEvent.transitionEnd(screen.getByText("the step"));
      expect(box.style.height).toBe("200px");

      fireEvent.transitionEnd(box);
      expect(box.style.height).toBe("");
      expect(box.style.overflow).toBe("");
    } finally {
      // @ts-expect-error restoring jsdom's own (absent) layout
      delete HTMLElement.prototype.offsetHeight;
      // @ts-expect-error restoring jsdom's own (absent) layout
      delete HTMLElement.prototype.scrollHeight;
      vi.unstubAllGlobals();
      vi.useRealTimers();
    }
  });
});

describe("a write with no sheet of its own", () => {
  function Row({ write }: { write: (cancelIds?: string[]) => Promise<unknown> }) {
    const step = useCancelStep({ host: "Approve Priya's request", goAhead: "approve", standalone: true });
    const [done, setDone] = useState("");
    return (
      <>
        <button onClick={async () => setDone((await step.run(write)) === KEPT ? "kept" : "approved")}>Approve</button>
        <span data-testid="seat">{done}</span>
        {step.element}
      </>
    );
  }

  it("opens a small sheet holding only the step; Keep it leaves the request where it was", async () => {
    const write = refusedOnce([change({ op_type: "add" })]);
    render(<Row write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("heading", { name: "Cancel giving Member to Rohan?" })).toBeInTheDocument();
    expect(within(dialog).getByText("Approve Priya's request")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Cancel giving it and approve" })).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Keep it waiting" }));
    await waitFor(() => expect(screen.getByTestId("seat")).toHaveTextContent("kept"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(write).toHaveBeenCalledTimes(1);
  });

  it("going ahead approves and closes the sheet", async () => {
    render(<Row write={refusedOnce([change({ op_type: "add" })])} />);
    fireEvent.click(screen.getByRole("button", { name: "Approve" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel giving it and approve" }));
    await waitFor(() => expect(screen.getByTestId("seat")).toHaveTextContent("approved"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

// An act the planner planned (A-21 §3.1): the step is pushed at once from the
// plan's changes. No refused write is needed to learn what it cancels.
describe("a step pushed from the plan (confirm)", () => {
  function Planned({ changes, write }: { changes: QueuedChange[]; write: () => Promise<unknown> }) {
    const step = useCancelStep({ host: "Give Rohan Mehta a role", goAhead: "give Member" });
    const foretell = useForetell();
    const [done, setDone] = useState("");
    return (
      <Modal open onClose={() => undefined} size="md" labelledBy="host-title">
        <PushedStep step={step}>
          <h2 id="host-title">Give a role</h2>
          <p data-testid="before">{foretell("Giving it", changes)}</p>
          <button
            onClick={async () => setDone((await step.confirm(changes, write)) === KEPT ? "kept" : "sent")}
          >
            Send
          </button>
          <div data-testid="after">{step.after}</div>
          <div data-testid="result">{done}</div>
        </PushedStep>
      </Modal>
    );
  }

  it("shows the step before any write, and goes ahead with one write that follows the answer", async () => {
    const write = vi.fn().mockResolvedValue({ status: "applied" });
    render(<Planned changes={[change()]} write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));

    expect(await screen.findByRole("heading", { name: "Cancel removing Member from Rohan?" })).toBeInTheDocument();
    expect(write).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Cancel removing it and give Member" }));
    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("sent"));
    expect(write).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("after")).toHaveTextContent("cancelled, so nothing goes to Zitadel");
  });

  it("Keep it sends nothing at all", async () => {
    const write = vi.fn();
    render(<Planned changes={[change()]} write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Keep it waiting" }));
    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("kept"));
    expect(write).not.toHaveBeenCalled();
  });

  it("with nothing to cancel runs the write at once and pushes no step", async () => {
    const write = vi.fn().mockResolvedValue({ status: "applied" });
    render(<Planned changes={[]} write={write} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(screen.getByTestId("result")).toHaveTextContent("sent"));
    expect(write).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument();
    expect(screen.getByTestId("after")).toBeEmptyDOMElement();
  });

  it("a write that fails after the step is shown rethrows, and the step is put away", async () => {
    const write = vi.fn().mockRejectedValue(new Error("boom"));
    function Fails() {
      const step = useCancelStep({ host: "h", goAhead: "give" });
      const [done, setDone] = useState("");
      return (
        <Modal open onClose={() => undefined} size="md" labelledBy="t">
          <PushedStep step={step}>
            <h2 id="t">t</h2>
            <button onClick={() => step.confirm([change()], write).catch(() => setDone("failed"))}>Send</button>
            <span data-testid="r">{done}</span>
          </PushedStep>
        </Modal>
      );
    }
    render(<Fails />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel removing it and give" }));
    await waitFor(() => expect(screen.getByTestId("r")).toHaveTextContent("failed"));
    await waitFor(() => expect(screen.queryByRole("heading", { name: /^(This cancels|Cancel (giving|removing))/ })).not.toBeInTheDocument());
  });

  it("says the before-line for one removal, one give and several, with who queued it and when", () => {
    const { rerender } = render(<Planned changes={[change()]} write={vi.fn()} />);
    expect(screen.getByTestId("before")).toHaveTextContent("Giving it cancels removing Member from Rohan, which was waiting for you to send.");
    rerender(<Planned changes={[change({ op_type: "add" })]} write={vi.fn()} />);
    expect(screen.getByTestId("before")).toHaveTextContent("Giving it cancels giving Member to Rohan, which was waiting for you to send.");
    rerender(<Planned changes={[change({ id: idOf(1) }), change({ id: idOf(2) })]} write={vi.fn()} />);
    expect(screen.getByTestId("before")).toHaveTextContent("Giving it cancels 2 changes that are waiting.");
    rerender(<Planned changes={[]} write={vi.fn()} />);
    expect(screen.getByTestId("before")).toBeEmptyDOMElement();
  });

  it("the after line says change, not grant", async () => {
    render(<Planned changes={[change({ op_type: "add" })]} write={vi.fn().mockResolvedValue({})} />);
    fireEvent.click(screen.getByRole("button", { name: "Send" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancel giving it and give Member" }));
    await waitFor(() => expect(screen.getByTestId("after")).toHaveTextContent("The change Sam queued 2h ago is cancelled"));
  });
});
