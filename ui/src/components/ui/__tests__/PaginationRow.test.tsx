// @vitest-environment jsdom
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ArriveList } from "@/components/ui/ArriveList";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { motionInTests } from "@/test-utils/motion";

const noop = () => {};
const row = (props: Partial<React.ComponentProps<typeof PaginationRow>>) =>
  render(<PaginationRow shown={10} onMore={noop} onAll={noop} onBack={noop} {...props} />);

describe("one pagination row (A-04 section 7.4)", () => {
  it("with a known total: Showing 10 of N, Show 25 more, Show all N", () => {
    row({ shown: 10, total: 61 });
    expect(screen.getByText("Showing 10 of 61")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 25 more" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show all 61" })).toBeInTheDocument();
    // Nothing to go back from while only the first ten are showing.
    expect(screen.queryByRole("button", { name: /Back to/ })).toBeNull();
  });

  it("offers Back to 10 once more than ten are showing", () => {
    row({ shown: 35, total: 61 });
    expect(screen.getByText("Showing 35 of 61")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back to 10" })).toBeInTheDocument();
  });

  it("never promises 25 more when fewer remain, and drops Show all when it would equal Show more", () => {
    row({ shown: 50, total: 61 });
    expect(screen.getByRole("button", { name: "Show 11 more" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show all/ })).toBeNull();
  });

  it("without a total: Showing 35, Show 25 more, Back to 10, and no Show all", () => {
    row({ shown: 35, hasMore: true });
    expect(screen.getByText("Showing 35")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 25 more" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Back to 10" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show all/ })).toBeNull();
  });

  it("at the end of a server-fetched list it says so, with the date it reaches back to", () => {
    row({ shown: 61, hasMore: false, since: "2 Sept" });
    expect(screen.getByText("That's everything since 2 Sept")).toBeInTheDocument();
    expect(screen.getByText("Showing all 61")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Show/ })).toBeNull();
  });

  it("at the end with no date to give, it still states the end", () => {
    row({ shown: 3, hasMore: false });
    expect(screen.getByText("That's everything")).toBeInTheDocument();
    expect(screen.getByText("Showing all 3")).toBeInTheDocument();
  });

  it("a list that fits the first page draws nothing", () => {
    const { container } = row({ shown: 8, total: 8 });
    expect(container).toBeEmptyDOMElement();
  });

  it("Show 25 more shows its own busy state while fetching", () => {
    row({ shown: 35, hasMore: true, pending: true });
    const more = screen.getByRole("button", { name: "Show 25 more" });
    expect(more).toBeDisabled();
    expect(more).toHaveAttribute("aria-busy", "true");
  });

  it("Back to 10 trims the list and returns focus to the heading", () => {
    const onBack = vi.fn();
    const heading = document.createElement("h2");
    heading.textContent = "People";
    document.body.appendChild(heading);
    const anchor = createRef<HTMLElement>();
    (anchor as { current: HTMLElement | null }).current = heading;
    row({ shown: 35, total: 61, onBack, anchor });
    fireEvent.click(screen.getByRole("button", { name: "Back to 10" }));
    expect(onBack).toHaveBeenCalledOnce();
    expect(document.activeElement).toBe(heading);
    heading.remove();
  });

  it("names the first page in Back to, whatever it is", () => {
    row({ shown: 30, total: 61, initial: 5 });
    expect(screen.getByRole("button", { name: "Back to 5" })).toBeInTheDocument();
  });
});

/** Motion 8c: Show more arrives only the new rows; Back to 10 glides first, then trims. */
describe("PaginationRow motion", () => {
  afterEach(() => {
    vi.useRealTimers();
    document.querySelectorAll("h2").forEach((h) => h.remove());
  });

  it("Back to 10 glides to the heading before the rows leave, and trims out of sight", () => {
    vi.useFakeTimers();
    motionInTests();
    const heading = document.createElement("h2");
    document.body.appendChild(heading);
    const scrollIntoView = vi.fn();
    heading.scrollIntoView = scrollIntoView;
    const onBack = vi.fn();
    row({ shown: 35, total: 61, onBack, anchor: { current: heading } });

    fireEvent.click(screen.getByRole("button", { name: "Back to 10" }));
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "start" });
    expect(onBack).not.toHaveBeenCalled();

    act(() => void vi.advanceTimersByTime(519));
    expect(onBack).not.toHaveBeenCalled();
    act(() => void vi.advanceTimersByTime(2));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("Show 25 more makes only the rows it adds arrive", async () => {
    const rows = (n: number) => (
      <>
        <ArriveList startSettled>
          {Array.from({ length: n }, (_, i) => (
            <p key={i}>row {i}</p>
          ))}
        </ArriveList>
        <PaginationRow shown={n} total={61} onMore={noop} onBack={noop} />
      </>
    );
    const { container, rerender } = render(rows(10));
    fireEvent.click(screen.getByRole("button", { name: "Show 25 more" }));
    rerender(rows(12));
    await act(async () => {});
    const arriving = Array.from(container.querySelectorAll("[data-arriving]")).map((el) => el.textContent);
    expect(arriving).toEqual(["row 10", "row 11"]);
  });
});

describe("usePaging", () => {
  it("starts at ten, grows by 25, shows all, and goes back", () => {
    const { result } = renderHook(() => usePaging());
    expect(result.current.limit).toBe(10);
    act(() => result.current.more());
    expect(result.current.limit).toBe(35);
    act(() => result.current.all(61));
    expect(result.current.limit).toBe(61);
    act(() => result.current.back());
    expect(result.current.limit).toBe(10);
  });
});
