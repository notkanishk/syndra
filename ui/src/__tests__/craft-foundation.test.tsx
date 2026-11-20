// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { Count } from "@/components/ui/Badge";
import { StatCard } from "@/components/ui/StatCard";
import { Relative } from "@/components/ui/Time";
import { GoTo, GoToPanel } from "@/components/ui/GoTo";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Term } from "@/components/ui/Term";
import { LATELY_SHOWN, goTo, keepLately, recallLately } from "@/lib/go-to";
import { ArriveList, Crossfade, TINT_MS, TIP_WARM_MS, notePage } from "@/lib/motion";
import { ADVANCED_NAV, BASIC_NAV } from "@/lib/nav";
import { NAME_CATALOG_KEY } from "@/lib/queries/useNameCatalog";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/**
 * A-24 wave 1, the foundation (§11, §2, §8, Q14, Q15): the small things a
 * person feels and cannot name. Each guard here fails if its rule is undone.
 */

vi.mock("@/lib/ui-view", () => ({
  useUiView: () => ({ audience: "advanced", view: "advanced", isOperator: true, setView: () => {}, revealInAdvanced: () => {} }),
  useIsAdvanced: () => true,
}));

const SRC = resolve(__dirname, "..");
const css = readFileSync(resolve(SRC, "app/globals.css"), "utf8");

function body(source: string, prelude: string): string {
  const start = source.indexOf(prelude);
  if (start < 0) return "";
  const open = source.indexOf("{", start);
  let depth = 0;
  for (let i = open; i < source.length; i += 1) {
    if (source[i] === "{") depth += 1;
    else if (source[i] === "}" && --depth === 0) return source.slice(open + 1, i);
  }
  return "";
}

function* sources(dir: string): Generator<string> {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      if (entry !== "__tests__" && entry !== "node_modules") yield* sources(path);
    } else if (/\.tsx?$/.test(entry)) yield path;
  }
}

const go = (path: string) => {
  window.history.pushState({}, "", path);
  notePage(path);
};

describe("a place arrives on every visit, and Back is still (S11; Q14 reversed by the owner, 19 Nov 2026)", () => {
  const head = () => screen.getByRole("heading", { level: 1 }).parentElement!.parentElement!.parentElement!;

  it("arrives on a second visit as on the first: the header tints in and what follows rises", () => {
    go("/a24/people");
    render(<PageHeader title="People" />);
    expect(head()).not.toHaveAttribute("data-settled");
    cleanup();
    go("/a24/home");
    go("/a24/people");
    render(<PageHeader title="People" />);
    expect(head()).not.toHaveAttribute("data-settled");
    cleanup();
    const { container } = render(
      <ArriveList>
        <div>One</div>
      </ArriveList>,
    );
    expect(container.querySelector(".arrive-list")).not.toHaveAttribute("data-settled");
  });

  it("arrives when reached by key too: Enter on a link is a visit like any other", () => {
    render(
      <a href="/a24/by-key" onClick={(event) => event.preventDefault()}>
        By key
      </a>,
    );
    fireEvent.click(screen.getByText("By key"), { detail: 0 });
    cleanup();
    go("/a24/by-key");
    render(<PageHeader title="By key" />);
    expect(head()).not.toHaveAttribute("data-settled");
  });

  it("is still on Back: the page is simply there", () => {
    go("/a24/back");
    window.dispatchEvent(new PopStateEvent("popstate"));
    render(<PageHeader title="Back" />);
    expect(head()).toHaveAttribute("data-settled");
  });

  it("keeps no memory of where it has arrived, and nothing marks a visit by key", () => {
    const arrive = readFileSync(resolve(SRC, "lib/motion/arrive.tsx"), "utf8");
    expect(arrive).not.toMatch(/sessionStorage|recallPaths|syndra:arrived|visitByKey|detail === 0/);
    // The page head has one way in, and no quiet one.
    expect(body(css, "@utility page-head")).not.toContain("data-quiet");
  });
});

