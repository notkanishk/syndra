// Writes ds-bundle/_ds_bundle.container.css: the same stylesheet as
// _ds_bundle.css, with every width @media rule turned into
// @container syndra-frame (...). The design canvas renders a board's frames
// inline, where @media sees the artboard's width, not the frame's (1440 or 390);
// each frame wrapper (and the board's <body>) is a `syndra-frame` container.
// Boards link this file INSTEAD of _ds_bundle.css. Hover, reduced-motion and
// colour-scheme queries stay @media. Run from ui/ after resync.
import { readFileSync, writeFileSync } from "node:fs";
import postcss from "postcss";

const src = "ds-bundle/_ds_bundle.css";
const out = "ds-bundle/_ds_bundle.container.css";
// One feature, width only: (width >= 45rem), (min-width: 40rem), (max-width: 480px).
const widthOnly = /^\(\s*(?:(?:min-|max-)?width\s*:|width\s*[<>]=?)[^()]*\)$/;
const widthAnywhere = /\b(?:min-|max-)?width\b/;

const root = postcss.parse(readFileSync(src, "utf8"), { from: src });
let rewritten = 0;
root.walkAtRules("media", (rule) => {
  const params = rule.params.trim();
  if (widthOnly.test(params)) {
    rule.name = "container";
    rule.params = `syndra-frame ${params}`;
    rewritten++;
  } else if (widthAnywhere.test(params)) {
    // A combined query (width plus something else) has no faithful one-line
    // rewrite; fail loudly so the next resync can't ship a half-converted file.
    throw new Error(`build-container-css: unhandled width query "@media ${params}"`);
  }
});
writeFileSync(out, root.toString());
console.log(`${out}: ${rewritten} width queries now @container syndra-frame`);
