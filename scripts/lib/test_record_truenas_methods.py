"""Tests for the recorder's --methods mode (slice 9a). No NAS, no Docker.

Run: python3 -m unittest discover -s scripts/lib -p 'test_*.py'

The end-to-end tests run the real scripts/record-truenas-fixtures.sh in a copy
of the files the runbook copies, with a fake `docker` on PATH that runs the
mounted helper directly against a fake `websockets` serving a core.get_methods
reply built from the shipped capture, enriched with what the NAS adds.
"""
import asyncio, copy, hashlib, importlib.util, json, os, pathlib, pty, select, shutil, subprocess, sys, tempfile, termios, textwrap, time, unittest

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
            s["$comment"] = "a note naming Stone Cold"
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
        for word in ("description", "examples", "$comment", "Stone Cold", "Undertaker", "Hogan", "example.invalid", "READONLY_ADMIN"):
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

    def test_a_discriminator_is_kept_whole(self):
        disc = {"propertyName": "kind", "mapping": {"description": "#/$defs/a"}}
        reply = {"user.query": {"accepts": [{"discriminator": disc, "description": "x"}], "returns": [{}], "job": False}}
        self.assertEqual(methods.project(reply, ["user.query"])["user.query"]["accepts"], [{"discriminator": disc}])

    def test_a_reply_that_never_comes_times_out(self):
        class Hang:
            async def __aenter__(self): return self
            async def __aexit__(self, *a): return False
            async def send(self, raw): pass
            async def recv(self): await asyncio.sleep(60)
        old, methods.TIMEOUT = methods.TIMEOUT, 0.05
        self.addCleanup(setattr, methods, "TIMEOUT", old)
        with self.assertRaises(asyncio.TimeoutError):
            asyncio.run(methods.session(Hang, KEY, []))

    def test_frames_for_other_calls_do_not_extend_the_wait(self):
        """One deadline per call: a stream of frames with another id never
        resets it."""
        class Chatter:
            async def __aenter__(self): return self
            async def __aexit__(self, *a): return False
            async def send(self, raw): pass
            async def recv(self):
                await asyncio.sleep(0.01)
                return json.dumps({"jsonrpc": "2.0", "id": 999, "result": None})
        old, methods.TIMEOUT = methods.TIMEOUT, 0.1
        self.addCleanup(setattr, methods, "TIMEOUT", old)
        start = time.monotonic()
        with self.assertRaises(asyncio.TimeoutError):
            asyncio.run(asyncio.wait_for(methods.session(Chatter, KEY, []), 3))
        self.assertLess(time.monotonic() - start, 1, "the wait was reset by frames for other calls")

    def test_services(self):
        self.assertEqual(methods.services(["pool.dataset.query", "pool.query", "user.create", "user.query"]),
                         ["pool", "pool.dataset", "user"])


FAKE_WEBSOCKETS = textwrap.dedent('''
    """A fake TrueNAS: answers only what the test lets it, and logs each method."""
    import json, os

    REPLY = json.load(open(os.environ["FAKE_REPLY"]))
    KEY, ELEVATED = json.load(open(os.environ["FAKE_KEYS"]))  # a file, so no key is in any environment
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
                r = self.key in (KEY, ELEVATED)
            elif m["method"] == "system.version":
                r = "TrueNAS-25.10.7"
            elif m["method"] == "core.get_methods":
                svc = m["params"][0] if m["params"] else None
                r = {n: e for n, e in REPLY.items()
                     if (svc is None or n.rsplit(".", 1)[0] == svc)
                     and (n != "user.delete" or self.key == ELEVATED)}
            self.out.append(json.dumps({"jsonrpc": "2.0", "id": m["id"], "result": r}))
        async def recv(self):
            return self.out.pop(0)

    def connect(url, **kw):
        if os.environ.get("FAKE_FAIL"):
            raise OSError(f"[Errno 61] Connect call failed {url}")
        return _WS()
''')

# The fake docker runs the helper the real command line mounts, with the mounts
# resolved and stdin passed through, in an environment built as docker builds a
# container's: only the `-e` names, plus what the fake itself needs. It logs its
# own argv and environment, which is what `ps` and `docker inspect` would show.
FAKE_DOCKER = textwrap.dedent('''
    #!/usr/bin/env python3
    import json, os, sys
    args, mounts, env, i = sys.argv[1:], {}, {}, 0
    with open(os.environ["FAKE_DOCKER_LOG"], "w") as f:
        json.dump({"argv": sys.argv, "env": dict(os.environ)}, f)
    while i < len(args):
        if args[i] == "-v":
            src, dst = args[i + 1].split(":")[:2]
            mounts[dst] = src
            i += 1
        elif args[i] == "-e":
            name, eq, value = args[i + 1].partition("=")
            if eq or name in os.environ:
                env[name] = value if eq else os.environ[name]
            i += 1
        i += 1
    env.update({k: v for k, v in os.environ.items() if k in ("PATH", "PYTHONPATH") or k.startswith("FAKE_")})
    cmd = args[-1].split("&&")[-1].split()
    os.execvpe(sys.executable, [sys.executable] + [mounts.get(a, a) for a in cmd[1:]], env)
''').lstrip()

