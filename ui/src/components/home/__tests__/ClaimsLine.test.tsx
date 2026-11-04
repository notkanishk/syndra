// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ClaimsLine } from "@/components/home/ClaimsLine";

const q = vi.hoisted(() => ({ value: {} as Record<string, unknown> }));
vi.mock("@/lib/queries/useClaimsDelivery", () => ({ useClaimsStatus: () => q.value }));

const DAY = 24 * 3600_000;
const status = (over: Record<string, unknown> = {}) => ({
  status: "ok",
  problems: 0,
  reasons: [],
  checking_since: new Date(Date.now() - 3 * DAY).toISOString(),
  sign_ins_24h: { delivered: 12 },
  waiting: 0,
  ...over,
});
const line = () => screen.getByRole("button", { name: "Claims" }).closest("div")!;

beforeEach(() => {
  q.value = { data: status(), error: null, isLoading: false, dataUpdatedAt: Date.now() };
});

describe("Home's claims line", () => {
  it("says delivery is whole in the owner's words", () => {
    render(<ClaimsLine />);
    expect(line()).toHaveTextContent("delivered to every sign-in in the last 24h");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("counts problems and links to System › Zitadel's claims delivery", () => {
    q.value = { ...q.value, data: status({ status: "failing", problems: 2, reasons: ["CONFIG_DRIFT:R2", "CLAIMS_NOT_CALLED"] }) };
    render(<ClaimsLine />);
    const link = screen.getByRole("link", { name: /2 problems/ });
    expect(link).toHaveAttribute("href", "/zitadel#claims-delivery");
    expect(link).toHaveTextContent("→");
  });

  it("says 1 problem, not 1 problems", () => {
    q.value = { ...q.value, data: status({ status: "degraded", problems: 1, reasons: ["REDIS_DOWN"] }) };
    render(<ClaimsLine />);
    expect(screen.getByRole("link", { name: /^1 problem/ })).toBeInTheDocument();
  });

  it("says since when it has been checking after a restart under a day ago", () => {
    // Midday, so "two hours ago" is today and reads as a time, not a date
    // (it failed just after midnight, 4 Nov 2026).
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 4, 12, 0, 0));
    try {
      const since = new Date(Date.now() - 2 * 3600_000);
      q.value = { ...q.value, data: status({ checking_since: since.toISOString() }) };
      render(<ClaimsLine />);
      expect(line()).toHaveTextContent(/delivered to every sign-in since \d\d:\d\d$/);
    } finally {
      vi.useRealTimers();
    }
  });

  it("says there was nothing to check when nobody signed in", () => {
    q.value = { ...q.value, data: status({ sign_ins_24h: { unchecked: 3 } }) };
    render(<ClaimsLine />);
    expect(line()).toHaveTextContent("no sign-ins to check in the last 24h");
  });

  it("claims nothing about health while loading", () => {
    q.value = { data: undefined, error: null, isLoading: true, dataUpdatedAt: 0 };
    render(<ClaimsLine />);
    expect(line()).toHaveTextContent("checking");
    expect(line().textContent).not.toMatch(/delivered|problem|\d/);
  });

  it("says one quiet line on a failed read, with no state", () => {
    q.value = { data: undefined, error: new Error("boom"), isLoading: false, dataUpdatedAt: 0 };
    render(<ClaimsLine />);
    expect(line()).toHaveTextContent("Couldn't check how delivery is going.");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("keeps the last answer, with its age, when a later read fails", () => {
    q.value = { data: status(), error: new Error("boom"), isLoading: false, dataUpdatedAt: Date.now() };
    render(<ClaimsLine />);
    expect(line()).toHaveTextContent(/delivered to every sign-in in the last 24h · as of \d\d:\d\d/);
  });
});
