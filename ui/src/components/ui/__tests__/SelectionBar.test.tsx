// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";

/**
 * Three claims this bar is in a position to get wrong, all of them about a
 * number that decides how many people lose access.
 */
describe("the selection bar over its ceiling", () => {
  function overflowing(onTakeCeiling = vi.fn()) {
    render(
      <SelectionBar
        count={640}
        noun={["person", "people"]}
        ceiling={500}
        onTakeCeiling={onTakeCeiling}
        onClear={() => {}}
      >
        <SelectionAction onClick={() => {}}>Rehearse removing a role</SelectionAction>
      </SelectionBar>,
    );
    return screen.getByRole("region", { name: "Selection" });
  }

  it("states the number selected and the number that can run, not one or the other", () => {
    const bar = overflowing();
    expect(within(bar).getByText(/640 people/)).toBeTruthy();
    expect(within(bar).getByText(/you can change at most 500 people at once/)).toBeTruthy();
  });

  // The two easy wrong answers: run the first 500 and report success for a
  // cohort nobody chose, or grey the bar out and leave the operator holding a
  // selection with nothing to do and no stated reason.
  it("withdraws the verbs and offers the one move that gets under the limit", () => {
    const bar = overflowing();
    expect(within(bar).queryByRole("button", { name: /Rehearse removing a role/ })).toBeNull();
    const narrow = within(bar).getByRole("button", {
      name: /Select the first 500 people in the order shown/,
    });
    expect(narrow).toBeEnabled();
  });

  it("says which 500, because an arbitrary 500 is not a cohort", () => {
    const bar = overflowing();
    expect(within(bar).getByText(/in the order shown/)).toBeTruthy();
  });

  it("narrows on demand rather than on its own", () => {
    const take = vi.fn();
    const bar = overflowing(take);
    expect(take).not.toHaveBeenCalled();
    fireEvent.click(within(bar).getByRole("button", { name: /Select the first 500/ }));
    expect(take).toHaveBeenCalledTimes(1);
  });

  it("leaves a selection under the ceiling entirely alone", () => {
    render(
      <SelectionBar
        count={9}
        noun={["person", "people"]}
        ceiling={500}
        onTakeCeiling={() => {}}
        onClear={() => {}}
      >
        <SelectionAction onClick={() => {}}>Rehearse removing a role</SelectionAction>
      </SelectionBar>,
    );
    const bar = screen.getByRole("region", { name: "Selection" });
    expect(within(bar).getByRole("button", { name: "Rehearse removing a role" })).toBeTruthy();
    expect(within(bar).queryByText(/at most 500/)).toBeNull();
  });
});

describe("select-all states both numbers", () => {
  it("says what it selects and how many exist beyond the filter", () => {
    render(
      <SelectAllRow
        inScope={12}
        total={340}
        noun={["person", "people"]}
        allSelected={false}
        checked={false}
        ref={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.getByText("Select these 12 people")).toBeTruthy();
    expect(screen.getByText(/340 people in all/)).toBeTruthy();
  });

  // Both numbers have to reach a screen reader, not just the eye — which is
  // why the wrapping label is the checkbox's accessible name and no aria-label
  // overrides it with the shorter half.
  it("announces the second number as part of the control's name", () => {
    render(
      <SelectAllRow
        inScope={12}
        total={340}
        noun={["person", "people"]}
        allSelected={false}
        checked={false}
        ref={() => {}}
        onChange={() => {}}
      />,
    );
    expect(
      screen.getByRole("checkbox", { name: /Select these 12 people.*340 people in all/ }),
    ).toBeTruthy();
  });

  // Once everything in scope is selected the label becomes "Clear the
  // selection", and a second number underneath it stops being an answer to
  // "these, as opposed to what?" and starts reading as a claim about what
  // clearing would do.
  it("drops the second number once the label stops asking the question", () => {
    render(
      <SelectAllRow
        inScope={12}
        total={340}
        noun={["person", "people"]}
        allSelected
        checked
        ref={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.getByText("Clear the selection")).toBeTruthy();
    expect(screen.queryByText(/in all/)).toBeNull();
  });

  it("says nothing about a wider set when the filter is not narrowing anything", () => {
    render(
      <SelectAllRow
        inScope={12}
        total={12}
        noun={["rule", "rules"]}
        allSelected={false}
        checked={false}
        ref={() => {}}
        onChange={() => {}}
      />,
    );
    expect(screen.queryByText(/in all/)).toBeNull();
  });
});

describe("the mode control", () => {
  it("is the same control leaving as entering, and says which state it is in", () => {
    const toggle = vi.fn();
    const { rerender } = render(<SelectModeToggle active={false} onToggle={toggle} />);
    const button = screen.getByRole("button", { name: "Select" });
    expect(button.getAttribute("aria-pressed")).toBe("false");

    fireEvent.click(button);
    expect(toggle).toHaveBeenCalledTimes(1);

    rerender(<SelectModeToggle active onToggle={toggle} />);
    expect(screen.getByRole("button", { name: "Done selecting" })).toBeTruthy();
  });
});

/**
 * The one surface where the counted unit and the capped unit differ.
 *
 * Expiring access selects grants; `services.BulkMaxUsers` caps distinct
 * people. Gating on the selection count there is wrong in both directions —
 * 600 grants held by 300 people is legal, and 500 grants held by 500 people is
 * already at the limit — so the bar has to be told what the ceiling counts.
 */
describe("a ceiling that counts something other than the selection", () => {
  function bar(count: number, ceilingCount: number) {
    render(
      <SelectionBar
        count={count}
        noun={["grant", "grants"]}
        ceiling={500}
        ceilingCount={ceilingCount}
        ceilingNoun={["person", "people"]}
        onTakeCeiling={() => {}}
        onClear={() => {}}
      >
        <SelectionAction onClick={() => {}}>Rehearse an extension</SelectionAction>
      </SelectionBar>,
    );
    return screen.getByRole("region", { name: "Selection" });
  }

  it("lets a selection past the ceiling through when it covers few enough people", () => {
    const region = bar(600, 300);
    expect(within(region).queryByText(/you can change at most/)).toBeNull();
    expect(within(region).getByRole("button", { name: "Rehearse an extension" })).toBeTruthy();
  });

  it("refuses on the capped unit, and says which number it is refusing on", () => {
    const region = bar(540, 520);
    expect(
      within(region).getByText(/they cover 520 people, and you can change at most 500 people at once/),
    ).toBeTruthy();
    // The selection count is still stated: the operator ticked grants and has
    // to recognise what they ticked.
    expect(within(region).getByText(/540 grants/)).toBeTruthy();
  });

  it("names the unit in the narrowing move, so the cohort is not misread as rows", () => {
    const region = bar(540, 520);
    expect(
      within(region).getByRole("button", { name: "Select the first 500 people in the order shown" }),
    ).toBeTruthy();
  });

  // Every other surface counts what the endpoint counts, and must keep the
  // shorter sentence rather than growing a clause about itself.
  it("says nothing about coverage when the two units are the same", () => {
    render(
      <SelectionBar count={640} noun={["person", "people"]} ceiling={500} onClear={() => {}}>
        <SelectionAction onClick={() => {}}>Rehearse</SelectionAction>
      </SelectionBar>,
    );
    const region = screen.getByRole("region", { name: "Selection" });
    expect(within(region).queryByText(/they cover/)).toBeNull();
    expect(within(region).getByText(/you can change at most 500 people at once/)).toBeTruthy();
  });
});
