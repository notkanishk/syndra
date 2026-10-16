import { describe, expect, it } from "vitest";

import { fromParams, sentence, sentenceText, suggest, toParams, type Entity, type Question } from "@/components/insights/question";

const E: Entity[] = [
  { id: "u1", kind: "person", label: "Priya Nair" },
  { id: "u2", kind: "person", label: "Aanya Rao" },
  { id: "role:p1:door", kind: "role", label: "Laser Lab · Door" },
  { id: "role:p1:op", kind: "role", label: "Laser Lab · Operator" },
  { id: "role:p2:welder", kind: "role", label: "Metal shop · Welder" },
  { id: "bundle:b1", kind: "bundle", label: "Lab Tech" },
  { id: "bundle:b2", kind: "bundle", label: "Safety basics" },
  { id: "project:p1", kind: "project", label: "Laser Lab" },
  { id: "application:a1", kind: "application", label: "Badge reader" },
];

const QUESTIONS: Question[] = [
  { kind: "follow", node: "role:p1:door" },
  { kind: "path", person: "u1", role: "role:p1:door" },
  { kind: "compare", person: "u1", other: "u2", role: "role:p2:welder" },
  { kind: "who", has: "role:p1:door", without: "bundle:b2" },
  { kind: "who", has: "bundle:b1", without: null },
  { kind: "gone", bundle: "bundle:b1" },
  { kind: "gives", bundle: "bundle:b1", role: "role:p1:op" },
];

describe("the question in the link", () => {
  it.each(QUESTIONS)("round-trips %o", (q) => {
    const params = new URLSearchParams(Object.entries(toParams(q)).filter((e): e is [string, string] => e[1] != null));
    expect(fromParams((k) => params.get(k))).toEqual(q);
  });

  it("keeps a follow as the bare id, so older links still open", () => {
    expect(toParams({ kind: "follow", node: "role:p1:door" })).toMatchObject({ q: "role:p1:door", person: null, role: null });
    expect(fromParams((k) => ({ q: "project:p1" })[k] ?? null)).toEqual({ kind: "follow", node: "project:p1" });
  });

  it("clears every part when there is no question", () => {
    expect(Object.values(toParams(null)).every((v) => v === null)).toBe(true);
  });

  it("is no question when a part is missing", () => {
    expect(fromParams((k) => ({ q: "path", person: "u1" })[k] ?? null)).toBeNull();
    expect(fromParams((k) => ({ q: "compare", person: "u1", role: "r" })[k] ?? null)).toBeNull();
    expect(fromParams((k) => ({ q: "gives", bundle: "b" })[k] ?? null)).toBeNull();
    expect(fromParams(() => null)).toBeNull();
  });
});

describe("the sentence", () => {
  it("writes each question as the board does", () => {
    const text = (q: Question) =>
      sentenceText(sentence(q, (id) => E.find((e) => e.id === id)!.label, (id) => E.find((e) => e.id === id)!.label));
    expect(QUESTIONS.map(text)).toEqual([
      "Follow Laser Lab · Door",
      "Path from Priya Nair to Laser Lab · Door",
      "Compare Priya Nair with Aanya Rao on Metal shop · Welder",
      "Who has Laser Lab · Door without Safety basics",
      "Who has Lab Tech",
      "What if Lab Tech went",
      "What if Lab Tech also gave Laser Lab · Operator",
    ]);
  });
});

describe("what typing offers", () => {
  it("offers nothing for nothing, and nothing for a name that matches nothing", () => {
    expect(suggest("", E)).toEqual([]);
    expect(suggest("zzz", E)).toEqual([]);
  });

  it("offers to follow what matches, best first, and knows a bundle can go", () => {
    expect(suggest("badge", E)[0]).toEqual({ kind: "follow", node: "application:a1" });
    const lab = suggest("lab tech", E);
    expect(lab[0]).toEqual({ kind: "follow", node: "bundle:b1" });
    expect(lab).toContainEqual({ kind: "who", has: "bundle:b1", without: null });
    expect(lab).toContainEqual({ kind: "gone", bundle: "bundle:b1" });
  });

  it("matches words that start a word of the name, in any order of punctuation", () => {
    expect(suggest("laser door", E)[0]).toEqual({ kind: "follow", node: "role:p1:door" });
  });

  it("offers a path from a person to a role", () => {
    expect(suggest("priya to door", E)[0]).toEqual({ kind: "path", person: "u1", role: "role:p1:door" });
  });

  it("offers a compare for two people on a role", () => {
    expect(suggest("compare priya with aanya on welder", E)[0]).toEqual({
      kind: "compare",
      person: "u1",
      other: "u2",
      role: "role:p2:welder",
    });
  });

  it("offers who has one without the other, led by the word without", () => {
    expect(suggest("who has door without safety", E)[0]).toEqual({
      kind: "who",
      has: "role:p1:door",
      without: "bundle:b2",
    });
  });

  it("offers what if a bundle went, led by the word went", () => {
    expect(suggest("what if lab tech went", E)[0]).toEqual({ kind: "gone", bundle: "bundle:b1" });
  });

  it("offers what if a bundle also gave a role, led by gave", () => {
    const [first] = suggest("what if lab tech also gave operator", E);
    expect(first).toEqual({ kind: "gives", bundle: "bundle:b1", role: "role:p1:op" });
  });

  it("does not offer a path whose second name is not a role", () => {
    expect(suggest("priya to safety", E).some((q) => q.kind === "path")).toBe(false);
  });
});
