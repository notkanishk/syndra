// Compiles src/app/globals.css through the app's own PostCSS/Tailwind pipeline,
// unchanged, so the synced stylesheet is exactly what the app ships: every
// token (incl. the --door-* block) plus the utilities the app's sources use.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import postcss from "postcss";
import tailwind from "@tailwindcss/postcss";

const from = "src/app/globals.css";
const out = ".design-sync/.cache/syndra.css";
const res = await postcss([tailwind({ base: process.cwd() })]).process(readFileSync(from, "utf8"), { from, to: out });
mkdirSync(".design-sync/.cache", { recursive: true });
// What the Next host provides at runtime (next/font variables, the body surface).
const fontVars = readFileSync(".design-sync/host.css", "utf8");
writeFileSync(out, `${res.css}\n${fontVars}`);
console.log(`wrote ${out} (${res.css.length} bytes)`);
