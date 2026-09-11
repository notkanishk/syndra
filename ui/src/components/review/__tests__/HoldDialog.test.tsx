// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { HoldDialog } from "@/components/review/HoldDialog";

const createHold = vi.hoisted(() => vi.fn());

vi.mock("@/lib/queries/useHolds", () => ({
  useCreateHold: () => ({ mutateAsync: createHold, isPending: false }),
}));

function open() {
  return render(
    <HoldDialog
      subjectId="u_2f81"
      subjectName="Tomas Beck"
      target="truenas"
      field="share"
      value="prints"
      label="the prints share"
      onClose={() => {}}
    />,
  );
}

describe("HoldDialog", () => {
  it("confirms the hold and says where it shows up next, without going quiet on success", async () => {
    createHold.mockResolvedValueOnce({});
    open();

    fireEvent.change(screen.getByLabelText(/Why — shown to Tomas Beck/i), {
      target: { value: "Per the safety review" },
    });
    fireEvent.change(screen.getByLabelText(/Remind us on/i), {
      target: { value: "2026-10-03" },
    });
    fireEvent.click(screen.getByText("Hold the prints share"));

    expect(await screen.findByText(/Hold placed on the prints share for Tomas Beck/)).toBeInTheDocument();
    // Default ending is "stays", so the reminder actually lands somewhere: the
    // queue where it will next surface, not just "the target catches up". The
    // radio option above also mentions Holds due, so match the fuller outcome
    // sentence to land on the result rather than the form.
    expect(screen.getByText(/appears under Review › Holds due on the date you set/)).toBeInTheDocument();
    expect(screen.getByText(/TrueNAS catches up on its next pass/)).toBeInTheDocument();
    // Once it has succeeded there is nothing left to re-run — the action
    // button is gone and Cancel has become the closing word.
    expect(screen.queryByText("Hold the prints share")).toBeNull();
    expect(screen.getByText("Done")).toBeInTheDocument();
  });

  it("says the hold lifts itself instead of pointing at Holds due, when that's the form chosen", async () => {
    createHold.mockResolvedValueOnce({});
    open();

    fireEvent.change(screen.getByLabelText(/Why — shown to Tomas Beck/i), {
      target: { value: "Until the cohort ends" },
    });
    fireEvent.click(screen.getByText("It lifts itself on a date."));
    fireEvent.change(screen.getByLabelText(/Lifts on/i), {
      target: { value: "2026-10-03" },
    });
    fireEvent.click(screen.getByText("Hold the prints share"));

    expect(await screen.findByText(/lifts itself on the date you set/)).toBeInTheDocument();
    // The two forms carry different next steps — a lifting hold has no review
    // reminder, so the RESULT must not claim the other form's queue. (The
    // "stays" radio option above still mentions Holds due on its own — that's
    // the form, not the outcome, so it is out of scope here.)
    const outcome = screen.getByRole("status");
    expect(within(outcome).queryByText(/Holds due/)).toBeNull();
  });
});
