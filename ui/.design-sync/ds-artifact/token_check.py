"""List token values in a bundle's _ds_bundle.css that differ from tokens.json.

usage: python3 -I token_check.py <_ds_bundle.css> <tokens.json>
"""
import json, re, sys

css, tokens = open(sys.argv[1]).read(), json.load(open(sys.argv[2]))
themes = [t["id"] for t in tokens["color"]["themes"]]


def norm(v):
    return re.sub(r"\s+", " ", v.strip().lower())


def expect(t, theme):
    v = t["value"]
    v = v if isinstance(v, str) else v.get(theme, v.get(themes[0]))
    return norm(re.sub(r"^\{(.+)\}$", r"var(--\1)", v))


# Declarations per theme block: dark is the :root default, light under [data-theme="light"].
blocks = {th: {} for th in themes}
for sel, body in re.findall(r"([^{}]+)\{([^{}]*)\}", css):
    th = "light" if 'data-theme="light"' in sel or "data-theme=light" in sel else (
        "dark" if re.search(r":root|data-theme=\"dark\"|:host", sel) else None)
    if not th:
        continue
    for n, v in re.findall(r"--([A-Za-z0-9_.-]+)\s*:\s*([^;]+);", body):
        blocks[th].setdefault(n, v)

found = 0
for fam, f in tokens.items():
    if fam == "type" or not isinstance(f, dict):
        continue
    for t in f.get("tokens", []):
        for th in (themes if fam in ("color", "shadow") else themes[:1]):
            got = blocks[th].get(t["name"]) or blocks[themes[0]].get(t["name"])
            if got is None:
                print(f"MISSING {fam} {t['name']} [{th}]"); found += 1
            elif norm(got) != expect(t, th) and not (fam not in ("color", "shadow") and norm(got) == norm(str(t["value"]))):
                print(f"CHANGED {fam} {t['name']} [{th}] tokens.json={expect(t, th)} css={norm(got)}"); found += 1
print("differences:", found)
