// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api-client";

// Makerspace settings (owner, 19 Oct 2026): the term dates are read by Trends,
// "new this term", the keep dialog and Ask for access, so they are edited in
// one global place. One row per term; one way on, in the card header.

const respond = vi.fn();
vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: (path: string, init?: { method?: string; body?: unknown }) => respond(path, init),
}));
vi.mock("@/components/names/UserName", () => ({
  UserName: ({ id }: { id: string }) => <>{id === "u1" ? "Kanishk Singh" : id}</>,
}));

import SettingsPage from "@/app/settings/page";

const Y = new Date().getFullYear();
const TERMS = [
  { name: "Spring", start: `${Y}-01-01`, end: `${Y}-05-31` },
  { name: "Summer", start: `${Y}-06-01`, end: `${Y}-07-31` },
  { name: "Monsoon", start: `${Y}-08-01`, end: `${Y}-12-31` },
];

function mount(read: unknown = { terms: TERMS, set_by: "u1", set_at: `${Y}-03-04T10:00:00Z` }) {
  respond.mockImplementation(async (path: string, init?: { method?: string }) => {
    if (init?.method === "PUT") return {};
    if (path === "/terms") {
      if (read instanceof Error) throw read;
      return read;
    }
    throw new Error(`unexpected ${path}`);
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SettingsPage />
    </QueryClientProvider>,
  );
}

const puts = () => respond.mock.calls.filter(([, init]) => init?.method === "PUT");
const dialog = () => screen.getByRole("dialog");

beforeEach(() => {
  respond.mockReset();
});
afterEach(cleanup);

describe("Makerspace settings page", () => {
  it("is titled, and its Term dates card says it is for the whole makerspace", async () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "Makerspace settings" })).toBeTruthy();
    await screen.findByText("Spring");
    expect(screen.getByText("Term dates")).toBeTruthy();
    expect(screen.getByText("For the whole makerspace")).toBeTruthy();
    expect(
      screen.getByText(/Trends, "new this term" and "to the end of next term" read these\. Terms move each year, so check them when one ends\./),
    ).toBeTruthy();
  });

  it("shows one row per term, name then its dates", async () => {
    mount();
    const row = (await screen.findByText("Monsoon")).closest("[data-term-row]") as HTMLElement;
    expect(row.textContent).toContain("1 Aug to 31 Dec");
    expect(document.querySelectorAll("[data-term-row]")).toHaveLength(3);
  });

  it("shows the year only when a term's day is not this year", async () => {
    mount({ terms: [{ name: "Spring", start: `${Y + 1}-01-05`, end: `${Y + 1}-05-31` }] });
    expect(await screen.findByText(`5 Jan ${Y + 1} to 31 May ${Y + 1}`)).toBeTruthy();
  });

  it("says who set it and when, or that it was never set", async () => {
    mount();
    expect(await screen.findByText("Set by Kanishk Singh on 4 Mar.")).toBeTruthy();
    cleanup();
    mount({ terms: TERMS, set_by: null, set_at: null });
    expect(await screen.findByText("Never set. These are the usual dates until you change them.")).toBeTruthy();
  });

  it("claims nothing before the read lands, or when it fails", async () => {
    mount(new ApiError(500, "boom"));
    await screen.findByText(/Couldn't load the term dates/);
    expect(screen.queryByRole("button", { name: "Edit term dates" })).toBeNull();
    expect(screen.queryByText(/Never set/)).toBeNull();
    expect(screen.queryByText(/Set by/)).toBeNull();
  });
});

