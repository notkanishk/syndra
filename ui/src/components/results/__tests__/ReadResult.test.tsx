// @vitest-environment jsdom
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Reads, ReadResult } from "@/components/results/ReadResult";
import { ActivityList } from "@/components/results/ActivityList";
import {
  NOT_READABLE_NEXT,
  emptyEvents,
  emptyMeters,
  emptyRecords,
  notReadableTitle,
  usageUnread,
} from "@/components/results/copy";
import type { ActivityResult, ReadView, RecordsResult, StatusResult } from "@/lib/queries/useMyStorage";

// The generic renderers, shape by shape. What matters in each: the empty and
// not-readable states end on a next step, nothing is decided from a value,
// and nothing here knows what product answered.

const NAME = "Parts shelf";
const taken = { taken_at: "2026-11-10T09:30:00Z", current: true };
const show = (read: ReadView, audience: "member" | "operator" = "member") => render(<ReadResult read={read} name={NAME} audience={audience} />);

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
      expect(screen.getByText(NOT_READABLE_NEXT)).toBeInTheDocument();
      expect(screen.queryByText(emptyRecords(NAME))).toBeNull();
      expect(screen.queryByText(emptyMeters(NAME))).toBeNull();
      expect(screen.queryByText(emptyEvents(NAME))).toBeNull();
    });
  }
});

describe("readable is the add-on's word", () => {
  it("a read that says it is not readable is never drawn as a result, even if a result came with it", () => {
    show({ ...records([{ id: "a", label: "Hot glue gun" }]), readable: false });
    expect(screen.getByText(notReadableTitle(NAME))).toBeInTheDocument();
    expect(screen.queryByText("Hot glue gun")).toBeNull();
  });
});

describe("records", () => {
  it("says an empty list in the guide's words, ending on nothing to do", () => {
    show(records([]));
    expect(screen.getByText("You hold nothing from Parts shelf. Nothing for you to do.")).toBeInTheDocument();
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
    expect(screen.getByText(/measuring/)).toHaveTextContent(/Since 20 Oct.*Due 3 Nov/);
    expect(screen.getByText("Overdue since 3 November")).toBeInTheDocument();
    expect(screen.getByText("Bring it back to the front desk")).toBeInTheDocument();
  });

  it("draws a record with no standing and no dates as just its label", () => {
    const { container } = show(records([{ id: "a", label: "Hot glue gun" }]));
    expect(screen.getByText("Hot glue gun")).toBeInTheDocument();
    expect(container.querySelector("[data-standing]")).toBeNull();
    expect(container.textContent).not.toMatch(/Due|Since/);
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
    expect(screen.getByText(NOT_READABLE_NEXT)).toBeInTheDocument();
    expect(screen.queryByText(emptyMeters(NAME))).toBeNull();
    expect(screen.queryByRole("meter")).toBeNull();
  });

  it("quota_read false with meters keeps the meters that were read and says the rest was not", () => {
    show(status({ meters: [{ label: "makers", used: 5, limit: 10, unit: "items" }], quota_read: false }));
    expect(screen.getByRole("meter", { name: "makers" })).toBeInTheDocument();
    expect(screen.getByText(usageUnread(NAME, true))).toBeInTheDocument();
    expect(screen.getByText(NOT_READABLE_NEXT)).toBeInTheDocument();
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

  it("names the places that were not recording, and says a quiet list is not proof", () => {
    show(activity({ events: [event], uncovered: ["scratch", "archive"] }));
    expect(screen.getByText(/archive and scratch were not recording Parts shelf|scratch and archive were not recording Parts shelf/)).toBeInTheDocument();
  });

  it("lets a kind say the places in its own words", () => {
    render(<ActivityList result={{ events: [], uncovered: ["scratch"], truncated: false, ...taken }} name={NAME} coverage={<>Kind words here.</>} />);
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
    const { container, rerender } = render(<Reads reads={[a, b]} name={NAME} audience="member" />);
    const order = [...container.querySelectorAll("[data-read]")].map((n) => n.getAttribute("data-read"));
    expect(order).toEqual(["holdings.list", "usage.get"]);
    rerender(<Reads reads={[a, b]} skip={["holdings.list"]} name={NAME} audience="member" />);
    expect([...container.querySelectorAll("[data-read]")].map((n) => n.getAttribute("data-read"))).toEqual(["usage.get"]);
    rerender(<Reads reads={undefined} name={NAME} audience="member" />);
    expect(container.innerHTML).toBe("");
  });
});
