import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * The writing guide, checked rather than trusted.
 *
 * `openspec/changes/plain-language-copy/design.md` says every sentence Syndra
 * shows must be readable by makerspace staff with no identity-management
 * background, and lists the words that never appear on screen. Before this
 * test the list was prose: seven readers found ~730 places it was broken,
 * and the worst of them — "Sign in with Zitadel" on the door, "read null" on
 * a stale list, a hold that rendered as the word `true` — had each shipped
 * past a review. A rule about words is the easiest rule in the product to
 * check by machine, and it was the only one that was not.
 *
 * What counts as copy: JSX text, the string values of the props that carry
 * copy, and any string literal with spaces in it in `lib/` (where outcome
 * sentences and labels live) and in components (where sentences are often
 * assigned to a const before they are rendered). Class names are swept up by
 * that last rule and do no harm — none of the checks below fires on one.
 *
 * Exceptions are argued in `ARGUED`, one reason each. It is the record of
 * every place the product chose to break its own rule, and it is meant to be
 * read.
 */

const SRC = join(import.meta.dirname, "../../..");

/**
 * Words that never appear on screen (design.md §6, "Words that never appear
 * on screen"), as patterns. Each names the mechanism the reader is not
 * supposed to need; the plain replacement is beside it in the guide.
 */
const NEVER: Array<[RegExp, string]> = [
  [/\bdrain(s|ed|ing)?\b/i, "send"],
  [/\bresum(e|es|ed|ing)\b/i, "send"],
  [/\boutbox\b/i, "Pending changes"],
  [/\bledger\b/i, "record"],
  [/\bcascad(e|es|ed|ing)\b/i, "the changes this sets off"],
  [/\bpropagat/i, "send"],
  [/\breconcil/i, "check"],
  [/\bconverg/i, "bring accounts in line"],
  [/\bsweep(s|ing)?\b/i, "check"],
  [/\bdrift\b/i, "unexplained access"],
  [/\brehears/i, "preview"],
  [/\bupstream\b/i, "in Zitadel"],
  [/\bdownstream\b/i, "from Syndra"],
  [/identity provider/i, "Zitadel"],
  [/\bIdP\b/, "Zitadel"],
  [/\bthe provider\b/i, "Zitadel"],
  [/\bthe directory\b/i, "Zitadel"],
  [/\bsubject id\b/i, "person"],
  [/\bprincipal\b/i, "person"],
  [/\bentitlement/i, "access"],
  [/cache compile/i, "within about a minute"],
  [/\bhydrat/i, "load"],
  [/\bpayload\b/i, "details"],
  [/\bmutation/i, "change"],
  [/\bidempot/i, "—"],
  [/\bfixture/i, "sample data"],
  [/\bseeder\b/i, "sample data"],
  [/\bmanifest\b/i, "—"],
  [/\btruncated\b/i, "cut short"],
  [/\bdegraded\b/i, "could not be read"],
  [/\btriage/i, "review"],
  [/\bbinding(s)?\b/i, "mapping / account"],
  [/\bhops?\b/i, "step"],
  [/\bnodes?\b/i, "item"],
  [/\btargets?\b/i, "the system's name"],
  [/\b(a|the|this|new|stale) plan\b/i, "preview"],
  [/\bfir(e|es|ed|ing)\b/i, "applies"],
  [/\bqueued\b/i, "waiting to be sent"],
  [/\boperators?\b/i, "makerspace staff"],
  [/\bsteward/i, "makerspace staff"],
  [/\blab manager/i, "makerspace staff"],
  [/whoever runs the makerspace/i, "makerspace staff"],
  [/\busers?\b/i, "person / people"],
  [/\bdirect grants?\b/i, "direct access"],
  [/\b(expir\w+|standalone) grants?\b/i, "access"],
  [/\bwithdraw\w* (their |this |the )?access\b/i, "revoke"],
  [/\bremov\w* (their |this |the |direct |all )?access\b/i, "revoke"],
  [/\btak(e|es|en|ing) (it |this |them |that |the access |their access |access )?away\b/i, "revoke"],
  [/\bden(y|ied|ies)\b/i, "decline"],
  [/\bplease\b/i, "—"],
  [/\bsorry\b/i, "—"],
  [/\boops\b/i, "—"],
];

/**
 * Argued exceptions. Path prefix → the patterns allowed there, each with its
 * reason. A pattern listed here is permitted in that file and nowhere else.
 */
const ARGUED: Record<string, Array<[RegExp, string]>> = {
  "components/login/": [
    [
      /\boperators?\b/i,
      "Names a TEST identity's role in the development sign-in list (`mode === \"demo\"`), which no member or staff member ever reaches — the deployed door has one button and no identity list. It labels the audience a developer is about to impersonate, and 'Staff' would be the wrong word for that: the thing being picked is the operator view.",
    ],
  ],
};

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === "__tests__" || entry === "node_modules") continue;
      walk(full, out);
      continue;
    }
    if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) out.push(full);
  }
  return out;
}

