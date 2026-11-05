"""Convert a Claude Design / design-sync bundle folder into the Design System
"files" layout, the same way the old org's "Finish setting up" did.

usage: python3 -I ds_merge.py <bundle_dir> <tokens.json> <out_project_dir> [MOVES.md]

With MOVES.md (ui/.design-sync/MOVES.md, written by moves.mjs), each card's
README.md ends with its Moves and Phone lines.

Writes only derived files into <out_project_dir> (which should start as a copy
of the live backup's project/):
  components/<Comp>/preview.html, components/<Comp>/README.md
  components/<group>/<Comp>/*      (the two-level sources, as the backup keeps)
  components/bundle.js, components/bundle.css, components/index.d.ts
  _ds_bundle.js, _ds_bundle.css, _ds_manifest.json, _preview/*.js
"""
import json, os, re, shutil, sys

src, tokens_path, out = sys.argv[1:4]
moves = {}
if len(sys.argv) > 4:
    for row in open(sys.argv[4]).read().splitlines():
        cells = [c.strip() for c in row.strip().strip("|").split("|")]
        if len(cells) == 3 and cells[0] and not set(cells[0]) <= set("-: ") and cells != ["Card", "Moves", "Phone"]:
            moves[cells[0]] = cells[1:]
tokens = json.load(open(tokens_path))

# Every --name the page declares from tokens.json (craft.md: One source).
names = set()
for fam, v in tokens.items():
    if fam != "type" and isinstance(v, dict):
        names.update(t["name"] for t in v.get("tokens", []))
names.update("font-" + k for k in tokens["type"].get("families", {}))


def strip_tokens(css):
    return "".join(l for l in css.splitlines(True)
                   if not ((m := re.match(r"\s*--([A-Za-z0-9_.-]+)\s*:", l)) and m.group(1) in names))


def preview(html):
    # The frame preloads tokens.css, bundle.css, React and bundle.js.
    html = re.sub(r'^(<!-- @dsCard [^>]*?)viewport="(\d+)x(\d+)"', r"\1height=\3 width=\2", html, count=1)
    html = re.sub(r'\n[ \t]*<link rel="stylesheet" href="\.\./\.\./\.\./(styles|_ds_bundle)\.css">', "", html)
    html = re.sub(r'\n[ \t]*<script src="\.\./\.\./\.\./(_vendor/react|_vendor/react-dom|_ds_bundle)\.js"></script>', "", html)
    html = html.replace('src="../../../_preview/', 'src="../../_preview/')
    html = (html.replace("background:#fff}", "background:var(--ground);color:var(--ink)}", 1)
                .replace("border:1px solid #e5e7eb", "border:1px solid var(--line-strong)")
                .replace("color:#6b7280", "color:var(--muted)"))
    return html


def put(rel, data):
    p = os.path.join(out, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    mode = "wb" if isinstance(data, bytes) else "w"
    with open(p, mode) as f:
        f.write(data)


def read(rel, binary=False):
    with open(os.path.join(src, rel), "rb" if binary else "r") as f:
        return f.read()


comps = {}  # name -> group
for g in sorted(os.listdir(os.path.join(src, "components"))):
    gd = os.path.join(src, "components", g)
    if not os.path.isdir(gd):
        continue
    for c in sorted(os.listdir(gd)):
        if os.path.isdir(os.path.join(gd, c)):
            comps[c] = g

dts = []
for c in sorted(comps, key=str.lower):
    g = comps[c]
    base = f"components/{g}/{c}/"
    for f in sorted(os.listdir(os.path.join(src, base))):
        put(base + f, read(base + f, binary=True))
    put(f"components/{c}/preview.html", preview(read(base + c + ".html")))
    readme = read(base + c + ".prompt.md")
    if c in moves:
        readme = readme.rstrip("\n") + f"\n\n**Moves.** {moves[c][0]}\n\n**Phone.** {moves[c][1]}\n"
    put(f"components/{c}/README.md", readme)
    d = read(base + c + ".d.ts")
    dts.append(d.replace("import * as React from 'react';\n", "", 1).strip("\n"))

put("components/index.d.ts", "import * as React from 'react';\n\n" + "\n\n".join(dts) + "\n")
bundle, css = read("_ds_bundle.js"), read("_ds_bundle.css")
# Preview shim, right after the @ds-bundle header line (which must stay line 1).
head, _, body = bundle.partition("\n")
shim = (# ThemeProvider takes its first theme from localStorage "syndra-theme" (else the OS),
        # so it would undo the theme the preview frame set on <html>. Hand it that theme.
        # ponytail: writes the key on every load; drop once ThemeProvider honours an existing data-theme.
        "(function () { var t = document.documentElement.getAttribute('data-theme'); "
        "if (t) try { localStorage.setItem('syndra-theme', t); } catch (e) {} })();\n")
put("components/bundle.js", head + "\n" + shim + body)
put("_ds_bundle.js", bundle)
put("components/bundle.css", strip_tokens(css))
put("_ds_bundle.css", css)
# Width queries as @container syndra-frame: what a Design canvas's boards link (see Frame).
put("_ds_bundle.container.css", read("_ds_bundle.container.css"))
if os.path.exists(os.path.join(src, "_ds_manifest.json")):
    put("_ds_manifest.json", read("_ds_manifest.json"))
for f in sorted(os.listdir(os.path.join(src, "_preview"))):
    put("_preview/" + f, read("_preview/" + f, binary=True))
print(json.dumps({"components": len(comps), "groups": sorted(set(comps.values()))}))
