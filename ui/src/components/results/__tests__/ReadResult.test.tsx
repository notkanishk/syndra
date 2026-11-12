// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Reads, ReadResult } from "@/components/results/ReadResult";
import { ActivityList } from "@/components/results/ActivityList";
import {
  MEMBER,
  emptyEvents,
  emptyMeters,
  emptyRecords,
  notReadableNext,
  notReadableTitle,
  uncoveredNote,
  usageUnread,
  type Reader,
} from "@/components/results/copy";
import type { ActivityResult, ReadView, RecordsResult, StatusResult } from "@/lib/queries/useMyStorage";

// The generic renderers, shape by shape. What matters in each: the empty and
// not-readable states end on a next step, nothing is decided from a value,
// and nothing here knows what product answered.

const NAME = "Parts shelf";
const taken = { taken_at: "2026-11-10T09:30:00Z", current: true };
const PERSON = "Bobby Roode";
const OPERATOR: Reader = { audience: "operator", person: PERSON };
const show = (read: ReadView, reader: Reader = MEMBER) => render(<ReadResult read={read} name={NAME} reader={reader} />);

const records = (r: RecordsResult["records"]): ReadView => ({ operation: "holdings.list", shape: "records", readable: true, result: { records: r, ...taken } });
const status = (over: Partial<StatusResult>): ReadView => ({
  operation: "usage.get",
  shape: "status",
  readable: true,
  result: { usable: true, standing: { usable: true, waiting_on: "none" }, meters: [], ...taken, ...over },
});
const activity = (over: Partial<ActivityResult>): ReadView => ({
  operation: "activity.get",
  shape: "activity",
  readable: true,
  result: { events: [], uncovered: [], truncated: false, ...taken, ...over },
});

describe("not readable", () => {
  for (const shape of ["records", "status", "activity", ""] as const) {
    it(`says so, with the next step, for shape "${shape}", and draws no empty result`, () => {
      show({ operation: "x.get", shape, readable: false });
      expect(screen.getByText(notReadableTitle(NAME))).toBeInTheDocument();
      expect(screen.getByText(notReadableNext(MEMBER))).toBeInTheDocument();
      expect(screen.queryByText(emptyRecords(NAME, MEMBER))).toBeNull();
      expect(screen.queryByText(emptyMeters(NAME))).toBeNull();
      expect(screen.queryByText(emptyEvents(NAME))).toBeNull();
    });
  }
});

describe("a read that did not come back, by reader", () => {
  it("names the one read, so it never contradicts the system's own read-just-now line", () => {
    show({ operation: "x.get", shape: "records", readable: false });
    expect(screen.getByText("One read from Parts shelf did not come back")).toBeInTheDocument();
  });

  it("sends a member to staff and an operator to the add-on's health, never the other way", () => {
    const { container, unmount } = show({ operation: "x.get", shape: "status", readable: false });
    expect(container.textContent).toContain("ask makerspace staff");
    expect(container.textContent).not.toContain("health");
    unmount();
    const op = show({ operation: "x.get", shape: "status", readable: false }, OPERATOR);
    expect(op.container.textContent).toContain("check the add-on's health on its system page");
    expect(op.container.textContent).not.toContain("makerspace staff");
  });
});

describe("readable is the add-on's word", () => {
  it("a read that says it is not readable is never drawn as a result, even if a result came with it", () => {
    show({ ...records([{ id: "a", label: "Hot glue gun" }]), readable: false });
    expect(screen.getByText(notReadableTitle(NAME))).toBeInTheDocument();
    expect(screen.queryByText("Hot glue gun")).toBeNull();
  });
});

