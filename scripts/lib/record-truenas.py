"""Ask a real TrueNAS what it answers, and write it down.

Run by scripts/record-truenas-fixtures.sh inside a throwaway container. Records
two kinds of thing:

  reads       what the methods this add-on depends on actually return, so a
              parser is written against a real shape rather than an imagined one
  write_rules what the target REFUSES, and the schema field it names — the half
              no hand-written fixture has ever contained, and the half that hid
              a completely broken account creation behind two green suites

Refusals are recorded as the FIELD PATHS the middleware named, never its prose.
The prose is free text built from a call whose parameters can include a member's
password; a field path is a key of the request schema and cannot carry a value.
That is the same rule the add-on itself follows (nas.go, validationFields).
"""
import asyncio, json, os, re, ssl, sys

import websockets

URL = os.environ["TRUENAS_URL"]
KEY = os.environ["TRUENAS_API_KEY"]
WRITE = os.environ.get("WRITE") == "yes"

# Verified by default, and the same variable the add-on reads (main.go, envBool
# TRUENAS_VERIFY_TLS). This probe authenticates with the target's API key, so an
# unverified context hands that key to anyone who can answer for the NAS's
# address — the wss:// check in the wrapper stops the key travelling in clear,
# and stops nothing else. An unparseable value verifies: the fallback of a
# security switch is the safe side, never the convenient one.
VERIFY = os.environ.get("TRUENAS_VERIFY_TLS", "").strip().lower() not in ("0", "f", "false")
PROBE_USER = "syndra-fixture-probe"
# Handed in by the wrapper so a read-only run can carry the write rules forward.
PREVIOUS = os.environ.get("PREVIOUS_RECORDING", "")
FIELD = re.compile(r"^[A-Za-z0-9_.\[\]-]{1,120}$")


def jtype(v):
    """The JSON type of one value, as a decoder has to survive it."""
    if v is None:
        return "null"
    if isinstance(v, bool):
        return "bool"
    if isinstance(v, int):
        return "int"
    if isinstance(v, float):
        return "float"
    if isinstance(v, str):
        return "str"
    if isinstance(v, list):
        return "list"
    return "dict"


def shape(rows):
    """Keys AND types, unioned over every row sampled.

    Types are the half that was missing and the half that broke things:
    `audit.query` answers `message_timestamp` as an INTEGER, the add-on decoded
    it as a string, the whole read failed to unmarshal, and `activity.get`
    returned "the audit log could not be read" for the entire life of the
    add-on. A recording of key NAMES agreed with the code the whole time.

    A key that is null in one row and typed in another records the type, not
    the null: absent-or-a-string is a string as far as a decoder is concerned.
    """
    keys, types = set(), {}
    for row in rows:
        if not isinstance(row, dict):
            continue
        keys |= set(row.keys())
        for k, v in row.items():
            t = jtype(v)
            if t == "null":
                types.setdefault(k, "null")
            elif types.get(k) in (None, "null"):
                types[k] = t
            elif types[k] != t:
                # A key that is genuinely two types is recorded as both, so a
                # decoder written against one of them is written knowingly.
                types[k] = "|".join(sorted(set(types[k].split("|")) | {t}))
    return {"keys": sorted(keys), "types": dict(sorted(types.items()))}


def fields(err):
    """The schema paths a refusal named, and nothing else."""
    out = []
    for entry in (err.get("data") or {}).get("extra") or []:
        if isinstance(entry, list) and entry and isinstance(entry[0], str) and FIELD.match(entry[0]):
            out.append(entry[0])
    return out