function sources(): Array<{ path: string; source: string }> {
  return walk(join(SRC, "components"))
    .concat(walk(join(SRC, "app")))
    .concat(walk(join(SRC, "lib")))
    .map((path) => ({ path: path.slice(SRC.length + 1), source: readFileSync(path, "utf8") }));
}

interface Snippet {
  line: number;
  text: string;
}

function lineOf(source: string, index: number): number {
  return source.slice(0, index).split("\n").length;
}

/**
 * One pass over the source that knows where strings and comments are.
 *
 * Returns the code with every comment and string body blanked to spaces —
 * same length, so a line number computed on it is a line number in the file —
 * and every string literal's text with the index it started at. A template's
 * `${…}` is code, nested to any depth, and is blanked out of the literal's
 * text; the quotes inside it never start a string of their own. The regex
 * version of this read `"people"} would be repinned` as one literal, because
 * a quote inside an interpolation had desynchronised it, and reported code as
 * prose.
 *
 * A raw apostrophe in JSX text would desynchronise this too, and does not
 * occur: `react/no-unescaped-entities` refuses it, so the product writes ’.
 */
function tokenize(source: string): { code: string; strings: Snippet[] } {
  const code = source.split("");
  const strings: Snippet[] = [];
  // Each frame is a string being read, and the brace depth of the code
  // inside its current `${…}` (−1 while inside the literal itself).
  const frames: Array<{ quote: string; start: number; text: string; depth: number }> = [];
  const blank = (i: number) => {
    if (code[i] !== "\n") code[i] = " ";
  };
  let i = 0;
  while (i < source.length) {
    const c = source[i];
    const top = frames[frames.length - 1];
    if (top && top.depth < 0) {
      // Inside a literal.
      if (c === "\\") {
        top.text += source[i + 1] ?? "";
        blank(i);
        blank(i + 1);
        i += 2;
        continue;
      }
      if (c === top.quote) {
        frames.pop();
        strings.push({ line: lineOf(source, top.start), text: top.text });
        i += 1;
        continue;
      }
      if (top.quote === "`" && c === "$" && source[i + 1] === "{") {
        top.depth = 0;
        top.text += " ";
        blank(i);
        blank(i + 1);
        i += 2;
        continue;
      }
      top.text += c;
      blank(i);
      i += 1;
      continue;
    }
    // Code — at top level, or inside a template's `${…}`.
    if (c === "/" && source[i + 1] === "/") {
      const end = source.indexOf("\n", i);
      const stop = end < 0 ? source.length : end;
      for (let k = i; k < stop; k += 1) blank(k);
      i = stop;
      continue;
    }
    if (c === "/" && source[i + 1] === "*") {
      const end = source.indexOf("*/", i + 2);
      const stop = end < 0 ? source.length : end + 2;
      for (let k = i; k < stop; k += 1) blank(k);
      i = stop;
      continue;
    }
    if (c === '"' || c === "'" || c === "`") {
      frames.push({ quote: c, start: i, text: "", depth: -1 });
      i += 1;
      continue;
    }
    if (top) {
      if (c === "{") top.depth += 1;
      else if (c === "}") {
        if (top.depth === 0) {
          top.depth = -1;
          blank(i);
          i += 1;
          continue;
        }
        top.depth -= 1;
      }
    }
    i += 1;
  }
  return { code: code.join(""), strings };
}