describe("Back gives you your row (§11.4, §2 warm)", () => {
  it("focuses the row you opened, without scrolling, and flashes it once", () => {
    window.history.pushState({}, "", "/a24/rows");
    const list = (
      <ArriveList>
        <a data-row href="/a24/rows/one" onClick={(event) => event.preventDefault()}>
          One
        </a>
        <a data-row href="/a24/rows/two" onClick={(event) => event.preventDefault()}>
          Two
        </a>
      </ArriveList>
    );
    const first = render(list);
    fireEvent.click(screen.getByText("Two"));
    first.unmount();

    // Back: the browser says so with popstate.
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    render(list);
    const two = screen.getByText("Two");
    expect(two).toHaveFocus();
    expect(two).toHaveClass("flash");
    expect(two).toHaveAttribute("data-row-current");
    expect(screen.getByText("One")).not.toHaveClass("flash");

    // Once: a reload after that is not another return to the row.
    cleanup();
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    render(list);
    expect(screen.getByText("Two")).not.toHaveClass("flash");
  });

  it("remembers only a plain primary click: a modified or middle click opened the row elsewhere", () => {
    window.history.pushState({}, "", "/a24/rows-elsewhere");
    const list = (
      <ArriveList>
        <a data-row href="/a24/rows-elsewhere/one" onClick={(event) => event.preventDefault()}>
          One
        </a>
      </ArriveList>
    );
    for (const click of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }]) {
      const opened = render(list);
      fireEvent.click(screen.getByText("One"), click);
      opened.unmount();
      // A reload of this page is a return to it: nothing was opened here, so nothing flashes.
      act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
      const back = render(list);
      expect(screen.getByText("One"), JSON.stringify(click)).not.toHaveClass("flash");
      back.unmount();
    }
  });

  it("never gives back a row from an older visit: a fresh visit forgets it", () => {
    window.history.pushState({}, "", "/a24/rows-fresh");
    const list = (
      <ArriveList>
        <a data-row href="/a24/rows-fresh/one" onClick={(event) => event.preventDefault()}>
          One
        </a>
      </ArriveList>
    );
    const opened = render(list);
    fireEvent.click(screen.getByText("One"));
    opened.unmount();
    // A later visit by a link, where nothing is opened, then Back to it.
    const fresh = render(list);
    expect(screen.getByText("One")).not.toHaveClass("flash");
    fresh.unmount();
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    render(list);
    expect(screen.getByText("One")).not.toHaveClass("flash");
  });
});

function renderGoTo() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(NAME_CATALOG_KEY, {
    users: [{ id: "u1", name: "Becky Lynch", email: "becky@example.org", title: "Student", team: "UG 2023" }],
    projects: [{ id: "p1", name: "Laser cutting", kind: "", description: "", roles: [] }],
    applications: [],
  });
  client.setQueryData(["name-bundles"], []);
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <GoTo open onClose={onClose} nav={ADVANCED_NAV} />
    </QueryClientProvider>,
  );
  return onClose;
}

describe("Go to appears at once and leaves at once (§8)", () => {
  it("has no scrim fade and no rise, and leaves no picture behind", () => {
    motionInTests();
    const client = new QueryClient();
    client.setQueryData(NAME_CATALOG_KEY, { users: [], projects: [], applications: [] });
    function Host() {
      const [open, setOpen] = useState(true);
      return (
        <QueryClientProvider client={client}>
          <GoTo open={open} onClose={() => setOpen(false)} nav={BASIC_NAV} />
        </QueryClientProvider>
      );
    }
    render(<Host />);
    const dialog = screen.getByRole("dialog", { name: "Go to" });
    expect(dialog).not.toHaveClass("settle-scrim");
    expect(dialog.firstElementChild).not.toHaveClass("sheet-in");

    fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(document.querySelector(".leave-scrim")).toBeNull();
  });
});

