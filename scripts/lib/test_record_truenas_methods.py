"""Tests for the recorder's --methods mode (slice 9a). No NAS, no Docker.

Run: python3 -m unittest discover -s scripts/lib -p 'test_*.py'

The end-to-end tests run the real scripts/record-truenas-fixtures.sh in a copy
of the files the runbook copies, with a fake `docker` on PATH that runs the
mounted helper directly against a fake `websockets` serving a core.get_methods
reply built from the shipped capture, enriched with what the NAS adds.
"""
import copy, hashlib, importlib.util, json, os, pathlib, shutil, subprocess, sys, tempfile, textwrap, unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
CAPTURE = ROOT / "addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json"
DEPS = ROOT / "addons/truenas/compat_deps.go"

KEY = "FAKE-ADDON-KEY-1-wrestlemania"
ELEVATED = "FAKE-ELEVATED-KEY-2-royalrumble"
URL = "wss://nas.example.invalid/api/current"

spec = importlib.util.spec_from_file_location("methods", ROOT / "scripts/lib/record-truenas-methods.py")
methods = importlib.util.module_from_spec(spec)
spec.loader.exec_module(methods)


def stand_in():
    return json.loads(CAPTURE.read_text())


def rich_reply():
    """The shipped capture's entries as the NAS sends them: docstrings at every
    depth, the other entry keys, and methods outside the tables."""
    out = {}
    for name, entry in stand_in()["methods"].items():
        e = copy.deepcopy(entry)
        e.update(description="Docs for " + name, cli_description="cli", examples={"websocket": ["x"]},
                 roles=["READONLY_ADMIN"], filterable=False, item_method=False)
        for s in e["accepts"] + e["returns"]:
            s["description"] = "prose naming The Undertaker"
            s["examples"] = [{"home": "/mnt/tank/hulk-hogan", "host": "nas.example.invalid"}]
        out[name] = e
    out["vm.query"] = {"accepts": [], "returns": [{"type": "array"}], "job": False, "description": "vm"}
    out["user.shell_choices"] = {"accepts": [], "returns": [{"type": "object"}], "job": False}
    return out


class Projection(unittest.TestCase):
    def test_wanted_reads_both_tables(self):
        deps, elevated = methods.wanted(DEPS.read_text())
        self.assertEqual(elevated, ["user.delete"])
        self.assertNotIn("user.delete", deps)
        self.assertEqual(sorted(deps + elevated), sorted(stand_in()["methods"]))

    def test_wanted_refuses_a_file_without_the_tables(self):
        with self.assertRaises(SystemExit):
            methods.wanted("package main\n")

    def test_project_keeps_listed_methods_and_three_keys(self):
        got = methods.project(rich_reply(), ["user.query", "group.create", "not.listed"])
        self.assertEqual(sorted(got), ["group.create", "user.query"])
        for entry in got.values():
            self.assertEqual(sorted(entry), ["accepts", "job", "returns"])
        text = json.dumps(got)
        for word in ("description", "examples", "Undertaker", "Hogan", "example.invalid", "READONLY_ADMIN"):
            self.assertNotIn(word, text)

    def test_a_property_named_description_survives(self):
        reply = {"group.create": {"accepts": [{"type": "object", "properties": {
            "description": {"type": "string", "description": "prose"}}}], "returns": [{}], "job": False}}
        got = methods.project(reply, ["group.create"])
        self.assertEqual(got["group.create"]["accepts"][0]["properties"], {"description": {"type": "string"}})

    def test_a_reply_that_is_not_a_map_of_entries_is_refused(self):
        for bad in ([], {"user.query": []}, {"user.query": {"accepts": [], "job": False}},
                    {"user.query": {"accepts": {}, "returns": [], "job": False}},
                    {"user.query": {"accepts": [], "returns": [], "job": "no"}}):
            with self.assertRaises(SystemExit, msg=repr(bad)):
                methods.project(bad, ["user.query"])

    def test_services(self):
        self.assertEqual(methods.services(["pool.dataset.query", "pool.query", "user.create", "user.query"]),
                         ["pool", "pool.dataset", "user"])


FAKE_WEBSOCKETS = textwrap.dedent('''
    """A fake TrueNAS: answers only what the test lets it, and logs each method."""
    import json, os

    REPLY = json.load(open(os.environ["FAKE_REPLY"]))
    LOG = os.environ["FAKE_CALLS"]

    class _WS:
        def __init__(self):
            self.key, self.out = None, []
        async def __aenter__(self): return self
        async def __aexit__(self, *a): return False
        async def send(self, raw):
            m = json.loads(raw)
            with open(LOG, "a") as f:
                f.write(json.dumps([m["method"], m["params"] if m["method"] == "core.get_methods" else None]) + "\\n")
            r = None
            if m["method"] == "auth.login_with_api_key":
                self.key = m["params"][0]
                r = self.key in (os.environ["FAKE_KEY"], os.environ["FAKE_ELEVATED"])
            elif m["method"] == "system.version":
                r = "TrueNAS-25.10.7"
            elif m["method"] == "core.get_methods":
                svc = m["params"][0] if m["params"] else None
                r = {n: e for n, e in REPLY.items()
                     if (svc is None or n.rsplit(".", 1)[0] == svc)
                     and (n != "user.delete" or self.key == os.environ["FAKE_ELEVATED"])}
            self.out.append(json.dumps({"jsonrpc": "2.0", "id": m["id"], "result": r}))
        async def recv(self):
            return self.out.pop(0)

    def connect(url, **kw):
        if os.environ.get("FAKE_FAIL"):
            raise OSError(f"[Errno 61] Connect call failed {url}")
        return _WS()
''')

