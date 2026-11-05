import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import ts from "typescript";
import { describe, expect, it } from "vitest";

/**
 * G14 / A-20 P1. A name part links by default, so a name part may never sit
 * inside another link or interactive element (a link in a link is invalid HTML
 * and two tab stops for one row). Where the container is the link, the name
 * says `link={false}`.
 */
const SRC = join(__dirname, "..", "..", "..");
const NAME_PARTS = new Set(["UserName", "ProjectName", "BundleName", "RoleRef", "ResolvedProjectName"]);
const INTERACTIVE_TAGS = new Set(["a", "Link", "NextLink", "ArrowLink", "button", "Button", "summary", "select", "label"]);
// `Capped` takes the "see all" href for its footer; its rows are plain.
const PLAIN_CONTAINERS = new Set(["Capped"]);
const INTERACTIVE_ATTRS = new Set(["href", "onClick", "onKeyDown", "onPointerDown"]);

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const p = join(dir, entry);
    if (statSync(p).isDirectory()) {
      if (entry === "__tests__" || entry === "node_modules") continue;
      walk(p, out);
    } else if (p.endsWith(".tsx") && !p.endsWith(".test.tsx")) out.push(p);
  }
  return out;
}

function tagOf(node: ts.Node): string | undefined {
  if (ts.isJsxElement(node)) return node.openingElement.tagName.getText();
  if (ts.isJsxSelfClosingElement(node)) return node.tagName.getText();
  return undefined;
}

function attrsOf(node: ts.Node): ts.JsxAttributes | undefined {
  if (ts.isJsxElement(node)) return node.openingElement.attributes;
  if (ts.isJsxSelfClosingElement(node)) return node.attributes;
  return undefined;
}

function hasAttr(attrs: ts.JsxAttributes | undefined, names: Set<string>): boolean {
  return (attrs?.properties ?? []).some((p) => ts.isJsxAttribute(p) && names.has(p.name.getText()));
}

function isLinkOff(attrs: ts.JsxAttributes | undefined): boolean {
  return (attrs?.properties ?? []).some(
    (p) =>
      ts.isJsxAttribute(p) &&
      p.name.getText() === "link" &&
      p.initializer !== undefined &&
      ts.isJsxExpression(p.initializer) &&
      p.initializer.expression?.getText() !== "true",
  );
}

function interactiveRole(attrs: ts.JsxAttributes | undefined): boolean {
  return (attrs?.properties ?? []).some(
    (p) => ts.isJsxAttribute(p) && p.name.getText() === "role" && /button|link|menuitem|tab|option/.test(p.initializer?.getText() ?? ""),
  );
}

export function nestedNames(source: string, file = "x.tsx"): string[] {
  const sf = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const found: string[] = [];
  const visit = (node: ts.Node) => {
    const tag = tagOf(node);
    if (tag && NAME_PARTS.has(tag) && !isLinkOff(attrsOf(node))) {
      for (let up: ts.Node | undefined = node.parent; up; up = up.parent) {
        const t = tagOf(up);
        if (!t) continue;
        const a = attrsOf(up);
        if (INTERACTIVE_TAGS.has(t) || (!PLAIN_CONTAINERS.has(t) && hasAttr(a, INTERACTIVE_ATTRS)) || interactiveRole(a)) {
          const { line } = sf.getLineAndCharacterOfPosition(node.getStart());
          found.push(`${file}:${line + 1} <${tag}> inside <${t}> — pass link={false}`);
          break;
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
  return found;
}

describe("names never nest inside another link or control", () => {
  it("the scan itself sees a name in a link, a button and a clickable row", () => {
    expect(nestedNames(`<Link href="/x"><UserName id="a" /></Link>`)).toHaveLength(1);
    expect(nestedNames(`<button><ProjectName id="a" /></button>`)).toHaveLength(1);
    expect(nestedNames(`<div onClick={f}><BundleName id="a" /></div>`)).toHaveLength(1);
    expect(nestedNames(`<Row href="/x"><RoleRef projectId="a" roleKey="b" /></Row>`)).toHaveLength(1);
  });

  it("the scan lets link={false} and a free-standing name through", () => {
    expect(nestedNames(`<Link href="/x"><UserName id="a" link={false} /></Link>`)).toHaveLength(0);
    expect(nestedNames(`<p><UserName id="a" /></p>`)).toHaveLength(0);
  });

  it("no name part in src renders inside an <a>, a button or a clickable container", () => {
    const bad = walk(SRC).flatMap((f) => nestedNames(readFileSync(f, "utf8"), relative(SRC, f)));
    expect(bad).toEqual([]);
  });
});
