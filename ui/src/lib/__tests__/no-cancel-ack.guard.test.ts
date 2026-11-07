import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * The cancel-ack header is gone (architecture deepening, task 1.2b): a write
 * names the queued changes it was shown in its own `cancel_ids` (a planned
 * act's plan, or `?cancel_ids=` for a write not on the planner). An ambient
 * header armed for "the next request to this path" is what that replaced, and
 * it must not come back under any name it had.
 */
const SRC = join(__dirname, "..", "..");
const BANNED = [/x-syndra-cancel-ack/i, /\barmAck\b/, /\btakeAck\b/, /\bCANCEL_ACK_HEADER\b/];

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) {
      if (entry !== "node_modules") walk(p, out);
    } else if (/\.(ts|tsx)$/.test(p) && p !== __filename) out.push(p);
  }
  return out;
}

describe("no cancel-ack header", () => {
  it("is named nowhere under ui/src", () => {
    const found = walk(SRC).flatMap((file) => {
      const text = readFileSync(file, "utf8");
      return BANNED.filter((re) => re.test(text)).map((re) => `${relative(SRC, file)}: ${re}`);
    });
    expect(found).toEqual([]);
  });
});
