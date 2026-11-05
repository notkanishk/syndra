// @vitest-environment jsdom
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ApiError, LostReply } from "@/lib/api-client";
import { MAY_HAVE_GONE_THROUGH, describeFailure, outcomeFromError, stateLine } from "@/lib/outcome";

/**
 * A write that fails part-way must never say "Nothing was changed." Every
 * backend code that can fail after something was done is in
 * MAY_HAVE_GONE_THROUGH, with the place to check. This reads the handlers, so
 * a new 5xx code is a decision here: partial, or one of the reads and
 * before-any-write refusals below.
 */
const fault = (code: string) =>
  new ApiError(code === "ZITADEL_ERROR" ? 502 : 500, { error: code, message: "Something went wrong inside Syndra.", details: { request_id: "ref-test-0003" } }, "POST");

describe("a write that may have partly landed", () => {
  for (const [code, where] of Object.entries(MAY_HAVE_GONE_THROUGH)) {
    it(`${code} says where to check and never "Nothing was changed"`, () => {
      const error = fault(code);
      const o = outcomeFromError(error);
      expect(o.partial).toBe(where);
      const { container } = render(<ActionOutcome word="Not done" outcome={o} />);
      const text = container.textContent ?? "";
      expect(text).not.toMatch(/nothing was changed/i);
      expect(describeFailure(error)).not.toMatch(/nothing was changed/i);
      expect(stateLine(error)).toBe(`Some of this may have gone through. Check ${where} before trying again.`);
      expect(text).toContain(`Check ${where} before trying again.`);
      expect(text).toContain("ref-test-0003");
    });
  }

  it("a failed read of Zitadel is still a failed read: nothing was changed", () => {
    const read = new ApiError(502, { error: "ZITADEL_ERROR", message: "Zitadel gave an answer Syndra could not use." }, "GET");
    expect(outcomeFromError(read).partial).toBeUndefined();
    expect(stateLine(read)).toBe("Nothing was changed.");
  });

  it("the generic internal error is partial for a write only", () => {
    const read = new ApiError(500, { error: "INTERNAL_ERROR", message: "Something went wrong inside Syndra." }, "GET");
    expect(outcomeFromError(read).partial).toBeUndefined();
  });

  it("a lost reply on a write says where to check and never \"Nothing was changed\"", () => {
    for (const method of ["POST", "PUT", "PATCH", "DELETE"]) {
      const lost = new LostReply(new TypeError("Failed to fetch"), method);
      const o = outcomeFromError(lost);
      expect(o.kind).toBe("failed");
      expect(o.partial).toBe("this page after a reload");
      const { container, unmount } = render(<ActionOutcome word="Not done" outcome={o} />);
      expect(container.textContent).not.toMatch(/nothing was changed/i);
      expect(container.textContent).toContain("Check this page after a reload before trying again.");
      expect(describeFailure(lost)).toContain("Syndra's server didn't answer.");
      expect(describeFailure(lost)).not.toMatch(/nothing was changed/i);
      expect(stateLine(lost)).toContain("Some of this may have gone through.");
      unmount();
    }
  });

  it("the proxy's own 'didn't answer' on a write is the same, and on a read stays as it is", () => {
    const body = { error: "BACKEND_UNREACHABLE", message: "Syndra's server didn't answer." };
    expect(outcomeFromError(new ApiError(502, body, "POST")).partial).toBe("this page after a reload");
    expect(outcomeFromError(new ApiError(502, { ...body, error: "UPSTREAM_ERROR" }, "DELETE")).partial).toBe("this page after a reload");
    expect(outcomeFromError(new ApiError(502, body, "GET")).partial).toBeUndefined();
    expect(stateLine(new ApiError(502, body, "GET"))).toBe("Nothing was changed.");
  });

  it("a read that never got a reply is untouched", () => {
    const plain = new TypeError("Failed to fetch");
    expect(outcomeFromError(plain).partial).toBeUndefined();
    expect(stateLine(plain)).toBe("Nothing was changed.");
  });

  it("a refusal (4xx) of the same code is a refusal", () => {
    expect(outcomeFromError(new ApiError(409, { error: "DRAIN_ERROR", message: "No." }, "POST")).partial).toBeUndefined();
  });
});

// Codes at 5xx that are reads, plans, or refusals before anything is written.
// CREDENTIAL_NOT_SET is only answered when no record was committed and nothing
// dispatched (handlers/my_storage.go:566-579 after services/addonop/dispatch.go:253-297;
// a sent change is answered "indeterminate" at my_storage.go:507-513). PLAN_CLAIM_ERROR
// is a claim transaction that rolls back on any error (db/plans.go:640-660), before
// anything is applied. ANSWER_NOT_RECORDED can write, so it is mapped instead.
// ZITADEL_REFUSED is a role write Zitadel refused and the read-back confirmed
// absent, after which Syndra undid its own record (services/roles.go writeRole).
const NOTHING_WRITTEN = new Set([
  "ZITADEL_REFUSED",
  "ADDON_UNREACHABLE", "TARGET_CAPABILITIES_UNKNOWN", "CATALOGUE_UNREAD", "NOT_CONFIGURED",
  "ZITADEL_UNAVAILABLE", "BULK_REHEARSAL_ERROR", "FINDINGS_ERROR", "VIEW_ERROR", "TARGET_UNREACHABLE",
  "RESEND_CHECK_FAILED", "CREDENTIAL_NOT_SET", "INVENTORY_ERROR", "TARGET_UNREADABLE", "DORMANT_ERROR",
  "ZITADEL_NOT_CONFIGURED", "PLAN_NOT_RECORDED", "PLAN_CLAIM_ERROR", "ROLE_MEMBERS_ERROR", "INDICATORS_ERROR",
  "SERVER_ERROR", "NOT_AUDITED", "APPS_UNREADABLE", "DIRECTORY_UNAVAILABLE", "DIRECTORY_UNREACHABLE",
  "UNCONFIRMED_READ_FAILED", "PLAN_ERROR", "DB_ERROR", "LOOKUP_ERROR", "PROCESSING_FAULT",
]);

describe("every 5xx code in the handlers is decided", () => {
  const dir = join(__dirname, "..", "..", "..", "..", "backend", "internal", "handlers");
  const codes = new Set<string>();
  for (const f of readdirSync(dir)) {
    if (!f.endsWith(".go") || f.endsWith("_test.go")) continue;
    const text = readFileSync(join(dir, f), "utf8");
    for (const m of text.matchAll(/writeUpstream(?:Status)?\([^"\n]*"([A-Z_]+)"\)/g)) codes.add(m[1]);
    for (const m of text.matchAll(/jsonErrorResponse\(w, http\.Status(?:BadGateway|ServiceUnavailable|InternalServerError),\s*"([A-Z_]+)"/g)) codes.add(m[1]);
  }

  it("finds the codes", () => {
    expect(codes.size).toBeGreaterThan(20);
  });

  it("each is partial (with a place to check) or listed as writing nothing", () => {
    const undecided = [...codes].filter((c) => !(c in MAY_HAVE_GONE_THROUGH) && !NOTHING_WRITTEN.has(c));
    expect(undecided).toEqual([]);
  });
});
