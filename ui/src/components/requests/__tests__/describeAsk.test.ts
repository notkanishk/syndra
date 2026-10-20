import { describe, expect, it } from "vitest";

import { describeAsk } from "@/components/requests/Holding";
import type { AccessRequest } from "@/lib/queries/useRequests";

const base: AccessRequest = {
  id: "r1",
  requester_id: "u1",
  project_id: "p1",
  role_key: "operator",
  justification: "",
  duration_days: 60,
  status: "pending",
  created_at: "2026-10-01T00:00:00Z",
};

describe("describeAsk", () => {
  it("shows the end the ask named, not one counted from today", () => {
    const line = describeAsk({ ...base, asked_until: "2027-05-31T00:00:00Z" });
    expect(line).toContain("until 31 May 2027");
  });

  it("counts from now only when the ask named no end", () => {
    expect(describeAsk(base)).toMatch(/^For 60 days · until /);
  });
});
