"""The recorder's --methods mode: capture the version check's baseline.

Run by scripts/record-truenas-fixtures.sh --methods inside a pinned container
(design-version-check.md §4.1, slice 9a). Usage: record-truenas-methods.py
<compat_deps.go>. Writes `{version, methods}` to stdout; says what it saw on
stderr. Its own file so the calls it can make are the three below and no
other: auth.login_with_api_key, system.version, core.get_methods.

  - Keeps only the methods in `deps` and `elevatedDeps` and only `accepts`,
    `returns`, `job`, with `description` and `examples` dropped at every depth.
  - Asks per service (the `service` filter, as the add-on will) and once
    unfiltered, and prints each reply's size and time: 9a's measurement.
  - The elevated key, if the first line of stdin holds one, opens one more
    session for one call, `core.get_methods` with `service: "user"`, and only
    `elevatedDeps` is taken from it (design §3.1). It is never stored.
  - Prints no address, no key and no exception text, which can carry either.
"""
import asyncio, json, os, re, ssl, sys, time

ENTRY = ("accepts", "returns", "job")
PROSE = {"description", "examples"}
NAMES = {"properties", "$defs", "patternProperties"}  # keys are names, not keywords
VALUES = {"default", "enum", "const"}  # values, never walked as schemas


def wanted(deps_go):
    """The method names in deps and in elevatedDeps, in that order."""
    tables = []
    for table in ("deps", "elevatedDeps"):
        m = re.search(r"^var %s = map\[string\]compat\.Dep\{\n(.*?)^\}" % table, deps_go, re.S | re.M)
        names = re.findall(r'^\t"([a-z0-9_.]+)":', m.group(1), re.M) if m else []
        if not names:
            raise SystemExit(f"error: no {table} table in compat_deps.go")
        tables.append(names)
    return tables


def services(names):
    return sorted({n.rsplit(".", 1)[0] for n in names})


def strip(s):
    if isinstance(s, list):
        return [strip(x) for x in s]
    if not isinstance(s, dict):
        return s
    out = {}
    for k, v in s.items():
        if k in PROSE:
            continue
        if k in NAMES and isinstance(v, dict):
            out[k] = {n: strip(x) for n, x in v.items()}
        else:
            out[k] = v if k in VALUES else strip(v)
    return out


def project(reply, names):
    """The listed methods' accepts, returns and job, as the NAS answered."""
    if not isinstance(reply, dict):
        raise SystemExit("error: core.get_methods did not answer a map of entries")
    out = {}
    for name in names:
        e = reply.get(name)
        if e is None:
            continue
        if not (isinstance(e, dict) and isinstance(e.get("accepts"), list)
                and isinstance(e.get("returns"), list) and isinstance(e.get("job"), bool)):
            raise SystemExit(f"error: {name} is not an entry with accepts, returns and job")
        out[name] = {k: strip(e[k]) for k in ENTRY}
    return out


def say(*a):
    print(*a, file=sys.stderr, flush=True)


async def session(connect, key, calls):
    """One login, then calls in order. Returns each reply's result, and says
    each core.get_methods reply's size and time."""
    async with connect() as ws:
        n = 0

        async def rpc(method, params):
            nonlocal n
            n += 1
            start = time.monotonic()
            await ws.send(json.dumps({"jsonrpc": "2.0", "id": n, "method": method, "params": params}))
            while True:
                raw = await ws.recv()
                msg = json.loads(raw)
                if msg.get("id") == n:
                    break
            if "error" in msg:
                raise SystemExit(f"error: {method} refused (code {msg['error'].get('code')})")
            if method == "core.get_methods":
                size = len(raw.encode() if isinstance(raw, str) else raw)
                say(f"    core.get_methods service={params[0] if params else '(all)'}: "
                    f"{size} bytes, {time.monotonic() - start:.2f} s, {len(msg['result'] or {})} methods")
            return msg["result"]

        if await rpc("auth.login_with_api_key", [key]) is not True:
            raise SystemExit("error: login refused")
        return [await rpc(m, p) for m, p in calls]


async def capture(connect, key, elevated_key, deps, elevated):
    say("==> the add-on's key")
    version, *replies = await session(connect, key, [("system.version", [])] +
                                      [("core.get_methods", [s]) for s in services(deps)] +
                                      [("core.get_methods", [])])
    if not (isinstance(version, str) and re.fullmatch(r"TrueNAS-[0-9][0-9A-Za-z.-]*", version)):
        raise SystemExit("error: system.version answered no TrueNAS version")
    say(f"    version {version}")
    whole, per = replies[-1], {}
    for r in replies[:-1]:
        per.update(r if isinstance(r, dict) else {})
    methods = project(per, deps)
    if project(whole, deps) != methods:
        say("    note: the unfiltered reply differs from the per-service replies; kept per service")
    say(f"    lists: {' '.join(sorted(methods)) or '(none)'}")
    say(f"    does not list: {' '.join(sorted(set(deps) - set(methods))) or '(none)'}")
    if elevated_key:
        say("==> the elevated key: one session, one call")
        (reply,) = await session(connect, elevated_key, [("core.get_methods", ["user"])])
        got = project(reply, elevated)
        say(f"    lists: {' '.join(sorted(got)) or '(none)'}")
        methods.update(got)
    missing = sorted(set(deps + elevated) - set(methods))
    if missing:
        say(f"    absent from the capture: {' '.join(missing)}")
        if "user.delete" in missing:
            say("    user.delete absent: the purge stays held (design §3.1)")
    return {"version": version, "methods": dict(sorted(methods.items()))}


def main():
    import websockets

    url, key = os.environ["TRUENAS_URL"], os.environ["TRUENAS_API_KEY"]
    elevated_key = sys.stdin.readline().strip()
    deps, elevated = wanted(open(sys.argv[1]).read())
    # Verified unless turned off, as the add-on reads it (main.go).
    if os.environ.get("TRUENAS_VERIFY_TLS", "").strip().lower() in ("0", "f", "false"):
        say("warning: TRUENAS_VERIFY_TLS is off: the keys go over an unauthenticated TLS session")
        ctx = ssl._create_unverified_context()
    else:
        ctx = ssl.create_default_context()
    try:
        doc = asyncio.run(capture(lambda: websockets.connect(url, ssl=ctx, max_size=None),
                                  key, elevated_key, deps, elevated))
    except SystemExit:
        raise
    except Exception as e:  # its text can name the address
        raise SystemExit(f"error: the capture failed ({type(e).__name__})")
    json.dump(doc, sys.stdout, indent=1)
    print()


if __name__ == "__main__":
    main()
