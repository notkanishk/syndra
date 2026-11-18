# The Design System artifact's tokens.css, generated from its tokens.json in the generator format the
# live file uses, plus the phone steps (BRIDGE step 2: tokens.json and tokens.css change together).
# usage: python3 -I .design-sync/ds-artifact/tokens_css.py <tokens.json> <tokens.css>
import json, sys
t = json.load(open(sys.argv[1]))
val = lambda v: f"var(--{v[1:-1]})" if isinstance(v, str) and v.startswith("{") else v
themes = [x["id"] for x in t["color"]["themes"]]
themed = [x for f in ("color", "shadow") for x in t[f]["tokens"]]
out = ["/* Syndra Design System — generated from tokens.json */"]
for th in themes:
    out.append(':root, [data-theme="dark"] {' if th == "dark" else f'[data-theme="{th}"] {{')
    out += [f"  --{x['name']}: {val(x['value'][th] if isinstance(x['value'], dict) else x['value'])};" for x in themed]
    out.append("}")
out.append(":root {")
for f, v in t.items():
    if f in ("color", "shadow", "type") or not isinstance(v, dict) or "tokens" not in v:
        continue
    out += [f"  --{x['name']}: {val(x['value'])};" for x in v["tokens"]]
out += [f"  --font-{k}: {v};" for k, v in t["type"]["families"].items()]
out.append("}")
# The phone step (globals.css @layer base, below the tablet breakpoint): the density a phone keeps.
# ponytail: phone values restated from globals.css (design-system.test.ts holds them); read them from the CSS if they start to drift.
PHONE = {"region-gap": "32px", "region-head-gap": "14px", "card-head-y": "16px", "row-y": "14px", "row-gap": "18px", "empty-y": "32px"}
out.append("@media (max-width: 44.99rem) {\n  :root {\n" + "\n".join(f"    --{k}: {v};" for k, v in PHONE.items()) + "\n  }\n}")
for g in t["type"]["groups"]:
    for s in g["styles"]:
        d = [f"font-family: var(--font-{s.get('family', g['family'])})", f"font-size: {s['fontSize']}", f"font-weight: {s['fontWeight']}"]
        if "lineHeight" in s: d.append(f"line-height: {s['lineHeight']}")
        if "letterSpacing" in s: d.append(f"letter-spacing: {s['letterSpacing']}")
        out.append(f".{s['name']} {{ " + "; ".join(d) + "; }")
# The display steps a phone takes (after the classes, so they win on source order).
PHONE_TYPE = {"greeting": "28px", "page-title": "26px", "card-title": "19px", "empty-title": "19px"}
out.append("@media (max-width: 44.99rem) {\n" + "\n".join(f"  .{k} {{ font-size: {v}; }}" for k, v in PHONE_TYPE.items()) + "\n}")
for f in t["type"]["fonts"]:
    out.append(f"@font-face {{ font-family: '{f['family']}'; src: url('{f['file']}') format('woff2'); font-weight: {f['weight']}; font-style: {f['style']}; font-display: swap; }}")
open(sys.argv[2], "w").write("\n".join(out) + "\n")