describe("Escape in two stages, in every search (§11.5)", () => {
  it("in Go to: clears what is typed and stays open, then closes", () => {
    const onClose = renderGoTo();
    const field = screen.getByRole("combobox");
    fireEvent.change(field, { target: { value: "bec" } });
    expect(screen.getByText("clear, then close")).toBeInTheDocument();

    fireEvent.keyDown(field, { key: "Escape" });
    expect(field).toHaveValue("");
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText("close")).toBeInTheDocument();

    fireEvent.keyDown(field, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("a dialog never closes on an Escape already spent, wherever its listener sits", () => {
    // In the app the React root is the document, beside the dialog's own listener, so only
    // defaultPrevented can tell it the search used this Escape.
    const onClose = renderGoTo();
    const spent = new KeyboardEvent("keydown", { key: "Escape", cancelable: true, bubbles: true });
    spent.preventDefault();
    document.dispatchEvent(spent);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("in a list's search: the first Escape clears through the caller's own onChange and goes no further", () => {
    const outside = vi.fn();
    function Search() {
      const [query, setQuery] = useState("rey");
      return <Input type="search" aria-label="Search people" value={query} onChange={(event) => setQuery(event.target.value)} />;
    }
    render(
      <div onKeyDown={(event) => outside(event.key)}>
        <Search />
      </div>,
    );
    const field = screen.getByRole("searchbox", { name: "Search people" });
    fireEvent.keyDown(field, { key: "Escape" });
    expect(field).toHaveValue("");
    expect(outside).not.toHaveBeenCalled();

    fireEvent.keyDown(field, { key: "Escape" });
    expect(outside).toHaveBeenCalledWith("Escape");
  });

  it("every search field is a search, so every one gets the two stages", () => {
    const plain: string[] = [];
    for (const file of sources(SRC)) {
      const source = readFileSync(file, "utf8");
      // Each <Input …/> up to its own close: an arrow in onChange holds a ">", so no regex stops at one.
      for (const chunk of source.split("<Input").slice(1)) {
        if (!/^\s/.test(chunk)) continue;
        const tag = chunk.slice(0, chunk.indexOf("/>"));
        if (/(?:placeholder|aria-label)=\{?["'`]Search/.test(tag) && !tag.includes('type="search"')) {
          plain.push(`${file.replace(SRC, "src")}: ${tag.trim().slice(0, 80)}`);
        }
      }
    }
    expect(plain, `A search field without type="search" keeps a one-stage Escape:\n${plain.join("\n")}`).toEqual([]);
  });
});

describe("Lately in Go to (§8 warm, Q13)", () => {
  const source = (lately: string[], here?: string) => ({
    nav: ADVANCED_NAV,
    names: true,
    people: [{ id: "u1", name: "Becky Lynch", email: "becky@example.org" }] as never,
    projects: [{ id: "p1", name: "Laser cutting", roles: [] }] as never,
    lately,
    here,
  });

  it("lists the places opened, most recent first, above Pages, and never the place you are in", () => {
    const groups = goTo(source(["/users/u1", "/governance/pending", "/projects/p1", "/users"], "/users"), "");
    expect(groups.map((g) => g.group)).toEqual(["Lately", "Pages"]);
    expect(groups[0].items.map((i) => i.name)).toEqual(["Becky Lynch", "Pending changes", "Laser cutting"]);
    expect(groups[0].items.map((i) => i.kind)).toEqual(["Person", "Page", "Project"]);
    // Its keys are its own, so a place in Lately and in Pages is never selected twice.
    const keys = groups.flatMap((g) => g.items.map((i) => i.key));
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("drops a place this session cannot open, and shows five at most", () => {
    const pages = ADVANCED_NAV.flatMap((entry) => (entry.kind === "leaf" ? [entry.href] : entry.children.map((c) => c.href)));
    const groups = goTo(source(["/somewhere/gone", ...pages]), "");
    expect(groups[0].items).toHaveLength(LATELY_SHOWN);
    expect(groups[0].items.map((i) => i.href)).toEqual(pages.slice(0, LATELY_SHOWN));
  });

  it("is gone the moment something is typed: the groups are A to Z again", () => {
    expect(goTo(source(["/users/u1"]), "becky").map((g) => g.group)).toEqual(["People"]);
  });

  it("is kept on this device: a path to the front, once, and nothing else", () => {
    keepLately("/users");
    keepLately("/users/u1");
    keepLately("/users");
    expect(recallLately()).toEqual(["/users", "/users/u1"]);
    expect(window.localStorage.getItem("syndra:lately")).toBe('["/users","/users/u1"]');
  });

  it("shows on the phone too, above the sheet's own list, and the sheet's list stays", () => {
    keepLately("/users/u1");
    const client = new QueryClient();
    client.setQueryData(NAME_CATALOG_KEY, { users: [{ id: "u1", name: "Becky Lynch", email: "becky@example.org" }], projects: [], applications: [] });
    render(
      <QueryClientProvider client={client}>
        <GoToPanel embedded nav={ADVANCED_NAV} onGo={() => {}} idle={<p>The sheet&apos;s pages</p>} />
      </QueryClientProvider>,
    );
    expect(screen.getByText("Lately")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Becky Lynch/ })).toBeInTheDocument();
    // Only Lately: the sheet's own list is its Pages.
    expect(screen.queryByText("Pages")).toBeNull();
    expect(screen.getByText("The sheet's pages")).toBeInTheDocument();
  });

  it("says one thing per row: a top-level page is a Page, with no 'A page' under it", () => {
    const rows = goTo({ nav: ADVANCED_NAV, names: true, bundles: true, bundleList: [{ id: "b1", name: "Printing basics", roles: ["a", "b"] }] as never }, "");
    const leaf = rows.flatMap((g) => g.items).find((item) => item.href === "/users")!;
    expect(leaf).toMatchObject({ kind: "Page", descriptor: "" });
    const typed = goTo({ nav: ADVANCED_NAV, names: true, bundles: true, bundleList: [{ id: "b1", name: "Printing basics", roles: ["a", "b"] }] as never }, "printing");
    expect(typed.flatMap((g) => g.items).find((item) => item.kind === "Bundle")?.descriptor).toBe("2 roles");
    for (const item of goTo({ nav: ADVANCED_NAV }, "").flatMap((g) => g.items)) {
      expect(item.descriptor, item.name).not.toMatch(/^An? (page|app|bundle)/);
    }
  });

  it("shows in Go to with nothing typed", () => {
    keepLately("/users/u1");
    renderGoTo();
    expect(screen.getByText("Lately")).toBeInTheDocument();
    expect(screen.getAllByRole("option")[0]).toHaveTextContent("Becky Lynch");
  });
});

describe("reduced motion keeps tint (§11.10, Q15)", () => {
  it("keeps the tint utilities' own durations inside the reduced block, and nothing that travels", () => {
    const reduce = body(css, "@media (prefers-reduced-motion: reduce)");
    // One rule reads each utility's own list back, so a list is never copied and the ring still appears at once.
    expect(reduce).toMatch(/\.motion-tint,\s*\.hover-tint,\s*\.link-hover\s*\{\s*transition-duration: var\(--tint-durations\) !important;/);
    for (const name of ["motion-tint", "hover-tint", "link-hover"]) {
      const utility = body(css, `@utility ${name} {`);
      const count = (utility.match(/transition-property:([^;]*);/)?.[1] ?? "").split(",").length;
      const list = utility.match(/--tint-durations:([^;]*);/)?.[1] ?? "";
      expect(list.split(",").length, name).toBe(count);
      expect(list.trim().endsWith("var(--ring-fade)"), name).toBe(true);
      expect(utility, name).toContain("transition-duration: var(--tint-durations);");
    }
    expect(body(reduce, ".tint-in,")).toContain("animation-duration: var(--duration-tint) !important");
    expect(reduce).toMatch(/\.tint-in,\s*\.tint-out\s*\{/);
  });

  it("still crossfades words changing in place for a reader who asked for less motion", () => {
    motionInTests();
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { container, rerender } = render(<Crossfade id="sent">Sent</Crossfade>);
    rerender(<Crossfade id="seen">In Zitadel</Crossfade>);
    expect(container.querySelector(".tint-out")).toHaveTextContent("Sent");
  });
});

describe("figures (§11.1, §11.2)", () => {
  it("are tabular by one rule, on figures only, and nothing else sets it", () => {
    expect(body(css, "  [data-figure] {")).toContain("font-variant-numeric: tabular-nums");
    expect(css.match(/font-variant-numeric\s*:/g)).toHaveLength(1);
    expect(body(css, "  body {")).not.toContain("font-variant");
    const again: string[] = [];
    for (const file of sources(SRC)) if (/\b(?:tabular|proportional)-nums\b/.test(readFileSync(file, "utf8"))) again.push(file.replace(SRC, "src"));
    expect(again).toEqual([]);
  });

  it("are carried by the parts that draw figures: Time, a count, StatCard", () => {
    render(
      <>
        <Relative iso="2026-11-01T10:00:00Z" />
        <Count n={12} />
        <StatCard label="Waiting" value="3" detail="Changes not sent yet." />
      </>,
    );
    expect(document.querySelector("time[data-figure]")).toHaveAttribute("dateTime", "2026-11-01T10:00:00Z");
    expect(screen.getByText("12")).toHaveAttribute("data-figure");
    expect(screen.getByText("3")).toHaveAttribute("data-figure");
  });

  it("change only where they changed: 47 to 46 fades the 7 and holds the 4, then is one word again", () => {
    vi.useFakeTimers();
    motionInTests();
    const { container, rerender } = render(<Count n={47} />);
    const figure = container.querySelector("[data-figure]")!;
    // At rest, plain text: a screen reader hears "47", not "4, 7".
    expect(figure.children).toHaveLength(0);
    expect(figure).toHaveTextContent("47");

    rerender(<Count n={46} />);
    expect(Array.from(figure.querySelectorAll(".tint-out")).map((digit) => digit.textContent)).toEqual(["7"]);
    expect(Array.from(figure.querySelectorAll(".tint-in")).map((digit) => digit.textContent)).toEqual(["6"]);
    // The old text is out of the flow, the reading and the selection.
    const old = figure.querySelector(".tint-out")!.parentElement!;
    expect(old).toHaveAttribute("aria-hidden");
    expect(old).toHaveClass("absolute", "select-none");
    const live = Array.from(figure.children).filter((child) => !child.hasAttribute("aria-hidden"));
    expect(live.map((digit) => digit.textContent).join("")).toBe("46");

    act(() => void vi.advanceTimersByTime(TINT_MS));
    expect(figure.children).toHaveLength(0);
    expect(figure).toHaveTextContent("46");
    vi.useRealTimers();
  });
});

describe("the small things in CSS and the shell (§11.3, §11.6, §11.7)", () => {
  it("keeps the scrollbar's seat, so the page never shifts sideways", () => {
    expect(readFileSync(resolve(SRC, "components/shell/AppShell.tsx"), "utf8")).toMatch(/id="app-scroll"[^>]*\[scrollbar-gutter:stable\]/);
  });

  it("keeps a sheet's scroll inside the sheet", () => {
    for (const file of ["components/ui/Modal.tsx", "components/shell/TouchNav.tsx", "components/ui/FilterPanel.tsx"]) {
      expect(readFileSync(resolve(SRC, file), "utf8"), file).toMatch(/overflow-y-auto overscroll-contain/);
    }
  });

  it("draws the caret in the accent in every field", () => {
    expect(body(css, "  input,\n  textarea,\n  [contenteditable] {")).toContain("caret-color: var(--accent)");
  });
});

describe("a second tooltip is instant (§11.11)", () => {
  const popover = () => document.querySelector('[role="note"]:not(.sr-only)');

  it("opens the first on its rise, and its neighbour at once", () => {
    vi.useFakeTimers();
    render(
      <p>
        <Term name="outbox">outbox</Term> and <Term name="hold">hold</Term>
      </p>,
    );
    const outbox = screen.getByRole("button", { name: "outbox" }).parentElement!;
    const hold = screen.getByRole("button", { name: "hold" }).parentElement!;

    fireEvent.mouseEnter(outbox);
    expect(popover()).toHaveClass("popover-in");
    fireEvent.mouseLeave(outbox);
    fireEvent.mouseEnter(hold);
    expect(popover()).not.toHaveClass("popover-in");

    fireEvent.mouseLeave(hold);
    act(() => void vi.advanceTimersByTime(TIP_WARM_MS + 1));
    fireEvent.mouseEnter(outbox);
    expect(popover()).toHaveClass("popover-in");
    vi.useRealTimers();
  });
});
