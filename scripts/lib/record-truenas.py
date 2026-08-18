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
PROBE_USER = "syndra-fixture-probe"
FIELD = re.compile(r"^[A-Za-z0-9_.\[\]-]{1,120}$")


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
    async with websockets.connect(URL, ssl=ssl._create_unverified_context(), max_size=None) as ws:
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
            ("sharing.smb.query", [[], {"select": ["name", "audit"], "limit": 1}]),
        ):
            r = await rpc(method, params)
            if "error" in r:
                doc["reads"][method] = {"refused_fields": fields(r["error"]), "code": r["error"].get("code")}
                continue
            result = r["result"]
            sample = result[0] if isinstance(result, list) and result else result
            doc["reads"][method] = {"keys": sorted(sample.keys())} if isinstance(sample, dict) else {"type": type(sample).__name__}

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
