import type { Lane } from "@/components/insights/model";

/**
 * The question the map is answering, as one value. It is what the seat shows
 * as chips, what the link keeps, and what picks the answer: there is no second
 * copy of it anywhere. A map node's id is also the reference the Insights reads
 * take (`role:<project>:<key>`, `bundle:<id>`), so the parts hold ids and nothing
 * else.
 */
export type Question =
  | { kind: "follow"; node: string }
  | { kind: "path"; person: string; role: string }
  | { kind: "compare"; person: string; other: string; role: string }
  | { kind: "who"; has: string; without: string | null }
  | { kind: "gone"; bundle: string }
  | { kind: "gives"; bundle: string; role: string };

export const PART_KEYS = ["person", "other", "role", "has", "without", "bundle"] as const;
const KINDS = ["path", "compare", "who", "gone", "gives"] as const;

/** The link's words for a question: every part cleared, then the ones it uses. */
export function toParams(q: Question | null): Record<string, string | null> {
  const out: Record<string, string | null> = { q: null };
  for (const k of PART_KEYS) out[k] = null;
  if (!q) return out;
  switch (q.kind) {
    case "follow":
      out.q = q.node;
      break;
    case "path":
      Object.assign(out, { q: "path", person: q.person, role: q.role });
      break;
    case "compare":
      Object.assign(out, { q: "compare", person: q.person, other: q.other, role: q.role });
      break;
    case "who":
      Object.assign(out, { q: "who", has: q.has, without: q.without });
      break;
    case "gone":
      Object.assign(out, { q: "gone", bundle: q.bundle });
      break;
    case "gives":
      Object.assign(out, { q: "gives", bundle: q.bundle, role: q.role });
      break;
  }
  return out;
}

/** The question a link holds; any other `q` is a thing being followed. */
export function fromParams(get: (key: string) => string | null): Question | null {
  const q = get("q");
  if (!q) return null;
  const part = (k: string) => get(k) || null;
  if (!(KINDS as readonly string[]).includes(q)) return { kind: "follow", node: q };
  const [person, other, role, has, without, bundle] = PART_KEYS.map(part);
  switch (q) {
    case "path":
      return person && role ? { kind: "path", person, role } : null;
    case "compare":
      return person && other && role ? { kind: "compare", person, other, role } : null;
    case "who":
      return has ? { kind: "who", has, without } : null;
    case "gone":
      return bundle ? { kind: "gone", bundle } : null;
    default:
      return bundle && role ? { kind: "gives", bundle, role } : null;
  }
}

/** The ids a question names, for checking they are still on the map. */
export function nodeIds(q: Question): string[] {
  switch (q.kind) {
    case "follow":
      return [q.node];
    case "path":
    case "compare":
      return [q.role];
    case "who":
      return q.without ? [q.has, q.without] : [q.has];
    case "gone":
      return [q.bundle];
    case "gives":
      return [q.bundle, q.role];
  }
}

/** The people a question names. */
export function personIds(q: Question): string[] {
  return q.kind === "path" ? [q.person] : q.kind === "compare" ? [q.person, q.other] : [];
}

export type Part = { word: string } | { token: string };

const w = (word: string): Part => ({ word });
const t = (token: string): Part => ({ token });

/** The question as the board writes it: quiet words, with the things named in chips. */
export function sentence(q: Question, node: (id: string) => string, person: (id: string) => string): Part[] {
  switch (q.kind) {
    case "follow":
      return [w("Follow"), t(node(q.node))];
    case "path":
      return [w("Path from"), t(person(q.person)), w("to"), t(node(q.role))];
    case "compare":
      return [w("Compare"), t(person(q.person)), w("with"), t(person(q.other)), w("on"), t(node(q.role))];
    case "who":
      return [w("Who has"), t(node(q.has)), ...(q.without ? [w("without"), t(node(q.without))] : [])];
    case "gone":
      return [w("What if"), t(node(q.bundle)), w("went")];
    case "gives":
      return [w("What if"), t(node(q.bundle)), w("also gave"), t(node(q.role))];
  }
}

