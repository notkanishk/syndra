// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { KeysHost } from "@/components/shell/KeysHost";
import { ArriveList } from "@/lib/motion";
import { KEY_ROWS } from "@/lib/keys";

const key = (init: KeyboardEventInit, target: Element | Window = window) => fireEvent.keyDown(target, init);

function mount(extra?: React.ReactNode) {
  return render(
    <>
      <input aria-label="a field" />
      {extra}
      <KeysHost />
    </>,
  );
}

describe("the Keys sheet (A-19 P7)", () => {
  it("opens on ? and lists every key once, drawn as keys", () => {
    mount();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    key({ key: "?" });
    const sheet = screen.getByRole("dialog", { name: "Keys" });
    for (const row of KEY_ROWS) {
      expect(within(sheet).getByText(row.does)).toBeInTheDocument();
    }
    expect(sheet.querySelectorAll("kbd").length).toBe(KEY_ROWS.flatMap((row) => row.keys).length);
  });

  it("has no letter shortcut for a verb", () => {
    for (const row of KEY_ROWS) {
      for (const cap of row.keys) expect(cap).not.toMatch(/^[a-z]$/i);
    }
  });

  it("never fires while typing in a field, a textarea or an editable region", () => {
    mount(<textarea aria-label="a note" />);
    key({ key: "?" }, screen.getByLabelText("a field"));
    key({ key: "?" }, screen.getByLabelText("a note"));
    const editable = document.body.appendChild(document.createElement("div"));
    Object.defineProperty(editable, "isContentEditable", { value: true });
    key({ key: "?" }, editable);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("closes on Esc", () => {
    mount();
    key({ key: "?" });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    key({ key: "Escape" }, screen.getByRole("dialog"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("does not stack on another dialog", () => {
    mount(<div role="dialog" aria-label="Something else" />);
    key({ key: "?" });
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });

  it("stands aside when a page answers ? itself", () => {
    mount();
    const take = (event: KeyboardEvent) => event.preventDefault();
    document.addEventListener("keydown", take);
    key({ key: "?" }, document.body);
    document.removeEventListener("keydown", take);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("ignores ? with a modifier held", () => {
    mount();
    key({ key: "?", ctrlKey: true });
    key({ key: "?", metaKey: true });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});

function rows() {
  return (
    <ArriveList startSettled>
      {["Ada", "Ben", "Chi"].map((name) => (
        <div key={name}>
          <a data-row href={`/users/${name}`}>
            {name}
          </a>
          <button>{`Extend ${name}`}</button>
        </div>
      ))}
    </ArriveList>
  );
}

describe("arrows between rows (A-19 P7)", () => {
  it("puts one row in the Tab order and the rest out of it", () => {
    render(rows());
    expect(screen.getByRole("link", { name: "Ada" })).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("link", { name: "Ben" })).toHaveAttribute("tabindex", "-1");
    expect(screen.getByRole("link", { name: "Chi" })).toHaveAttribute("tabindex", "-1");
    // The row's own buttons stay on Tab.
    expect(screen.getByRole("button", { name: "Extend Ben" })).not.toHaveAttribute("tabindex");
  });

  it("moves down and up, and the row focused takes the Tab stop", () => {
    render(rows());
    const [ada, ben, chi] = ["Ada", "Ben", "Chi"].map((n) => screen.getByRole("link", { name: n }));
    ada.focus();
    key({ key: "ArrowDown" }, ada);
    expect(document.activeElement).toBe(ben);
    expect(ben).toHaveAttribute("tabindex", "0");
    expect(ada).toHaveAttribute("tabindex", "-1");
    key({ key: "ArrowDown" }, ben);
    expect(document.activeElement).toBe(chi);
    key({ key: "ArrowUp" }, chi);
    expect(document.activeElement).toBe(ben);
  });

  it("stops at the ends rather than wrapping", () => {
    render(rows());
    const [ada, , chi] = ["Ada", "Ben", "Chi"].map((n) => screen.getByRole("link", { name: n }));
    ada.focus();
    key({ key: "ArrowUp" }, ada);
    expect(document.activeElement).toBe(ada);
    chi.focus();
    key({ key: "ArrowDown" }, chi);
    expect(document.activeElement).toBe(chi);
  });

  it("opens the focused row on Enter, which is the link's own press", () => {
    render(rows());
    const ben = screen.getByRole("link", { name: "Ben" });
    let opened = "";
    ben.addEventListener("click", (event) => {
      event.preventDefault();
      opened = ben.getAttribute("href")!;
    });
    ben.focus();
    // jsdom does not turn Enter into a click; the browser does, for a link. The row is a link.
    expect(ben.tagName).toBe("A");
    fireEvent.click(ben);
    expect(opened).toBe("/users/Ben");
  });

  it("leaves a row's own button alone, and a field too", () => {
    render(
      <>
        {rows()}
        <ArriveList startSettled>
          <a data-row href="/x">
            x
          </a>
          <input aria-label="search" />
        </ArriveList>
      </>,
    );
    const button = screen.getByRole("button", { name: "Extend Ada" });
    button.focus();
    const ev = key({ key: "ArrowDown" }, button);
    expect(ev).toBe(true); // not prevented: the button's arrow is its own
    expect(document.activeElement).toBe(button);
    const field = screen.getByLabelText("search");
    field.focus();
    key({ key: "ArrowDown" }, field);
    expect(document.activeElement).toBe(field);
  });

  it("keeps the Tab stop on the row the reader was on when rows change", () => {
    const { rerender } = render(rows());
    const ben = screen.getByRole("link", { name: "Ben" });
    ben.focus();
    expect(ben).toHaveAttribute("tabindex", "0");
    rerender(
      <ArriveList startSettled>
        {["Ada", "Ben", "Chi", "Dev"].map((name) => (
          <div key={name}>
            <a data-row href={`/users/${name}`}>
              {name}
            </a>
          </div>
        ))}
      </ArriveList>,
    );
    expect(screen.getByRole("link", { name: "Ben" })).toHaveAttribute("tabindex", "0");
    expect(screen.getByRole("link", { name: "Ada" })).toHaveAttribute("tabindex", "-1");
    expect(screen.getByRole("link", { name: "Dev" })).toHaveAttribute("tabindex", "-1");
  });
});
