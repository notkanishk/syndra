import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect } from "vitest";

/**
 * Reads the tokens of globals.css the way the contrast tests need them: a
 * theme's variables, a token's colour (alpha kept), one colour over another,
 * and the WCAG 2 ratio. Shared so a second test of a token is not a second
 * copy of the parser.
 */
export const SRC = resolve(__dirname, "..");
export const css = readFileSync(resolve(SRC, "app/globals.css"), "utf8");

export type RGBA = [number, number, number, number];

export function block(prelude: string): Record<string, string> {
  const start = css.indexOf(prelude);
  expect(start, `${prelude} not found in globals.css`).toBeGreaterThan(-1);
  const open = css.indexOf("{", start);
  let depth = 0;
  let end = open;
  for (let i = open; i < css.length; i += 1) {
    if (css[i] === "{") depth += 1;
    else if (css[i] === "}" && --depth === 0) {
      end = i;
      break;
    }
  }
  const vars: Record<string, string> = {};
  const body = css.slice(open + 1, end).replace(/\/\*[\s\S]*?\*\//g, "");
  for (const m of Array.from(body.matchAll(/(--[\w-]+):\s*([^;]+);/g))) vars[m[1]] = m[2].trim();
  return vars;
}

export function colour(vars: Record<string, string>, name: string): RGBA {
  let v = vars[name];
  while (v?.startsWith("var(")) v = vars[v.slice(4, -1).trim()];
  expect(v, `${name} has no value`).toBeDefined();
  const hex = /^#([0-9a-f]{6})$/i.exec(v);
  if (hex) return [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16)).concat(1) as RGBA;
  const rgba = /^rgba?\(([^)]+)\)$/.exec(v);
  expect(rgba, `${name}: ${v} is not a colour this test reads`).not.toBeNull();
  const [r, g, b, a = "1"] = rgba![1].split(",").map((x) => x.trim());
  return [Number(r), Number(g), Number(b), Number(a)];
}

export const over = (top: RGBA, under: RGBA): RGBA =>
  [0, 1, 2].map((i) => top[3] * top[i] + (1 - top[3]) * under[i]).concat(1) as RGBA;

export function luminance([r, g, b]: RGBA): number {
  const lin = (c: number) => (c / 255 <= 0.04045 ? c / 255 / 12.92 : ((c / 255 + 0.055) / 1.055) ** 2.4);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

export function ratio(text: RGBA, ground: RGBA): number {
  const [hi, lo] = [luminance(over(text, ground)), luminance(ground)].sort((a, b) => b - a);
  return (hi + 0.05) / (lo + 0.05);
}