/** The sentence as plain text, for the accessible name and the copy. */
export const sentenceText = (parts: Part[]) => parts.map((p) => ("word" in p ? p.word : p.token)).join(" ");

export interface Entity {
  id: string;
  kind: "person" | Lane;
  label: string;
}

const norm = (s: string) => s.toLowerCase().replace(/[\s·.,:;()_/\\-]+/g, " ").trim();

/** 0 is the best match; null is none. Every word typed must start a word of the name. */
function rank(label: string, phrase: string): number | null {
  const name = norm(label);
  const typed = norm(phrase);
  if (!typed) return null;
  if (name === typed) return 0;
  if (name.startsWith(typed)) return 1;
  const words = name.split(" ");
  return typed.split(" ").every((p) => words.some((x) => x.startsWith(p))) ? 2 : null;
}

const STOP = /\b(?:follow|path|compare|who|has|have|what|if|went|go|gone|also|gave|give|gives|from|to|with|without|on|and|the)\b/;

/**
 * The questions a typed phrase can ask, best first. Typing names things; a
 * phrase between the words "to", "with", "without" and the like is one name.
 * Nothing is guessed from meaning: a name that matches nothing asks nothing.
 */
export function suggest(text: string, entities: Entity[], max = 6): Question[] {
  const needle = norm(text);
  if (!needle) return [];
  const whole = entities.some((e) => rank(e.label, needle) != null);
  const phrases = whole ? [needle] : needle.split(STOP).map((p) => p.trim()).filter(Boolean);
  if (!phrases.length) return [];

  const pick = (phrase: string | undefined, kinds: Entity["kind"][], limit = 1): Entity[] =>
    !phrase
      ? []
      : entities
          .map((e) => ({ e, r: kinds.includes(e.kind) ? rank(e.label, phrase) : null }))
          .filter((x): x is { e: Entity; r: number } => x.r != null)
          .sort((a, b) => a.r - b.r || a.e.label.localeCompare(b.e.label))
          .slice(0, limit)
          .map((x) => x.e);

  const [p0, p1, p2] = phrases;
  const NODES: Entity["kind"][] = ["bundle", "role", "project", "application"];
  const HAS: Entity["kind"][] = ["role", "bundle"];

  const follows = pick(p0, NODES, phrases.length > 1 ? 1 : 4).map((e): Question => ({ kind: "follow", node: e.id }));
  const person = pick(p0, ["person"])[0];
  const other = pick(p1, ["person"])[0];
  const role2 = pick(p1, ["role"])[0];
  const role3 = pick(p2, ["role"])[0];
  const has = pick(p0, HAS)[0];
  const without = pick(p1, HAS)[0];
  const bundle = pick(p0, ["bundle"])[0];

  const compound: Question[] = [];
  if (person && other && role3) compound.push({ kind: "compare", person: person.id, other: other.id, role: role3.id });
  if (person && role2) compound.push({ kind: "path", person: person.id, role: role2.id });
  if (has && without && has.id !== without.id) compound.push({ kind: "who", has: has.id, without: without.id });
  if (bundle && role2) compound.push({ kind: "gives", bundle: bundle.id, role: role2.id });
  const single: Question[] = [];
  if (phrases.length === 1) {
    if (has) single.push({ kind: "who", has: has.id, without: null });
    if (bundle) single.push({ kind: "gone", bundle: bundle.id });
  }

  // The words typed lean the order: "without" asks Who, "also gave" and "went" ask What if.
  const lean = (q: Question) =>
    (q.kind === "who" && /\bwithout\b/.test(needle)) || (q.kind === "gives" && /\bgave\b|\bgives\b/.test(needle)) || (q.kind === "gone" && /\bwent\b|\bgone\b/.test(needle)) ? 0 : 1;
  const all = phrases.length > 1 ? [...compound, ...follows] : [...follows, ...single];
  // Array.prototype.sort is stable, so the rest keep their order.
  all.sort((x, y) => lean(x) - lean(y));
  return all.slice(0, max);
}