describe("Edit term dates", () => {
  async function open() {
    mount();
    await screen.findByText("Spring");
    fireEvent.click(screen.getByRole("button", { name: "Edit term dates" }));
    return dialog();
  }

  it("opens one row per term with Name, Start and End, and says what saving does before it happens", async () => {
    const d = await open();
    expect(within(d).getAllByLabelText("Name")).toHaveLength(3);
    expect(within(d).getAllByLabelText("Start")).toHaveLength(3);
    expect(within(d).getAllByLabelText("End")).toHaveLength(3);
    expect((within(d).getAllByLabelText("Start")[0] as HTMLInputElement).type).toBe("date");
    expect(within(d).getByText("Saving changes the dates for everyone. Nothing about anyone's access changes.")).toBeTruthy();
    expect(within(d).getByRole("button", { name: "Cancel" })).toBeTruthy();
    expect(within(d).getByRole("button", { name: "Save term dates" })).toBeTruthy();
  });

  it("sends the edited terms, then ends on its result with Close", async () => {
    const d = await open();
    fireEvent.change(within(d).getAllByLabelText("End")[2], { target: { value: `${Y}-12-20` } });
    fireEvent.click(within(d).getByRole("button", { name: "Save term dates" }));

    await within(dialog()).findByText("Saved");
    expect(puts()).toHaveLength(1);
    expect(puts()[0][0]).toBe("/terms");
    expect(puts()[0][1].body).toEqual({
      terms: [TERMS[0], TERMS[1], { ...TERMS[2], end: `${Y}-12-20` }],
    });
    expect(within(dialog()).getByText("Trends and new this term now use these dates.")).toBeTruthy();
    expect(within(dialog()).queryByLabelText("Name")).toBeNull();
    expect(within(dialog()).queryByRole("button", { name: "Save term dates" })).toBeNull();
    fireEvent.click(within(dialog()).getByRole("button", { name: "Close" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("refuses to send an overlap, saying so under the field", async () => {
    const d = await open();
    fireEvent.change(within(d).getAllByLabelText("Start")[1], { target: { value: `${Y}-05-31` } });
    fireEvent.click(within(d).getByRole("button", { name: "Save term dates" }));

    expect(await within(dialog()).findByText(/Overlaps Spring/)).toBeTruthy();
    expect(puts()).toHaveLength(0);
  });

  it("clears a shown error as soon as the field is edited", async () => {
    const d = await open();
    fireEvent.change(within(d).getAllByLabelText("Start")[1], { target: { value: `${Y}-05-31` } });
    fireEvent.click(within(d).getByRole("button", { name: "Save term dates" }));
    await within(dialog()).findByText(/Overlaps Spring/);
    fireEvent.change(within(dialog()).getAllByLabelText("Start")[1], { target: { value: `${Y}-06-01` } });
    expect(within(dialog()).queryByText(/Overlaps Spring/)).toBeNull();
  });

  it("shows the server's field errors under the right field, and stays open", async () => {
    const d = await open();
    respond.mockImplementation(async (_p: string, init?: { method?: string }) => {
      if (init?.method === "PUT")
        throw new ApiError(400, {
          error: "VALIDATION_FAILED",
          message: "Some of that needs another look",
          details: { "terms[1].start": "overlaps Spring", request_id: "req-9" },
        });
      return { terms: TERMS };
    });
    fireEvent.click(within(d).getByRole("button", { name: "Save term dates" }));

    const under = (await within(dialog()).findByText("Overlaps Spring.")).closest("[data-term-field]") as HTMLElement;
    expect(under.getAttribute("data-term-field")).toBe("terms[1].start");
    expect(within(dialog()).queryByText("Saved")).toBeNull();
    expect(within(dialog()).getByRole("button", { name: "Save term dates" })).toBeTruthy();
  });

  it("says a failure that is not about a field, and nothing was changed", async () => {
    const d = await open();
    respond.mockImplementation(async (_p: string, init?: { method?: string }) => {
      if (init?.method === "PUT") throw new ApiError(403, "forbidden");
      return { terms: TERMS };
    });
    fireEvent.click(within(d).getByRole("button", { name: "Save term dates" }));
    expect(await within(dialog()).findByText(/don't have permission/)).toBeTruthy();
    expect(within(dialog()).queryByText("Saved")).toBeNull();
  });

  it("adds a blank term and removes one, never leaving none", async () => {
    const d = await open();
    fireEvent.click(within(d).getByRole("button", { name: "Add a term" }));
    expect(within(dialog()).getAllByLabelText("Name")).toHaveLength(4);
    expect((within(dialog()).getAllByLabelText("Name")[3] as HTMLInputElement).value).toBe("");

    fireEvent.click(within(dialog()).getAllByRole("button", { name: /^Remove/ })[3]);
    expect(within(dialog()).getAllByLabelText("Name")).toHaveLength(3);

    for (let i = 0; i < 2; i++) fireEvent.click(within(dialog()).getAllByRole("button", { name: /^Remove/ })[0]);
    expect(within(dialog()).getAllByLabelText("Name")).toHaveLength(1);
    expect((within(dialog()).getByRole("button", { name: /^Remove/ }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("starts clean when reopened after Cancel", async () => {
    const d = await open();
    fireEvent.change(within(d).getAllByLabelText("Name")[0], { target: { value: "Edited" } });
    fireEvent.click(within(d).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    fireEvent.click(screen.getByRole("button", { name: "Edit term dates" }));
    expect((within(dialog()).getAllByLabelText("Name")[0] as HTMLInputElement).value).toBe("Spring");
  });
});