# The staging `.env`: the three names the recorder reads, among others it must
# not read, and a line that would run if the file were sourced.
ENV_FILE = textwrap.dedent(f'''
    SESSION_SECRET=undertaker-session-secret
    TRUENAS_URL="{URL}"
    TRUENAS_API_KEY={KEY}
    touch sourced
    PWNED=$(touch sourced)
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
        (self.tmp / "keys.json").write_text(json.dumps([KEY, ELEVATED]))
        self.docker_log = self.tmp / "docker.json"
        self.env_file = self.tmp / "staging.env"
        self.env_file.write_text(ENV_FILE)
        env = {k: v for k, v in os.environ.items() if not k.startswith("TRUENAS_")}
        self.env = dict(env, PATH=f"{fakes}:{os.environ['PATH']}", PYTHONPATH=str(fakes),
                        RECORD_ENV_FILE=str(self.env_file), FAKE_KEYS=str(self.tmp / "keys.json"),
                        FAKE_DOCKER_LOG=str(self.docker_log),
                        FAKE_REPLY=str(self.tmp / "reply.json"), FAKE_CALLS=str(self.calls))

    def run_recorder(self, *args, typed=None, stdin=""):
        """Runs the recorder; `typed` is what the owner types at a terminal,
        otherwise stdin is a pipe holding `stdin`."""
        cmd = ["bash", "scripts/record-truenas-fixtures.sh", *args]
        if typed is None:
            return subprocess.run(cmd, cwd=self.work, env=self.env, input=stdin,
                                  capture_output=True, text=True, timeout=60)
        master, slave = pty.openpty()
        p = subprocess.Popen(cmd, cwd=self.work, env=self.env, stdin=slave, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE, text=True)
        # Typed once the prompt has turned echo off: bash's `read -s` flushes
        # anything typed before it.
        deadline = time.monotonic() + 10
        while termios.tcgetattr(slave)[3] & termios.ECHO and p.poll() is None:
            if time.monotonic() > deadline:
                p.kill()
                p.communicate()
                os.close(slave), os.close(master)
                self.fail("the prompt never turned echo off")
            time.sleep(0.01)
        os.close(slave)
        try:
            os.write(master, (typed + "\n").encode())
        except OSError:  # it refused before it asked, and the terminal is gone
            pass
        try:
            out, err = p.communicate(timeout=60)
        except subprocess.TimeoutExpired:
            p.kill()
            p.communicate()
            os.close(master)
            self.fail("the recorder hung")
        # What the terminal showed: the paste is never echoed to it.
        shown = b""
        while select.select([master], [], [], 0)[0]:
            try:
                chunk = os.read(master, 4096)
            except OSError:  # the terminal is gone
                break
            if not chunk:
                break
            shown += chunk
        os.close(master)
        if typed:
            self.assertNotIn(typed.encode(), shown, "the pasted key was echoed to the terminal")
        return subprocess.CompletedProcess(cmd, p.returncode, out, err)

    def assert_nothing_happened(self, r, before):
        self.assertNotEqual(r.returncode, 0)
        self.assertFalse(self.calls.exists(), "it connected")
        self.assertFalse(self.docker_log.exists(), "it ran docker")
        self.assertEqual(tree(self.work), before)

    def out(self):
        return self.work / "addons/truenas/compat/baseline/TrueNAS-25.10.7.methods.json"

    def test_write_is_refused_before_connecting(self):
        for args in (["--methods", "--write"], ["--write", "--methods"]):
            before = tree(self.work)
            r = self.run_recorder(*args, typed=ELEVATED)
            self.assertNotEqual(r.returncode, 0)
            self.assertIn("--write", r.stderr)
            self.assertFalse(self.calls.exists(), "it connected")
            self.assertEqual(tree(self.work), before)

    def test_capture_with_the_elevated_key(self):
        before = tree(self.work)
        r = self.run_recorder("--methods", typed=ELEVATED)
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
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("OSError", r.stderr)
        for secret in (KEY, ELEVATED, URL, "nas.example.invalid"):
            self.assertNotIn(secret, r.stdout + r.stderr)
        self.assertEqual(tree(self.work), before)

    def test_neither_key_reaches_argv_or_an_environment(self):
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assertEqual(r.returncode, 0, r.stderr)
        seen = json.loads(self.docker_log.read_text())
        text = json.dumps(seen)
        for secret in (KEY, ELEVATED, "undertaker-session-secret"):
            self.assertFalse(secret in text, f"{secret} reached docker's argv or environment")
        argv = seen["argv"]
        names = {argv[i + 1].split("=")[0] for i, a in enumerate(argv) if a == "-e"}
        self.assertEqual(names, {"TRUENAS_URL", "TRUENAS_VERIFY_TLS"})
        self.assertFalse((self.work / "sourced").exists(), "the .env file was run as shell")

    def test_an_empty_paste_writes_nothing(self):
        before = tree(self.work)
        r = self.run_recorder("--methods", typed="")
        self.assert_nothing_happened(r, before)
        self.assertIn("elevated key is required", r.stderr)
        self.assertTrue(CAPTURE.exists())

    def test_the_helper_refuses_without_both_keys(self):
        for stdin in ("", KEY + "\n", KEY + "\n\n", "\n" + ELEVATED + "\n"):
            r = subprocess.run([sys.executable, str(ROOT / "scripts/lib/record-truenas-methods.py"), str(DEPS)],
                               env=dict(self.env, TRUENAS_URL=URL), input=stdin,
                               capture_output=True, text=True, timeout=60)
            self.assertNotEqual(r.returncode, 0, repr(stdin))
            self.assertIn("both keys", r.stderr)
            self.assertFalse(self.calls.exists(), "it connected")

    def test_without_a_terminal_no_key_is_read(self):
        before = tree(self.work)
        r = self.run_recorder("--methods", stdin=ELEVATED + "\n")
        self.assert_nothing_happened(r, before)
        self.assertIn("terminal", r.stderr)

    def test_a_key_given_as_a_file_stops_without_reading_it(self):
        secret = self.tmp / "api.key"
        secret.write_text(KEY)
        secret.chmod(0)
        self.env_file.write_text(f"TRUENAS_URL={URL}\nTRUENAS_API_KEY=\nADDON_API_KEY_FILE={secret}\n")
        before = tree(self.work)
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assert_nothing_happened(r, before)
        self.assertIn("ADDON_API_KEY_FILE", r.stderr)
        self.assertNotIn(str(secret), r.stderr)

    def test_a_key_file_stops_even_beside_an_inline_key(self):
        # The add-on reads ADDON_API_KEY_FILE first (main.go secretValue), so
        # the inline key is not the key it uses.
        secret = self.tmp / "api.key"
        secret.write_text(KEY)
        secret.chmod(0)
        self.env_file.write_text(f"TRUENAS_URL={URL}\nTRUENAS_API_KEY={KEY}\nADDON_API_KEY_FILE={secret}\n")
        before = tree(self.work)
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assert_nothing_happened(r, before)
        self.assertIn("ADDON_API_KEY_FILE", r.stderr)

    def test_a_key_in_the_callers_environment_stays_out_of_docker(self):
        self.env["TRUENAS_API_KEY"] = KEY
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertNotIn(KEY, self.docker_log.read_text(), "the key reached docker's argv or environment")

    def parsed(self, text):
        """Runs a capture with `text` as the env file; returns the URL the
        container saw. It succeeds only if the key was read exactly."""
        self.env_file.write_bytes(text.encode())
        r = self.run_recorder("--methods", typed=ELEVATED)
        self.assertEqual(r.returncode, 0, r.stderr)
        return json.loads(self.docker_log.read_text())["env"]["TRUENAS_URL"]

    def test_env_file_crlf(self):
        self.assertEqual(self.parsed(f"TRUENAS_URL={URL}\r\nTRUENAS_API_KEY={KEY}\r\n"), URL)

    def test_env_file_export(self):
        self.assertEqual(self.parsed(f"export TRUENAS_URL={URL}\nexport TRUENAS_API_KEY={KEY}\n"), URL)

    def test_env_file_unquoted_comment_is_dropped(self):
        self.assertEqual(self.parsed(f"TRUENAS_URL={URL} # the NAS\nTRUENAS_API_KEY={KEY} # the add-on's\n"), URL)

    def test_env_file_hash_inside_quotes_is_kept(self):
        self.assertEqual(self.parsed(f"TRUENAS_URL='{URL}#a # b' # note\nTRUENAS_API_KEY=\"{KEY}\"\n"),
                         URL + "#a # b")

    def test_env_file_value_containing_equals(self):
        self.assertEqual(self.parsed(f"TRUENAS_URL={URL}?a=b\nTRUENAS_API_KEY={KEY}\n"), URL + "?a=b")

    def test_env_file_last_duplicate_wins(self):
        self.assertEqual(self.parsed(f"TRUENAS_URL=wss://x.invalid/\nTRUENAS_API_KEY=wrong\n"
                                     f"TRUENAS_URL={URL}\nTRUENAS_API_KEY={KEY}\n"), URL)


if __name__ == "__main__":
    unittest.main()