describe("records", () => {
  it("says an empty list in the guide's words, for the reader, ending on nothing to do", () => {
    const { unmount } = show(records([]));
    expect(screen.getByText("Parts shelf lists nothing for you. Nothing for you to do.")).toBeInTheDocument();
    unmount();
    show(records([]), OPERATOR);
    expect(screen.getByText("Parts shelf lists nothing for Bobby Roode. Nothing for you to do.")).toBeInTheDocument();
  });

  const order = (container: HTMLElement) =>
    [...container.querySelectorAll("li > span:first-child > span:first-child")].map((n) => n.textContent);
  const dated = records([
    { id: "1", label: "zip ties" },
    { id: "2", label: "Caliper", due: "2026-12-01T00:00:00Z" },
    { id: "3", label: "bandsaw", due: "2026-11-03T00:00:00Z" },
    { id: "4", label: "Anvil", due: "2026-12-01T00:00:00Z", since: "2026-11-01T00:00:00Z" },
    { id: "5", label: "Awl" },
    { id: "6", label: "Jig", due: "2026-10-01T00:00:00Z" },
  ]);

  it("opens in the logical order: earliest due first (overdue before due soon), then no due date, then name", () => {
    const { container } = show(dated);
    // Jig and bandsaw are the earlier dates, Anvil and Caliper tie on a day (name), Awl and zip ties have none (name).
    expect(order(container)).toEqual(["Jig", "bandsaw", "Anvil", "Caliper", "Awl", "zip ties"]);
  });

  it("re-sorts by a heading, reverses on a second press, and keeps records with no date last either way", () => {
    const { container } = show(dated);
    fireEvent.click(screen.getByRole("button", { name: "Sort by due" }));
    expect(order(container)).toEqual(["Anvil", "Caliper", "bandsaw", "Jig", "Awl", "zip ties"]); // ties stay in name order
    fireEvent.click(screen.getByRole("button", { name: "Sort by record" }));
    expect(order(container)).toEqual(["Anvil", "Awl", "bandsaw", "Caliper", "Jig", "zip ties"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by record" }));
    expect(order(container)).toEqual(["zip ties", "Jig", "Caliper", "bandsaw", "Awl", "Anvil"]);
    fireEvent.click(screen.getByRole("button", { name: "Sort by since" }));
    expect(order(container)[0]).toBe("Anvil");
  });

  it("marks the heading in force, and the list opens on Due", () => {
    show(dated);
    expect(screen.getByRole("columnheader", { name: /due/i })).toHaveAttribute("aria-sort", "ascending");
    expect(screen.getByRole("columnheader", { name: /record/i })).toHaveAttribute("aria-sort", "none");
  });

  it("shows label, category and both dates, and a standing's note and next step", () => {
    show(
      records([
        {
          id: "a",
          label: "Digital caliper",
          category: "measuring",
          since: "2026-10-20T11:00:00Z",
          due: "2026-11-03T17:00:00Z",
          standing: { usable: false, waiting_on: "member", note: "Overdue since 3 November", next: "Bring it back to the front desk" },
        },
      ]),
    );
    expect(screen.getByText("Digital caliper")).toBeInTheDocument();
    expect(screen.getByText("measuring")).toBeInTheDocument();
    expect(screen.getByText("Since 20 Oct")).toBeInTheDocument();
    expect(screen.getByText("Due 3 Nov")).toBeInTheDocument();
    expect(screen.getByText("Overdue since 3 November")).toBeInTheDocument();
    expect(screen.getByText("Bring it back to the front desk")).toBeInTheDocument();
  });

  it("draws a record with no standing and no dates as just its label", () => {
    const { container } = show(records([{ id: "a", label: "Hot glue gun" }]));
    expect(screen.getByText("Hot glue gun")).toBeInTheDocument();
    expect(container.querySelector("[data-standing]")).toBeNull();
    expect(container.querySelector("li")?.textContent).not.toMatch(/Due|Since/);
  });

  it("never works out lateness from a date: a past due date with no standing says nothing is late", () => {
    show(records([{ id: "a", label: "Old loan", due: "2020-01-01T00:00:00Z" }]));
    expect(screen.queryByText(/overdue|late/i)).toBeNull();
  });

  it("says how old the read is", () => {
    const { container } = show(records([{ id: "a", label: "Hot glue gun" }]));
    expect(container.querySelector("[data-freshness-dot]")).not.toBeNull();
  });
});

describe("status meters", () => {
  const GB = 1024 ** 3;

  it("shows used against the limit in the unit's words, with a meter", () => {
    show(status({ meters: [{ label: "makers", used: 19 * GB, limit: 50 * GB, unit: "bytes" }] }));
    expect(screen.getByText(/19 GB · of 50 GB/)).toBeInTheDocument();
    const meter = screen.getByRole("meter", { name: "makers" });
    expect(meter).toHaveAttribute("aria-valuemax", String(50 * GB));
    expect(meter).toHaveAttribute("aria-valuenow", String(19 * GB));
  });

  it("says no limit, and draws no bar, when there is no ceiling", () => {
    show(status({ meters: [{ label: "scratch", used: 3 * GB, unit: "bytes" }] }));
    expect(screen.getByText(/3 GB · no limit/)).toBeInTheDocument();
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("writes a unit it has no words for after the number", () => {
    show(status({ meters: [{ label: "Laser time", used: 12, limit: 40, unit: "credits" }] }));
    expect(screen.getByText(/12 credits · of 40 credits/)).toBeInTheDocument();
  });

  it("never draws a bar past full", () => {
    show(status({ meters: [{ label: "over", used: 60, limit: 50, unit: "items" }] }));
    expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "50");
  });

  it("says no usage is reported when the list is empty and nothing was unread", () => {
    show(status({ meters: [], quota_read: true }));
    expect(screen.getByText(emptyMeters(NAME))).toBeInTheDocument();
  });

  it("quota_read false with no meters says it could not be read, with a next step, and never empty meters or 'nothing used'", () => {
    show(status({ meters: [], quota_read: false }));
    expect(screen.getByText(usageUnread(NAME, false))).toBeInTheDocument();
    expect(screen.getByText(notReadableNext(MEMBER))).toBeInTheDocument();
    expect(screen.queryByText(emptyMeters(NAME))).toBeNull();
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("quota_read false with meters keeps the meters that were read and says the rest was not", () => {
    show(status({ meters: [{ label: "makers", used: 5, limit: 10, unit: "items" }], quota_read: false }));
    expect(screen.getByRole("meter", { name: "makers" })).toBeInTheDocument();
    expect(screen.getByText(usageUnread(NAME, true))).toBeInTheDocument();
    expect(screen.getByText(notReadableNext(MEMBER))).toBeInTheDocument();
  });

  it("lists meters alphabetically by label, case aside", () => {
    show(
      status({
        meters: [
          { label: "scratch", used: 1, limit: 2, unit: "items" },
          { label: "Archive", used: 1, limit: 2, unit: "items" },
          { label: "makers", used: 1, limit: 2, unit: "items" },
        ],
      }),
    );
    expect(screen.getAllByRole("meter").map((m) => m.getAttribute("aria-label"))).toEqual(["Archive", "makers", "scratch"]);
  });

  it("says the unread usage to an operator with an operator's next step", () => {
    show(status({ meters: [], quota_read: false }), OPERATOR);
    expect(screen.getByText(notReadableNext(OPERATOR))).toBeInTheDocument();
    expect(screen.queryByText(notReadableNext(MEMBER))).toBeNull();
  });
});

describe("activity", () => {
  const event = { at: "2026-11-10T08:12:30Z", event: "CONNECT", success: false, resource: "makers", address: "192.0.2.17", detail: "NT_STATUS_NO_SUCH_USER" };

  it("says an empty log in the guide's words", () => {
    show(activity({}));
    expect(screen.getByText("No recorded activity on Parts shelf. Nothing for you to do.")).toBeInTheDocument();
  });

  it("shows resource as its own column, with the address, the detail and the refusal", () => {
    const { container } = show(activity({ events: [event] }));
    expect(container.querySelector("[data-resource]")).toHaveTextContent("makers");
    expect(screen.getByText("192.0.2.17")).toBeInTheDocument();
    expect(screen.getByText("NT_STATUS_NO_SUCH_USER")).toBeInTheDocument();
    expect(screen.getByText("Refused")).toBeInTheDocument();
  });

  it("names the places that were not recording, alphabetically, and says a quiet list is not proof", () => {
    show(activity({ events: [event], uncovered: ["scratch", "Archive"] }));
    expect(screen.getByText(/^Archive and scratch did not record you, so nothing from those places appears here, whether or not it happened\. Nothing for you to do\.$/)).toBeInTheDocument();
  });

  it("names the person to an operator, one place or many, and says to switch recording on", () => {
    const one = show(activity({ uncovered: ["scratch"] }), OPERATOR);
    expect(one.container.textContent).toContain("scratch did not record Bobby Roode, so nothing from that place appears here");
    expect(one.container.textContent).toContain("Switch recording on there.");
    expect(one.container.textContent).not.toContain("Nothing for you to do. ");
    one.unmount();
    const many = show(activity({ uncovered: ["b", "a"] }), OPERATOR);
    expect(many.container.textContent).toContain("a and b did not record Bobby Roode, so nothing from those places appears here");
  });

  it("is worded by the same sentence the guide holds", () => {
    expect(uncoveredNote(["scratch"], MEMBER)).toContain("did not record you");
  });

  it("lets a kind say the places in its own words", () => {
    render(<ActivityList result={{ events: [], uncovered: ["scratch"], truncated: false, ...taken }} name={NAME} reader={MEMBER} coverage={<>Kind words here.</>} />);
    expect(screen.getByText("Kind words here.")).toBeInTheDocument();
  });

  it("says when the read was cut short", () => {
    show(activity({ events: [event], truncated: true }));
    expect(screen.getByText(/not the whole list/)).toBeInTheDocument();
  });
});

describe("an unknown shape", () => {
  it("falls back to the raw view, behind Technical detail, exactly as it came", () => {
    const { container } = show({ operation: "x.get", shape: "", readable: true, result: '{"queue":3}' });
    expect(container.textContent).not.toContain('{"queue":3}');
    fireEvent.click(within(container).getByText("Technical detail"));
    expect(container.textContent).toContain('{"queue":3}');
  });
});

describe("what the add-on also says", () => {
  it("shows x- values raw and escaped, never as markup", () => {
    const { container } = show({
      ...records([{ id: "a", label: "Hot glue gun" }]),
      also_says: ['"records[0].x-inventory-shelf": "<img src=x onerror=alert(1)>"'],
    });
    expect(screen.getByText("What the add-on also says")).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toContain('"records[0].x-inventory-shelf": "<img src=x onerror=alert(1)>"');
  });

  it("caps a long line and a long list", () => {
    const lines = Array.from({ length: 60 }, (_, i) => `"x-${i}": "${"a".repeat(i === 0 ? 900 : 3)}"`);
    const { container } = show({ ...records([]), also_says: lines });
    const items = container.querySelectorAll("[data-also-says] li");
    expect(items).toHaveLength(51);
    expect(items[0].textContent!.length).toBeLessThanOrEqual(401);
    expect(items[0].textContent!.endsWith("…")).toBe(true);
    expect(items[50]).toHaveTextContent("…and 10 more");
  });

  it("is absent when there is nothing to say", () => {
    const { container } = show(records([]));
    expect(container.querySelector("[data-also-says]")).toBeNull();
  });
});

describe("Reads", () => {
  it("draws every read in the order given, skips the ones a kind draws itself, and nothing for none", () => {
    const a = records([{ id: "a", label: "First thing" }]);
    const b = { ...status({ meters: [{ label: "Second meter", used: 1, unit: "items" }] }) };
    const { container, rerender } = render(<Reads reads={[a, b]} name={NAME} reader={MEMBER} />);
    const order = [...container.querySelectorAll("[data-read]")].map((n) => n.getAttribute("data-read"));
    expect(order).toEqual(["holdings.list", "usage.get"]);
    rerender(<Reads reads={[a, b]} skip={["holdings.list"]} name={NAME} reader={MEMBER} />);
    expect([...container.querySelectorAll("[data-read]")].map((n) => n.getAttribute("data-read"))).toEqual(["usage.get"]);
    rerender(<Reads reads={undefined} name={NAME} reader={MEMBER} />);
    expect(container.innerHTML).toBe("");
  });
});

describe("long text", () => {
  // jsdom has no layout, so the guard is the wrapping rule itself; the
  // 390px screenshots (design handoff, 6a) are the proof it does its work.
  const LABEL = "Soldering station ".repeat(17).trim(); // 300 characters
  const TOKEN = "x".repeat(120);
  const MAPPED = "::ffff:192.0.2.17";
  const wraps = (n: Element | null) => expect(n?.className).toContain("[overflow-wrap:anywhere]");

  it("lets the whole read break anywhere, so a long label, token or URL cannot widen the card", () => {
    const { container } = show({
      ...records([{ id: "a", label: LABEL, standing: { usable: false, waiting_on: "member", note: TOKEN, next: `Return it at https://example.org/${TOKEN}` } }]),
    });
    expect(container.textContent).toContain(LABEL);
    wraps(container.querySelector("[data-read]"));
    wraps(container.querySelector("[data-standing]"));
  });

  it("lets an activity row break anywhere and its columns shrink to a minimum rather than fix a width", () => {
    const { container } = show(activity({ events: [{ at: "2026-11-10T08:12:30Z", event: LABEL, success: true, resource: TOKEN, address: MAPPED, detail: TOKEN }] }));
    const row = container.querySelector("[data-resource]")!.parentElement;
    wraps(row);
    for (const col of [container.querySelector("[data-resource]"), screen.getByText(MAPPED)]) {
      const tokens = col!.className.split(/\s+/);
      expect(tokens).toContain("min-w-[120px]");
      expect(tokens).not.toContain("w-[120px]");
      expect(tokens).not.toContain("shrink-0");
    }
  });
});