/** True when a `>`…`<` fragment is code that happened to sit between two angle brackets. */
function looksLikeCode(text: string): boolean {
  return (
    /[;=`]|\b(import|const|let|var|return|export|function|interface|type|class|new|await)\b/.test(text) ||
    /[a-z]\.[a-z]{2,}|\s\?\s|\?\?|\?\.|^\s*[,:;)\]]/.test(text)
  );
}

/**
 * Every fragment of text a person could read, with the line it starts on.
 *
 * JSX text is anything between a `>` or `}` and the next `<` or `{` that
 * carries a letter and does not look like code — a text node, or the literal
 * half of one that has an interpolation in it. Every string literal with a
 * space in it is read as copy too: outcome sentences live in `lib/` as
 * literals, copy props take literals, and a sentence is often assigned to a
 * name before it is rendered. Class names are swept up by that rule and do
 * no harm — nothing below fires on one.
 */
function copyOf(source: string): Snippet[] {
  const { code, strings } = tokenize(source);
  const out: Snippet[] = [];
  const push = (line: number, text: string) => {
    const trimmed = text.replace(/\s+/g, " ").trim();
    if (/[A-Za-z]/.test(trimmed)) out.push({ line, text: trimmed });
  };
  for (const m of code.matchAll(/[>}]([^<>{}]*[A-Za-z][^<>{}]*)(?=[<{])/g)) {
    if (code[m.index] === ">" && code[m.index - 1] === "=") continue;
    if (looksLikeCode(m[1])) continue;
    push(lineOf(code, m.index + 1), m[1]);
  }
  // A URL path is not prose, however many interpolations it carries.
  for (const { line, text } of strings) {
    // A one-word literal is read too, when it is capitalised like a label.
    // Banned words live in exactly that shape — a badge, a filter option, a
    // column header — and the space rule alone could see none of them:
    // `{approved ? "Approved" : "Denied"}` sat in Requests through this
    // guard's own release. Lowercase single words stay out, because that is
    // what code strings look like (`"admin"`, `"oidc"`, a query key).
    const isLabel = /^[A-Z][a-z]+$/.test(text);
    const isProse = text.includes(" ") && !/^\s*[\/^?&]|^https?:|\w=/.test(text);
    if (isLabel || isProse) push(line, text);
  }
  return out;
}

/** Source with comments and string bodies blanked, same length. */
function stripComments(source: string): string {
  return tokenize(source).code;
}

function allowed(path: string, pattern: RegExp): boolean {
  return Object.entries(ARGUED).some(
    ([prefix, rules]) => path.startsWith(prefix) && rules.some(([p]) => p.source === pattern.source),
  );
}

describe("plain language", () => {
  it("uses no word from the never-on-screen list", () => {
    const offenders: string[] = [];
    for (const { path, source } of sources()) {
      for (const { line, text } of copyOf(source)) {
        for (const [pattern, instead] of NEVER) {
          if (!pattern.test(text) || allowed(path, pattern)) continue;
          offenders.push(`${path}:${line} — "${text.slice(0, 90)}" (say: ${instead})`);
        }
      }
    }
    expect(offenders, "design.md §6 lists the plain word for each").toEqual([]);
  });

  it("never raises its voice", () => {
    const offenders: string[] = [];
    for (const { path, source } of sources()) {
      for (const { line, text } of copyOf(source)) {
        if (/[A-Za-z]!(?=[\s"'`)]|$)/.test(text)) offenders.push(`${path}:${line} — "${text.slice(0, 90)}"`);
      }
    }
    expect(offenders, "no exclamation marks — the register is a university office").toEqual([]);
  });

  it("gives every page a lede", () => {
    const offenders: string[] = [];
    for (const { path, source } of sources()) {
      const clean = stripComments(source);
      for (const m of clean.matchAll(/<PageHeader\b/g)) {
        const tag = tagAt(clean, m.index);
        if (!/\blede=/.test(tag)) offenders.push(`${path}:${lineOf(clean, m.index)}`);
      }
    }
    expect(offenders, "what the page shows, when you come here, what inaction means").toEqual([]);
  });

  it("keeps sentences out of meta", () => {
    const offenders: string[] = [];
    for (const { path, source } of sources()) {
      const clean = stripComments(source);
      for (const m of clean.matchAll(/\bmeta=(?:"([^"]*)"|\{\s*[`"]([^`"]*)[`"]\s*\})/g)) {
        const text = m[1] ?? m[2] ?? "";
        if (/[.!?]\s|[a-z]\.$/.test(text) && text.split(" ").length > 5) {
          offenders.push(`${path}:${lineOf(clean, m.index)} — "${text.slice(0, 80)}"`);
        }
      }
    }
    expect(offenders, "meta is a count, an email, an id — the sentence goes in lede").toEqual([]);
  });

  it("names the object on every button", () => {
    const offenders: string[] = [];
    for (const { path, source } of sources()) {
      const clean = stripComments(source);
      for (const m of clean.matchAll(/>\s*(Dismiss|OK|Submit|Confirm|Go|Yes|No)\s*</g)) {
        offenders.push(`${path}:${lineOf(clean, m.index)} — "${m[1]}"`);
      }
      for (const m of clean.matchAll(/aria-label=(?:"|\{")(\w+)(?:"|"\})/g)) {
        offenders.push(`${path}:${lineOf(clean, m.index)} — aria-label="${m[1]}" says the verb and not its object`);
      }
    }
    expect(offenders, "read out of context, the label must still say what it does").toEqual([]);
  });

  it("carries no argued exception whose file is gone", () => {
    const paths = sources().map((s) => s.path);
    const gone = Object.keys(ARGUED).filter((prefix) => !paths.some((p) => p.startsWith(prefix)));
    expect(gone, "an exception for a deleted file is a permission granted to nobody").toEqual([]);
  });
});

/** The full text of a JSX tag starting at `at`, balancing braces and strings. */
function tagAt(source: string, at: number): string {
  let i = source.indexOf(" ", at);
  let depth = 0;
  let quote: string | null = null;
  while (i < source.length) {
    const c = source[i];
    if (quote) {
      if (c === "\\") i += 1;
      else if (c === quote) quote = null;
    } else if (c === '"' || c === "'" || c === "`") quote = c;
    else if (c === "{") depth += 1;
    else if (c === "}") depth -= 1;
    else if (c === ">" && depth === 0) break;
    i += 1;
  }
  return source.slice(at, i + 1);
}
