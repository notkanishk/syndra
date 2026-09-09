import { describe, expect, it, vi } from "vitest";

import TargetPage from "@/app/system/targets/[target]/page";

/**
 * (1) `/system/targets/zitadel` and `/zitadel` used to give two different
 * accounts of the same fact — one computed from the add-on registry, which
 * has never heard of Zitadel and always answers "not registered", read on
 * this page as "not answering"; the other computed from the real Management
 * client. Zitadel is never a registered add-on, so this route has nothing
 * true to say about it — it defers to the one page that does.
 */
vi.mock("next/navigation", () => ({ redirect: vi.fn() }));

describe("the per-target route defers Zitadel to its own page", () => {
  it("redirects /system/targets/zitadel to /zitadel instead of rendering it as an add-on", async () => {
    const { redirect } = await import("next/navigation");
    await TargetPage({ params: Promise.resolve({ target: "zitadel" }) });
    expect(redirect).toHaveBeenCalledWith("/zitadel");
  });

  it("leaves a real add-on target alone", async () => {
    const { redirect } = await import("next/navigation");
    vi.mocked(redirect).mockClear();
    const element = await TargetPage({ params: Promise.resolve({ target: "truenas" }) });
    expect(redirect).not.toHaveBeenCalled();
    expect(element).toBeTruthy();
  });
});
