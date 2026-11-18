import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

// A page's props to a client component are serialised into the HTML, so the
// Zitadel access token may be touched only by the two server files that need
// it: the callback writes it into the httpOnly cookie, the proxy reads it back
// through getAccessToken(). Anything else under app/ or components/ naming the
// token is one prop away from putting it in the page.
const SRC = resolve(__dirname, "..");
const ALLOWED: Record<string, string> = {
  "app/api/proxy/[...path]/route.ts": "reads it to send the backend bearer",
  "app/auth/callback/route.ts": "writes it into the signed cookie",
};

function sources(dir: string): string[] {
  return (readdirSync(join(SRC, dir), { recursive: true }) as string[])
    .filter((f) => /\.tsx?$/.test(f) && !/(__tests__|\.test\.)/.test(f))
    .map((f) => join(dir, f));
}

describe("the access token stays server-side", () => {
  it("only the allowed server files name the token or its accessor", () => {
    const offenders = [...sources("app"), ...sources("components")].filter(
      (f) => !(f in ALLOWED) && /[aA]ccessToken/.test(readFileSync(join(SRC, f), "utf8")),
    );
    expect(offenders).toEqual([]);
  });

  // lib/ too: a helper wrapping getAccessToken would hand the token to any
  // page that imports the helper, past the check above.
  it("only the proxy route calls getAccessToken, and only lib/session.ts defines it", () => {
    const callers = [...sources("app"), ...sources("components"), ...sources("lib")].filter((f) =>
      /\bgetAccessToken\b/.test(readFileSync(join(SRC, f), "utf8")),
    );
    expect(callers.sort()).toEqual(["app/api/proxy/[...path]/route.ts", "lib/session.ts"]);
  });
});
