import { describe, expect, it, vi } from "vitest";

vi.mock("next/headers", () => ({ cookies: vi.fn() }));
import { cookies } from "next/headers";
import { POST as login } from "@/app/auth/login/route";
import { POST as logout } from "@/app/auth/logout/route";

// SameSite=Lax still lets a sibling subdomain POST a form, so both sign-in and sign-out
// refuse a write that names another origin before they touch a cookie.
describe.each([
  ["login", login],
  ["logout", logout],
])("POST /auth/%s", (_name, handler) => {
  it("refuses a cross-origin form without touching the session", async () => {
    const res = await handler(
      new Request("http://localhost/auth/x", {
        method: "POST",
        headers: { origin: "https://evil.example" },
        body: new URLSearchParams({ userId: "u1" }),
      }),
    );
    expect(res.status).toBe(403);
    expect(cookies).not.toHaveBeenCalled();
  });
});
