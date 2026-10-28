// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import AuditPage from "@/app/audit/page";

const log = vi.hoisted(() => ({
  entries: [] as Array<{ id: string; actor_id: string; target_id: string; action: string; resource_id: string; created_at: string }>,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useAudit", () => ({
  useAuditPages: () => ({
    data: { pages: [log.entries] },
    isLoading: false,
    error: null,
    refetch: () => {},
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: () => {},
  }),
}));
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({ resolveUser: () => ({ value: null }) }),
}));
vi.mock("@/components/names", () => ({ UserName: () => <span>someone</span> }));
vi.mock("@/components/audit/EventLine", () => ({ EventSentence: () => <span>did something</span> }));
vi.mock("@/components/audit/TraceCell", () => ({ TraceCell: () => <span /> }));

const entry = (i: number, actor = "u1") => ({
  id: `a${i}`,
  actor_id: actor,
  target_id: "u2",
  action: "grant.create",
  resource_id: "r",
  created_at: new Date(Date.now() - i * 60_000).toISOString(),
});

afterEach(cleanup);

describe("Audit export", () => {
  it("says it exports every matching entry, and does: past the ten on screen, formulas defused", async () => {
    // The 7-day window is the default; entries a minute apart all sit inside it.
    log.entries = [entry(0, "=HYPERLINK(1)"), ...Array.from({ length: 29 }, (_, i) => entry(i + 1))];
    let blob: Blob | undefined;
    URL.createObjectURL = vi.fn((made: Blob) => ((blob = made), "blob:x"));
    URL.revokeObjectURL = vi.fn();
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    render(<AuditPage />);
    expect(screen.getAllByText("did something")).toHaveLength(10);
    fireEvent.click(screen.getByRole("button", { name: "Export every matching entry (CSV)" }));

    expect(blob).toBeDefined();
    const text = await new Promise<string>((resolve) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.readAsText(blob!);
    });
    await waitFor(() => expect(text.split("\n")).toHaveLength(31)); // header + all 30
    expect(text).toContain(`"'=HYPERLINK(1)"`);
    expect(text).not.toContain(`"=HYPERLINK(1)"`);
  });
});