async def main():
    doc = {
        "_comment": "Recorded from a real target by scripts/record-truenas-fixtures.sh. "
                    "Do not hand-edit: a fixture somebody can write by hand is a fixture "
                    "that can agree with the code and disagree with the target.",
        "reads": {},
        "write_rules": [],
    }
    # A read-only run keeps whatever refusals the last --write run recorded,
    # which is what this script's own usage text promises. Rebuilding the
    # document from scratch discarded them, so every read-only run silently
    # deleted the half that took a throwaway account to obtain.
    if not WRITE:
        try:
            doc["write_rules"] = json.loads(PREVIOUS).get("write_rules", [])
        except ValueError:
            pass
    if VERIFY:
        ctx = ssl.create_default_context()
    else:
        # Opted out explicitly, and said out loud. A self-signed NAS certificate
        # is the honest reason for this, and the operator is told what it costs
        # rather than finding out from the default.
        print("warning: TRUENAS_VERIFY_TLS is off — this sends TRUENAS_API_KEY over an "
              "unauthenticated TLS session. Record from a network you trust, or put a "
              "trusted certificate on the NAS.", file=sys.stderr)
        ctx = ssl._create_unverified_context()

    async with websockets.connect(URL, ssl=ctx, max_size=None) as ws:
        n = [0]

        async def rpc(method, params):
            n[0] += 1
            await ws.send(json.dumps({"jsonrpc": "2.0", "id": n[0], "method": method, "params": params}))
            while True:
                msg = json.loads(await ws.recv())
                if msg.get("id") == n[0]:
                    return msg

        login = await rpc("auth.login_with_api_key", [KEY])
        if "error" in login or login.get("result") is not True:
            print(json.dumps({"error": "login failed"}), file=sys.stderr)
            raise SystemExit(1)

        version = (await rpc("system.version", [])).get("result")
        doc["product_version"] = version
        doc["reads"]["system.version"] = version

        # Shapes, not contents: what a parser has to survive. Values that name
        # real people are not recorded — one row, keys only.
        for method, params in (
            ("auth.me", []),
            ("user.query", [[["builtin", "=", False]], {"select": ["id", "uid", "username", "locked", "smb", "password_disabled"], "limit": 1}]),
            ("group.query", [[], {"select": ["id", "gid", "group"], "limit": 1}]),
            # The union of both call sites' selects. `unauditedShares` asks for
            # name+audit and `shareUsage` asks for name+path+enabled, so a
            # recording of either one alone leaves the other unproven.
            ("sharing.smb.query", [[], {"select": ["name", "audit", "path", "enabled"], "limit": 1}]),
        ):
            r = await rpc(method, params)
            if "error" in r:
                doc["reads"][method] = {"refused_fields": fields(r["error"]), "code": r["error"].get("code")}
                continue
            result = r["result"]
            rows = result if isinstance(result, list) else [result]
            doc["reads"][method] = shape(rows) if any(isinstance(x, dict) for x in rows) \
                else {"type": jtype(result)}

        # The audit read behind `activity.get`. Recorded with the add-on's own
        # parameter shape rather than a convenient one: what has to hold is that
        # the target accepts THIS call, and a probe that asks differently proves
        # a different thing. The username filter is exercised separately against
        # a name that cannot exist, because an accepted filter returning nothing
        # and a refused filter are the same empty list from the outside.
        audit_params = [{
            "services": ["SMB"],
            "query-options": {"limit": 1, "order_by": ["-message_timestamp"]},
        }]
        audit_params[0]["query-options"]["limit"] = 200
        r = await rpc("audit.query", audit_params)
        if "error" in r:
            doc["reads"]["audit.query"] = {
                "refused_fields": fields(r["error"]), "code": r["error"].get("code")}
        else:
            rows = [x for x in (r["result"] or []) if isinstance(x, dict)]
            # Grouped BY EVENT TYPE, because SMB writes several and they carry
            # different payloads: an AUTHENTICATION row has no share and a
            # CONNECT row does, so one sampled row proves whichever happened to
            # be newest and nothing about the field the add-on actually reads.
            per_event = {}
            for row in rows:
                ev = str(row.get("event", "?"))
                data = row.get("event_data")
                per_event.setdefault(ev, set())
                if isinstance(data, dict):
                    per_event[ev] |= set(data.keys())
            entry = dict(shape(rows), sampled_rows=len(rows))
            entry["event_data_keys_by_event"] = {k: sorted(v) for k, v in sorted(per_event.items())}
            entry["event_data_types"] = shape([r.get("event_data") for r in rows])["types"]
            filtered = await rpc("audit.query", [{
                "services": ["SMB"],
                "query-filters": [["username", "=", PROBE_USER]],
                "query-options": {"limit": 1, "order_by": ["-message_timestamp"]},
            }])
            entry["username_filter_accepted"] = "error" not in filtered
            if "error" in filtered:
                entry["username_filter_refused_fields"] = fields(filtered["error"])
            doc["reads"]["audit.query"] = entry

        # The quota read behind the member's storage page. Its argument is a
        # DATASET, derived by the add-on from a share's own path, so the probe
        # derives it the same way — a hand-picked dataset name would prove the
        # method answers and not that the derivation reaches it.
        shares = (await rpc("sharing.smb.query",
                            [[], {"select": ["name", "path", "enabled"]}])).get("result") or []
        dataset = ""
        for sh in shares:
            if sh.get("enabled") and str(sh.get("path", "")).startswith("/mnt/"):
                dataset = str(sh["path"])[len("/mnt/"):].strip("/")
                break
        if not dataset:
            doc["reads"]["pool.dataset.get_quota"] = {
                "skipped": "no enabled SMB share under /mnt to derive a dataset from"}
        else:
            r = await rpc("pool.dataset.get_quota", [dataset, "USER"])
            if "error" in r:
                doc["reads"]["pool.dataset.get_quota"] = {
                    "refused_fields": fields(r["error"]), "code": r["error"].get("code")}
            else:
                rows = [x for x in (r["result"] or []) if isinstance(x, dict)]
                # A key absent from every row is absent from the contract. The
                # union is what a decoder may rely on; one row is what it
                # happened to get.
                doc["reads"]["pool.dataset.get_quota"] = dict(
                    shape(rows), sampled_rows=len(rows),
                    quota_types=sorted({str(r.get("quota_type")) for r in rows}),
                )

        if WRITE:
            existing = (await rpc("user.query", [[["username", "=", PROBE_USER]], {"select": ["id"]}])).get("result") or []
            if existing:
                print(json.dumps({"error": f"{PROBE_USER} already exists; refusing to touch it"}), file=sys.stderr)
                raise SystemExit(1)

            async def rule(name, method, params):
                r = await rpc(method, params)
                entry = {"case": name, "method": method}
                if "error" in r:
                    entry["refused_fields"] = fields(r["error"])
                    entry["code"] = r["error"].get("code")
                else:
                    entry["accepted"] = True
                doc["write_rules"].append(entry)
                return r

            base = {"username": PROBE_USER, "full_name": PROBE_USER, "group_create": True, "locked": False}

            await rule("create with no credential decision", "user.create", [dict(base, smb=False)])
            await rule("create with smb and disabled password", "user.create",
                       [dict(base, smb=True, password_disabled=True)])
            created = await rule("create with password auth disabled", "user.create",
                                 [dict(base, smb=False, password_disabled=True)])

            rec = created.get("result")
            if isinstance(rec, dict) and "id" in rec:
                doc["write_rules"].append({"case": "user.create returns", "returns": "record" , "keys": sorted(rec.keys())[:12]})
                uid = rec["id"]
                await rule("password alone leaves auth disabled", "user.update", [uid, {"password": "Fixture-Passw0rd!"}])
                after = (await rpc("user.query", [[["username", "=", PROBE_USER]], {"select": ["password_disabled"]}])).get("result") or [{}]
                doc["write_rules"].append({"case": "password alone, resulting password_disabled",
                                           "password_disabled": after[0].get("password_disabled")})
                await rule("enable smb in a later call", "user.update", [uid, {"smb": True}])
                await rule("password, enable auth and smb together", "user.update",
                           [uid, {"password": "Fixture-Passw0rd!", "password_disabled": False, "smb": True}])
                await rpc("user.delete", [uid])

            left = (await rpc("user.query", [[["username", "=", PROBE_USER]], {"select": ["id"]}])).get("result") or []
            doc["probe_account_removed"] = not left

    json.dump(doc, sys.stdout, indent=2, sort_keys=False)
    print()

asyncio.run(main())
