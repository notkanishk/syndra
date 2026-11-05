// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { usePaging } from "@/components/ui/PaginationRow";
import { useSort } from "@/components/ui/SortHead";
import { usePill, usePlace, usePlaceScroll } from "@/lib/place";

vi.mock("@/lib/motion", async (orig) => ({ ...(await orig<typeof import("@/lib/motion")>()), stillMotion: () => true }));

const back = () => act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
// A visit later than the five seconds a Back is remembered for.
const later = () => vi.setSystemTime(Date.now() + 10_000);

function Fold() {
  const paging = usePaging();
  return (
    <button onClick={paging.more}>
      limit {paging.limit}
    </button>
  );
}

function Opened() {
  const [open, setOpen] = usePlace("open", false);
  return <button onClick={() => setOpen(!open)}>{open ? "open" : "shut"}</button>;
}

const KEYS = ["name", "people"] as const;
function Sorted() {
  const [sort, flip] = useSort<(typeof KEYS)[number]>("name", { list: "t", keys: KEYS });
  const [query, setQuery] = usePlace("never-persisted-as-view", "");
  const [pill, setPill] = usePill("t", "14", ["14", "30"] as const);
  return (
    <>
      <button onClick={() => flip("people")}>sort {sort.key} {sort.dir}</button>
      <button onClick={() => setPill("30")}>pill {pill}</button>
      <input aria-label="search" value={query} onChange={(e) => setQuery(e.target.value)} />
    </>
  );
}

// The clock only moves forward across tests, so one test's Back is never another's.
let clock = new Date("2026-11-05T10:00:00Z").getTime();
beforeEach(() => {
  clock += 60_000;
  vi.useFakeTimers({ now: clock });
  window.sessionStorage.clear();
  window.localStorage.clear();
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe("Back keeps your place (P5)", () => {
  it("returns the fold and an open section after Back", () => {
    later(); // nothing from earlier tests counts as a return
    const first = render(
      <>
        <Fold />
        <Opened />
      </>,
    );
    fireEvent.click(screen.getByText("limit 10"));
    fireEvent.click(screen.getByText("shut"));
    expect(screen.getByText("limit 35")).toBeTruthy();
    first.unmount();

    back();
    render(
      <>
        <Fold />
        <Opened />
      </>,
    );
    expect(screen.getByText("limit 35")).toBeTruthy();
    expect(screen.getByText("open")).toBeTruthy();
  });

  it("a fresh visit starts again, and a Back after it returns the fresh state", () => {
    later();
    const first = render(<Fold />);
    fireEvent.click(screen.getByText("limit 10"));
    first.unmount();

    later(); // no popstate: this is a link followed, not a Back
    const second = render(<Fold />);
    expect(screen.getByText("limit 10")).toBeTruthy();
    second.unmount();

    back();
    render(<Fold />);
    expect(screen.getByText("limit 10")).toBeTruthy();
  });

  it("is keyed by the route: another address keeps nothing of this one", () => {
    later();
    const first = render(<Fold />);
    fireEvent.click(screen.getByText("limit 10"));
    first.unmount();
    window.history.pushState({}, "", "/elsewhere?x=1");
    back();
    render(<Fold />);
    expect(screen.getByText("limit 10")).toBeTruthy();
    window.history.pushState({}, "", "/");
  });

  it("never writes place to localStorage", () => {
    later();
    render(<Fold />);
    fireEvent.click(screen.getByText("limit 10"));
    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBeGreaterThan(0);
  });

  it("restores scroll after Back and not on a fresh visit", () => {
    later();
    const box = document.createElement("div");
    box.id = "app-scroll";
    document.body.append(box);
    function Page() {
      usePlaceScroll("/");
      return null;
    }
    const first = render(<Page />);
    box.scrollTop = 480;
    fireEvent.scroll(box);
    act(() => void vi.advanceTimersByTime(50));
    first.unmount();

    box.scrollTop = 0;
    later();
    const fresh = render(<Page />);
    act(() => void vi.advanceTimersByTime(1500));
    expect(box.scrollTop).toBe(0);
    fresh.unmount();

    // The fresh visit wrote the top; give the Back something to return to.
    box.scrollTop = 480;
    fireEvent.scroll(box);
    const again = render(<Page />);
    box.scrollTop = 480;
    fireEvent.scroll(box);
    act(() => void vi.advanceTimersByTime(50));
    again.unmount();

    box.scrollTop = 0;
    back();
    render(<Page />);
    act(() => void vi.advanceTimersByTime(1500));
    expect(box.scrollTop).toBe(480);
    box.remove();
  });
});

describe("Your last view (P8)", () => {
  it("keeps sort and pill across visits, never the search text", () => {
    later();
    const first = render(<Sorted />);
    fireEvent.click(screen.getByText("sort name 1"));
    fireEvent.click(screen.getByText("pill 14"));
    fireEvent.change(screen.getByLabelText("search"), { target: { value: "priya" } });
    first.unmount();

    later(); // a visit, not a Back
    render(<Sorted />);
    expect(screen.getByText("sort people 1")).toBeTruthy();
    expect(screen.getByText("pill 30")).toBeTruthy();
    expect((screen.getByLabelText("search") as HTMLInputElement).value).toBe("");
    expect(window.localStorage.getItem("syndra:view:t")).not.toContain("priya");
    expect(window.localStorage.length).toBe(1);
  });

  it("does not remember the starting order, only a press", () => {
    later();
    render(<Sorted />);
    expect(window.localStorage.getItem("syndra:view:t")).toBeNull();
  });

  it("ignores a remembered sort the list no longer has", () => {
    window.localStorage.setItem("syndra:view:t", JSON.stringify({ sort: { key: "gone", dir: -1 }, pill: "7" }));
    render(<Sorted />);
    expect(screen.getByText("sort name 1")).toBeTruthy();
    expect(screen.getByText("pill 14")).toBeTruthy();
  });
});

describe("storage that fails is harmless", () => {
  it("reads and writes that throw leave the page working", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("full");
    });
    render(
      <>
        <Sorted />
        <Fold />
      </>,
    );
    fireEvent.click(screen.getByText("sort name 1"));
    fireEvent.click(screen.getByText("pill 14"));
    fireEvent.click(screen.getByText("limit 10"));
    expect(screen.getByText("sort people 1")).toBeTruthy();
    expect(screen.getByText("pill 30")).toBeTruthy();
    expect(screen.getByText("limit 35")).toBeTruthy();
  });

  it("an unreadable stored value is ignored", () => {
    window.localStorage.setItem("syndra:view:t", "{not json");
    render(<Sorted />);
    expect(screen.getByText("pill 14")).toBeTruthy();
  });
});