# The fake docker runs the helper the real command line mounts, with the mounts
# resolved, the same environment and stdin: everything but the container.
FAKE_DOCKER = textwrap.dedent('''
    #!/usr/bin/env python3
    import os, subprocess, sys
    args, mounts, i = sys.argv[1:], {}, 0
    while i < len(args):
        if args[i] == "-v":
            src, dst = args[i + 1].split(":")[:2]
            mounts[dst] = src
            i += 1
        i += 1
    cmd = args[-1].split("&&")[-1].split()
    os.execvp(sys.executable, [sys.executable] + [mounts.get(a, a) for a in cmd[1:]])
''').lstrip()


def tree(root):
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob("*")) if p.is_file()}


class EndToEnd(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.tmp)
        # The runbook's copy: scripts/ and the tables, nothing else.
        self.work = self.tmp / "work"
        shutil.copytree(ROOT / "scripts", self.work / "scripts",
                        ignore=shutil.ignore_patterns("__pycache__", "test_*"))
        (self.work / "addons/truenas").mkdir(parents=True)
        shutil.copy(DEPS, self.work / "addons/truenas/compat_deps.go")
        fakes = self.tmp / "fakes"
        fakes.mkdir()
        (fakes / "websockets.py").write_text(FAKE_WEBSOCKETS)
        (fakes / "docker").write_text(FAKE_DOCKER)
        (fakes / "docker").chmod(0o755)
        (self.tmp / "reply.json").write_text(json.dumps(rich_reply()))
        self.calls = self.tmp / "calls.log"
        self.env = dict(os.environ, PATH=f"{fakes}:{os.environ['PATH']}", PYTHONPATH=str(fakes),
                        TRUENAS_URL=URL, TRUENAS_API_KEY=KEY, FAKE_KEY=KEY, FAKE_ELEVATED=ELEVATED,
                        FAKE_REPLY=str(self.tmp / "reply.json"), FAKE_CALLS=str(self.calls))

    def run_recorder(self, *args, stdin=""):
        return subprocess.run(["bash", "scripts/record-truenas-fixtures.sh", *args], cwd=self.work,
                              env=self.env, input=stdin, capture_output=True, text=True, timeout=60)

    def out(self):
        return self.work / "addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json"

    def test_write_is_refused_before_connecting(self):
        for args in (["--methods", "--write"], ["--write", "--methods"]):
            before = tree(self.work)
            r = self.run_recorder(*args, stdin=ELEVATED + "\n")
            self.assertNotEqual(r.returncode, 0)
            self.assertIn("--write", r.stderr)
            self.assertFalse(self.calls.exists(), "it connected")
            self.assertEqual(tree(self.work), before)

    def test_capture_with_the_elevated_key(self):
        before = tree(self.work)
        r = self.run_recorder("--methods", stdin=ELEVATED + "\n")
        self.assertEqual(r.returncode, 0, r.stderr)
        after = tree(self.work)
        rel = str(self.out().relative_to(self.work))
        self.assertEqual(set(after) - {rel}, set(before), "touched a file other than the baseline")
        self.assertEqual({k: v for k, v in after.items() if k != rel}, before)

        doc = json.loads(self.out().read_text())
        want = stand_in()
        self.assertNotIn("_synthetic", doc)
        self.assertEqual(doc["version"], want["version"])
        # The shipped capture is this projection plus `_synthetic`, so what the
        # recorder writes is a file compat.Baselines() loads, entry for entry.
        self.assertEqual(doc["methods"], want["methods"])

        said = r.stdout + r.stderr
        for secret in (KEY, ELEVATED, URL, "nas.example.invalid"):
            self.assertNotIn(secret, said)
            self.assertNotIn(secret, self.out().read_text())
        self.assertIn("core.get_methods", said)
        self.assertIn("bytes", said)

        calls = [json.loads(l) for l in self.calls.read_text().splitlines()]
        self.assertEqual({c[0] for c in calls}, {"auth.login_with_api_key", "system.version", "core.get_methods"})
        # The elevated key opens one session and makes one call.
        logins = [i for i, c in enumerate(calls) if c[0] == "auth.login_with_api_key"]
        self.assertEqual(len(logins), 2)
        self.assertEqual(calls[logins[1] + 1:], [["core.get_methods", ["user"]]])
        got = [c[1] for c in calls if c[0] == "core.get_methods"]
        self.assertIn([], got, "the unfiltered form is not measured")
        self.assertIn(["pool.dataset"], got)

    def test_a_failure_names_no_address_and_writes_nothing(self):
        before = tree(self.work)
        self.env["FAKE_FAIL"] = "1"
        r = self.run_recorder("--methods", stdin=ELEVATED + "\n")
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("OSError", r.stderr)
        for secret in (KEY, ELEVATED, URL, "nas.example.invalid"):
            self.assertNotIn(secret, r.stdout + r.stderr)
        self.assertEqual(tree(self.work), before)

    def test_capture_without_the_elevated_key_leaves_user_delete_out(self):
        r = self.run_recorder("--methods", stdin="\n")
        self.assertEqual(r.returncode, 0, r.stderr)
        doc = json.loads(self.out().read_text())
        self.assertNotIn("user.delete", doc["methods"])
        self.assertIn("user.delete", r.stderr)
        calls = self.calls.read_text().splitlines()
        self.assertEqual(sum('"auth.login_with_api_key"' in l for l in calls), 1)


if __name__ == "__main__":
    unittest.main()
