// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PersonActivity } from "@/components/people/PersonActivity";
import type { AuditEntry } from "@/lib/queries/useAudit";
import { formatShortDate } from "@/lib/format";

/**
 * The date bug: an entry rendered "10 Sept 04:18" while the Audit page, for
 * the SAME instant, correctly showed 11 Sept. The day heading was keyed by
 * `created_at.slice(0, 10)` — the UTC calendar date — while the row's own
 * clock (`formatClock`) reads local time. Fixed by keying the heading with
 * `formatShortDate`, the same local-time source the clock reads against.
 */

const audit = vi.hoisted(() => ({ data: [] as AuditEntry[] }));

vi.mock("@/lib/queries/useAudit", () => ({
  useAuditEntries: () => ({ data: audit.data, isLoading: false, error: null, refetch: () => {} }),
}));
vi.mock("@/lib/queries/useTargets", () => ({
  useTargets: () => ({ data: [], isLoading: false, error: null }),
}));
vi.mock("@/components/names", () => ({ UserName: () => null }));

function entry(created_at: string): AuditEntry {
  return {
    id: "a1",
    actor_id: "u1",
    target_id: "-",
    action: "direct_grant.upserted",
    resource_id: "",
    created_at,
  };
}

function renderActivity() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PersonActivity userId="u1" name="Ada" />
    </QueryClientProvider>,
  );
}

// Fixed to a timezone ahead of UTC so the test reproduces the bug regardless
// of the machine it runs on: the instant below falls after local midnight but
// before UTC midnight, which is exactly the window the UTC-sliced key got
// wrong.
const ORIGINAL_TZ = process.env.TZ;
beforeEach(() => {
  process.env.TZ = "Asia/Kolkata";
  audit.data = [];
});
afterEach(() => {
  process.env.TZ = ORIGINAL_TZ;
});

describe("the day heading over an activity row", () => {
  it("groups by the local day the row's own clock reads, not the UTC date", () => {
    // 2026-09-10T20:00:00Z is 2026-09-11 01:30 in Asia/Kolkata (UTC+5:30).
    const iso = "2026-09-10T20:00:00Z";
    audit.data = [entry(iso)];
    renderActivity();

    // `formatShortDate` is the same function the heading renders through —
    // computing the expectation with it is what keeps this test honest. The
    // UTC-sliced bug would have rendered `formatShortDate("2026-09-10")`
    // instead — a different string, which this positive assertion already
    // rules out.
    expect(screen.getByText(formatShortDate(iso))).toBeInTheDocument();
  });
});
