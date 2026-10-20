// @vitest-environment jsdom
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ReadLine, SectionPill, type NavSection } from "@/components/ui/SectionNav";

/**
 * The read line, its chips and the floating pill, through the one interface a
 * page uses: the sections it draws and what to do when one is asked for. The
 * positions are read from the DOM by id, so the headings are drawn here and
 * their places are set by hand.
 */
const SECTIONS: NavSection[] = [
  { id: "sec-a", label: "Undecided", count: 4, tone: "warn" },
  { id: "sec-b", label: "Let lapse", count: 2 },
];

function Page({ sections = SECTIONS, onOpen }: { sections?: NavSection[]; onOpen?: (s: NavSection) => void }) {
  return (
    <div>
      <ReadLine sections={sections} label="Sections of things" onOpen={onOpen}>
        <span>read just now</span>
      </ReadLine>
      <SectionPill sections={sections} label="Sections of things" onOpen={onOpen} />
      {sections.map((section) => (
        <section key={section.id} id={section.id}>
          {section.label} heading
        </section>
      ))}
    </div>
  );
}

const rect = (top: number, bottom = top) => ({ top, bottom }) as DOMRect;

/** Where things sit on screen: the chips' line and each heading, from the top of the window. */
function place(chipsBottom: number, tops: Record<string, number>) {
  const chips = document.getElementById("section-chips")!;
  chips.getBoundingClientRect = () => rect(chipsBottom - 30, chipsBottom);
  for (const [id, top] of Object.entries(tops)) {
    document.getElementById(id)!.getBoundingClientRect = () => rect(top);
  }
  act(() => {
    document.dispatchEvent(new Event("scroll"));
  });
}

// By label rather than by role: while it is away the pill is aria-hidden, which is the point.
const pill = () => document.querySelector<HTMLElement>('nav[aria-label="Sections of things (while scrolling)"]')!;

beforeEach(() => {
  window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
});
afterEach(() => vi.useRealTimers());

describe("the read line", () => {
  it("holds how current the list is, and a chip per section with its count", () => {
    render(<Page />);
    expect(screen.getByText("read just now")).toBeInTheDocument();
    const chips = screen.getByRole("navigation", { name: "Sections of things" });
    expect(within(chips).getByRole("link", { name: /Undecided\s*4/ })).toBeInTheDocument();
    expect(within(chips).getByRole("link", { name: /Let lapse\s*2/ })).toBeInTheDocument();
  });

  it("draws a dot only for a section that has a tone", () => {
    render(<Page />);
    const chips = screen.getByRole("navigation", { name: "Sections of things" });
    expect(within(chips).getByRole("link", { name: /Undecided/ }).querySelector(".bg-warn")).not.toBeNull();
    expect(within(chips).getByRole("link", { name: /Let lapse/ }).querySelector("[class*='bg-']:not(.bg-tint-2)")).toBeNull();
  });

  it("draws no chips for a single section, and the pill goes with them", () => {
    render(<Page sections={SECTIONS.slice(0, 1)} />);
    expect(screen.getByText("read just now")).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Sections of things" })).toBeNull();
    expect(document.querySelector("nav[aria-label*=\"while scrolling\"]")).toBeNull();
  });

  it("asks for the section to be opened, then glides to it", () => {
    vi.useFakeTimers();
    const onOpen = vi.fn();
    render(<Page onOpen={onOpen} />);
    document.getElementById("sec-b")!.getBoundingClientRect = () => rect(900);

    fireEvent.click(screen.getByRole("link", { name: /Let lapse/ }));
    expect(onOpen).toHaveBeenCalledWith(SECTIONS[1]);
    expect(window.scrollTo).not.toHaveBeenCalled();

    act(() => {
      vi.runAllTimers();
    });
    // 900 down, less the 64 the heading is left clear of the top by.
    expect(window.scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 836 }));
  });
});

describe("the floating pill", () => {
  it("stays out of sight and out of the tab order while the read line is on screen", () => {
    render(<Page />);
    place(80, { "sec-a": 200, "sec-b": 900 });
    expect(pill()).toHaveAttribute("aria-hidden", "true");
    expect(Array.from(pill().querySelectorAll("a")).every((a) => a.tabIndex === -1)).toBe(true);
  });

  it("appears once the read line has scrolled away, with the section you are in filled", () => {
    render(<Page />);
    place(-40, { "sec-a": 10, "sec-b": 900 });
    expect(pill()).toHaveAttribute("aria-hidden", "false");
    const links = within(pill()).getAllByRole("link");
    expect(links[0]).toHaveAttribute("aria-current", "location");
    expect(links[1]).not.toHaveAttribute("aria-current");
    // The one you are in points nowhere; the other points down to where it is.
    expect(links[1]).toHaveTextContent("↓");
    expect(links[0]).not.toHaveTextContent(/[↑↓]/);
  });

  it("moves the fill, and turns the arrows, as the reader passes into the next section", () => {
    render(<Page />);
    place(-40, { "sec-a": -600, "sec-b": 50 });
    const links = within(pill()).getAllByRole("link");
    expect(links[1]).toHaveAttribute("aria-current", "location");
    expect(links[0]).toHaveTextContent("↑");
  });

  it("opens a folded section from the pill, as the chips do", () => {
    const onOpen = vi.fn();
    render(<Page onOpen={onOpen} />);
    place(-40, { "sec-a": 10, "sec-b": 900 });
    fireEvent.click(within(pill()).getByRole("link", { name: /Let lapse/ }));
    expect(onOpen).toHaveBeenCalledWith(SECTIONS[1]);
  });
});
