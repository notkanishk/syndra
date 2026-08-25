# Design — add-on transport from one derived secret

## 1. What the certificates were actually for

The transport conflates two needs, and naming them apart is the whole change.

| Need | Currently answered by | Actually needs |
|---|---|---|
| The add-on knows the caller is the backend | client certificate, or HMAC | a shared secret, signed over the request |
| The backend knows the peer is the real add-on | server certificate + private CA | proof the peer holds that secret |
| The body cannot be read on the wire | TLS | TLS |

Only the third genuinely requires transport encryption, and the evidence is in
the contract rather than in an argument: `capabilities.go` declares
`secret_params: ["password"]` and `secret_params: ["elevated_key"]`. A member's
plaintext TrueNAS password and the elevated purge credential travel in that
body. Every redaction rule on this branch exists because of it.

The first two needs are satisfiable by a shared secret. `transport.go`'s
objection —

> A bare shared secret is deliberately not one of them: it identifies the caller
> and binds nothing, so an intercepted call replays verbatim, forever.

— is an argument against a **presented** secret, not a **signing** secret, and
signed mode already answers it: the MAC covers timestamp, method, path and body,
inside a two-minute window, with operation-id dedup underneath. That reasoning
survives this change untouched. What does not survive is the inference that the
*other* direction therefore needs a PKI.

## 2. Derive rather than distribute

One value per target in `.env`. Both keys come out of it and neither is
transmitted.

```
ADDON_<TARGET>_SECRET
  ├─ HKDF(SHA-256, salt=<target>, info="syndra/addon-tls/v1",  32) → Ed25519 seed → keypair
  └─ HKDF(SHA-256, salt=<target>, info="syndra/addon-hmac/v1", 32) → HMAC key
```

The add-on derives the keypair, self-signs an in-memory certificate around it,
and serves it. The backend derives the same **public** key and pins it. Neither
end reads a certificate file; neither end has an expiry to renew.

`crypto/hkdf` is standard library from Go 1.24 and the modules are on 1.25, so
this adds **no dependency at either end** — which matters, because the add-on's
entire dependency set today is the TrueNAS client, bbolt, and gorilla/websocket.

### Why Ed25519 and not ECDSA

`ed25519.NewKeyFromSeed` is RFC 8032: a 32-byte seed defines the keypair, by
specification. Deriving an ECDSA key deterministically means handing
`ecdsa.GenerateKey` a seeded reader and depending on how the standard library
happens to consume randomness — reproducible today, and not a property Go
promises. A silent change there would produce a public key mismatch on a routine
toolchain bump, on both ends, with no code change to point at. Ed25519 has no
such exposure, and Go's TLS 1.3 stack has supported Ed25519 certificates since
1.13.

### The certificate needs no determinism

Only the key is derived. Serial number and validity dates can be random and
wall-clock, because the backend pins the public key and nothing else — under
`InsecureSkipVerify` the chain, the name and the expiry are all unchecked.
Attempting to make the DER reproducible would be solving a problem this design
does not have.

## 3. The encoding trap, pinned first

**HKDF is computed over the configured value's UTF-8 bytes, after trimming
surrounding whitespace. The value is NOT hex-decoded, even though
`openssl rand -hex 32` makes it look like hex.**

This is the highest-risk ambiguity in the change and it is exactly the shape of
the defect the branch keeps hitting. The precedent is already here: the signing
key was once HMAC'd as the file's contents by one end and as the literal path
string by the other, and the only symptom was `no matching signature`. A
disagreement about hex-decoding is the same failure with the same symptom.

Trimming is not optional and is not tidiness — a mounted secret almost always
carries a trailing newline, and `secretValue` already trims for this reason.

## 4. Domain separation, and what the salt does not buy

Two derivations from one value, separated by `info`, so the key that
authenticates the backend and the key that identifies the add-on are unrelated
bytes. The raw secret is never used directly for either.

The salt is the target name, so a configuration mistake that reuses one secret
across two add-ons still yields different keys. **State the limit plainly: this
prevents accidental key collision and is not a security control.** Anything
holding the secret knows the algorithm and the target names and derives both
sets. The control is a **distinct secret per add-on**, and the spec requires it.

`/v1` in each `info` string is the migration path: bump it to rotate every
derived value without changing the configured secret.

## 5. Pinning replaces name verification, and is stronger

```go
InsecureSkipVerify: true,
VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
	c, err := x509.ParseCertificate(raw[0])
	if err != nil { return err }
	got, ok := c.PublicKey.(ed25519.PublicKey)
	if !ok || !got.Equal(want) {
		return errors.New("add-on's key is not the one derived from the deployment secret")
	}
	return nil
},
```

`InsecureSkipVerify` reads like a downgrade and is not one here. Go still calls
`VerifyPeerCertificate` when it is set, and what replaces the CA and hostname
checks is an exact public-key match. A name check asks whether some authority
vouched for a string; this asks whether the peer holds the deployment secret.
The second question is the one worth asking between two components of one
deployment.

**The comment on this code must say that.** A future reader meeting
`InsecureSkipVerify: true` with no explanation will either "fix" it or lose
confidence in the whole path — and §32 of `addon-platform` is a record of what
wrong explanations cost.

## 6. Why not the cheaper option

Skip-verify with HMAC and no pinning is ~35 fewer lines and was considered.

An active on-path attacker terminates the TLS session, reads the body, and
relays the still-valid signed request onward. The HMAC travels with it and
verifies. **What leaks is a member's plaintext password and the elevated purge
key** — the two values the entire `secret_params` machinery exists to protect.

With pinning, the handshake fails before any body is written. That is the
argument for the extra lines, and it is specific to what this contract carries.

## 7. Alternatives rejected

- **Unix domain socket.** Simpler still, and deletes more: filesystem
  permissions authenticate, and the backend could drop the `addons` network
  entirely. Rejected because it makes co-location with the backend permanent,
  and add-ons exist to sit near targets that may not be on this host.
- **Bearer shared secret, no signature.** The original objection stands: nothing
  binds it to the request, so an intercepted call replays verbatim.
- **SPIFFE/SPIRE, Istio, Linkerd.** Automatic certificate rotation with a
  control plane an order of magnitude larger than the two services it secures.
- **step-ca in the Compose stack.** The right answer at twenty services. At two,
  a new container and a new bootstrap to replace one `openssl` invocation.
- **Tailscale/Nebula sidecars.** An external control plane for a hop that never
  leaves the host.
- **TLS-PSK.** Would remove certificates outright, and Go's `crypto/tls` does
  not expose PSK outside session resumption.

## 8. The contract vector is the load-bearing artifact

The derivation creates a new place where two separately deployed binaries must
agree byte for byte and nothing makes them — the pattern named in the header of
`addon-platform/tasks.md`, found there as two fakes agreeing with each other, as
`btrim` against `strings.TrimSpace`, and as the proxy allowlist against the
router. A salt, an `info` string, a hash choice, or an encoding decision
disagreeing by one byte produces "the key is not the one derived", with each
side internally consistent and certain.

`addons/contract/transport_derivation.json` is therefore not documentation. It
carries a fixed secret and, for two targets, the derived seed, public key and
HMAC key; plus one full signature over a known request. Both suites assert
against it, exactly as `apply_request.json` is asserted from both ends.

Two properties of the vector are deliberate:

1. **The secret appears twice** — as UTF-8 and as the hex of those bytes — so
   the encoding decision in §3 cannot be misread from the artifact.
2. **The signature entry pins the existing MAC construction** against a derived
   key, so the vector covers the whole authentication path rather than the new
   half. Its `mac_hex` has been verified against the shipped `computeMAC`, not
   against a reimplementation of it.

## 9. `requireHTTPS` survives, with a different reason

The check must stay: a plaintext base URL would put the declared secret
parameters on the wire. Its current justification will become false —

> because a client certificate is never presented and a private CA is never
> consulted on a connection that performs no handshake

— since under this design there is no client certificate and no private CA. Left
standing, it reads as vestigial to the next person, who removes the check. It
must be rewritten to the confidentiality reason.

**It must not be anchored the way an earlier draft of this document proposed.**
That draft called for a single test — *a target declaring `secret_params` cannot
register over `http://`* — and the implementation cannot perform that check.
`requireHTTPS` is called inside `Init`'s loop over `ADDON_TARGETS`, from
environment configuration, before the `Registration` struct is built and before
any add-on has been contacted; the manifest that declares `secret_params` is read
later, over the transport this check is guarding. A test with that name would
pass while asserting a causal path that does not exist, and its name would stop
the next reader looking — which is precisely the §32 defect, committed in a
specification rather than in a comment.

The anchoring is **two assertions, because there are two facts**:

1. `http://` is refused unconditionally, decided from configuration alone.
2. Separately, the shipped contracts do declare `secret_params` — which is what
   makes rule 1 worth having, and what would need revisiting if it ever stopped
   being true.

The "no development escape hatch" property is unchanged and non-negotiable.

## 10. Delivering the secret, and rotating it

**A mount survives the certificates.** Retiring the transport material is not a
reason to retire file-based secret delivery: an environment value is readable
from `docker inspect` and from `/proc/1/environ`, and the add-on is the least
trusted container in the deployment. Removing the mounts while documenting
`_FILE` as preferred would leave a promise with no delivery path behind it.

**One file per target, mounted into both — not a copy per side.** An earlier
draft called for the backend's copy and the add-on's copy, written "as one
operation" and kept identical. That specified an atomicity no filesystem
provides: an interrupt, a failed `chown`, or a host crash between the two writes
leaves one copy, and the accompanying refusal-if-either-exists rule then stranded
the deployment in exactly the split-secret state the generator existed to
prevent. The rule and the failure mode were each reasonable and jointly a
deadlock.

The two copies were never justified anyway, and noticing why is the useful part.
The two-directory rule in this deployment protects the backend's **client key** —
material the add-on must not be able to read. That reasoning does not survive
into a symmetric scheme: both ends hold the same bytes by definition, and there
is no half the add-on should not see. A second copy carries no confidentiality
and buys only a state in which the two can disagree.

So: one file, `secrets/addon-<target>.key`, mounted read-only into the backend
and into that add-on. Compose top-level `secrets:` is the native mechanism.
`0640` owned `root:65532` — the backend reads it as owner, the add-on by group,
since it runs as uid 65532 against a read-only mount and cannot open a root-owned
`0600` file, while `0644` would leave the deployment's most sensitive value
world-readable on the host.

With one file the atomicity question becomes ordinary rather than impossible:
write to a temporary path on the same filesystem, set mode and ownership there,
publish, and clean up the temporary on every exit the process reaches.

**Publish with `link`, not `rename`.** An earlier draft said to check the file is
absent and then `rename` into place. `rename` is atomic but it is not
*exclusive*: it replaces the destination silently. Two runs that both observe no
file both publish, and the later one destroys a secret the earlier may already
have put into service — which is the split state between the two ends that this
entire design exists to make impossible, produced by the tool meant to prevent
it. The check does not help; it is separated from the publication by a window,
which is what a TOCTOU is.

`link(2)` is the primitive that carries the guarantee: `ln "$tmp" "$dest"` fails
`EEXIST` and never clobbers, and the temporary is already on the destination
filesystem for the ownership step, so the hard link is valid. Unlink the
temporary afterwards. The guarantee must live in the publication step, not in a
check before it — a lock may serialise runs as a convenience, but a lock not
taken, or taken on a different path, leaves the clobber available.

The preflight — destination writable, ownership settable, **fail before creating
anything** — stays, but as ergonomics rather than safety. It stops a run that
cannot succeed from leaving temporaries behind and reports the missing privilege
instead of a failed syscall. It is explicitly not what makes the operation safe.

**And because publication is indivisible, "recovery" has only two cases.** An
interruption lands either before the `link` — no secret exists, so the next run
publishes one — or after it, leaving a complete, correctly owned secret. In the
second case the run produced exactly what it existed to produce. The target is
provisioned, and the next run's refusal is the **successful** end of that story
rather than an obstacle.

An earlier draft's two scenarios contradicted each other on precisely this: one
required a subsequent run to "proceed without manual repair", the other required
it to refuse, and after a post-publication interrupt both applied. "Proceed" was
written meaning *the operator is never stranded* and reads as *succeed*. There is
no third state to repair, so the fix is to say which outcome is correct rather
than to make the generator idempotent — a generator that silently succeeded on
an existing secret would turn a mistaken rotation into a no-op that reports
success.

That puts weight on the refusal message. It must distinguish *this target already
has a secret and nothing needs doing* from *insufficient privilege* and
*destination unwritable*, because an operator meeting a bare "refused" straight
after an interruption cannot tell completion from breakage, and the tempting next
action is deleting a live secret to unblock themselves.

Two consequences for the temporary. It must be created with its restrictive mode
from the outset, not widened or narrowed afterwards — a run killed between
creation and a later `chmod` would leave key material readable by whatever the
interim mode allowed, and a killed run is exactly the case this section is
about. And its path must be unique per run.

**The stale temporary is not an input to the decision.** Publication and the
removal of the temporary are separate steps, so a kill between them leaves a
complete secret *and* a stale temporary at once. An earlier draft said such a
temporary "must not prevent a subsequent run from succeeding" — which
contradicts the refusal that finished secret requires, and was the third
appearance of one mistake: asserting an **outcome** where the invariant is that
the temporary **does not affect** the outcome.

The destination decides. Where no secret exists the next run publishes one;
where one exists it refuses; the temporary changes neither which of those
happens nor fails the run on its own account. Unique temporary paths are what
make that true mechanically — a deterministic path is precisely how an abandoned
temporary would become an input, by colliding with the next run's own exclusive
creation and failing it for a reason unrelated to the state of the secret.

Relatedly, "remove temporaries on any failure path" cannot be required of a
process that has been `SIGKILL`ed, and this design models exactly that case.
Cleanup is required on every exit the process still reaches; the killed case is
covered by the restrictive mode and the irrelevance rule, not by a cleanup that
never runs.

**Both ends must read it the same way**, and today they do not. The add-on's
`secretValue` accepts an inline value or a `_FILE` path; the backend accepts only
a path, read by `readSigningKey`. That asymmetry is why `.env.example` currently
has to warn that the signing key is "A PATH, matching the backend's … which is
also a path" — a warning is what a design owes an operator when two things that
should agree do not. The backend takes the add-on's semantics, so
`ADDON_<TARGET>_SECRET` and `ADDON_<TARGET>_SECRET_FILE` mean one thing
everywhere.

**Distinctness is enforced, not assumed.** The requirement that each target hold
its own secret has no check behind it. Generating fresh values covers first
configuration and nothing after: a copied `.env` block, a hand-edit, or a
rotation that reuses a value reintroduces the duplicate silently, and the
deployment continues to look isolated. `Init` compares the **resolved bytes** —
not the configured strings, since one target may be inline and another a file
path — and refuses the affected targets without preferring either. Registering
one of a colliding pair would leave an operator believing two add-ons are
isolated when one credential opens both.

**Generation is its own privileged step, and the bootstrap must not invoke it.**
An earlier draft had `gen-prod-env.sh` call the generator once per name in
`ADDON_TARGETS` at first setup. That cannot work, for two independent reasons,
and both are visible in the runbook rather than needing to be reasoned about:

1. **Privilege.** `gen-prod-env.sh` runs as `su - runner` (`DEPLOY.md` step 3),
   and `DEPLOY.md:92` states plainly that the deploy user "must not run as
   root" — it drives automated deploys. Setting `root:65532` ownership requires
   exactly the privilege that user is forbidden to hold. An unprivileged parent
   cannot invoke a child that needs privilege by design.
2. **Input.** `ADDON_TARGETS` is not an input to that script. It is a line in the
   `.env` the script is generating, filled in by the operator afterwards — so a
   loop over it would be reading something it had just written empty.

So `scripts/gen-addon-secret.sh <target>` is standalone, takes the target as an
argument, and is run under `sudo` per target — exactly as `gen-addon-certs.sh`
is today. The bootstrap keeps its unprivileged property and gains nothing by
knowing about add-ons. Adding UniFi later is the same command as the first time.

Granting the deploy user membership of gid 65532 would avoid the `sudo` and is
the wrong trade: it would make every add-on secret permanently readable by the
account that runs automated deploys, to save a one-time action.

Refusal on an existing file is safe here only because there is one file. With
two, refuse-if-present was the rule that converted a partial failure into a
deadlock — the script that existed to prevent a split secret was the thing that
could create one and then decline to repair it.

**The contract vector does not cover any of this.** It answers "given these
bytes, what comes out" and is blind to how the bytes were obtained — `_FILE`
precedence, a missing mount, an empty file, a trailing newline, or two targets
resolving to one value. An earlier draft of this design claimed the vector
asserted the configuration semantics, which was a false-coverage claim of exactly
the kind §32.4 records. Those cases are tested separately, in both modules,
because the property under test is that the two ends resolve configuration
identically.

**Rotation recreates.** `docker compose restart` restarts a container with the
environment it was created with, so a procedure written as "replace the value and
restart both ends" leaves both running the previous secret while reporting
success — a silent no-op on a security control, which is the worst shape a
runbook line can take.

**And the drain is a runtime call, not an environment edit.** An earlier draft of
this section said to set `TRUENAS_LIFECYCLE_STATE=draining` first. That variable
is read once, by `envOr("LIFECYCLE_STATE", …)` at start-up; editing `.env` does
nothing to a running container. The instruction was also **circular** — applying
it would require the recreation the drain exists to precede. The real control is
`POST /lifecycle` (`server.go:65`), with `GET /health`'s `drained` field as the
completion signal, which is what `lifecycle.go` says it is for: "`Drained` is what
an operator waits [on]".

That handler is authenticated over the transport being rotated, which fixes the
ordering: **drain before the secret changes.** Afterwards the old container's
lifecycle surface is unreachable and the only remaining lever is SIGTERM.

So the procedure is: `POST /lifecycle` → poll `/health` until `drained` →
replace the value → `docker compose up -d --force-recreate backend truenas-addon`
→ confirm registration and the first manifest read.

**A defect found here lives in its own change.** The add-on's SIGTERM drain is
truncated by Docker's default stop timeout on every stop the deployment has ever
performed. That is real, shipped, and unrelated to this proposal — so it is
specified by [`addon-shutdown-grace-period`](../addon-shutdown-grace-period/)
rather than carried here. Nothing in this change depends on it: the rotation
above quiesces through `POST /lifecycle` and waits for `drained` **before**
anything is stopped, so the signal path is never the mechanism.

An earlier draft described it as separable in prose while carrying it as a task
and a requirement. Scope stated one way and encoded another is not separable —
it is scope, plus a sentence that stops anyone checking.

The two ends cannot move atomically. There is a bounded window in which one has
rotated and the other has not, and the honest description of it is that calls
fail to authenticate during it — they do not proceed unauthenticated, which is
the property that makes the window tolerable rather than dangerous. Rollback is
restoring the previous value and recreating again; nothing persistent is derived
from the secret, so it leaves nothing behind.

## 11. Per-add-on networks

Docker bridge networks let co-resident containers observe each other's traffic
given `CAP_NET_RAW`, which containers hold by default. With UniFi and the LLDAP
replacement joining `addons`, a compromised add-on would sit on the path of the
backend's calls to its peers.

One network per add-on makes each backend↔add-on link a two-member segment. This
is the structural half of the isolation and it is free; the cryptographic half
above is what holds if it is ever misconfigured.

## 12. Residual risks, stated

1. **The scheme is symmetric.** Either end can impersonate the other, unlike
   private keys. Within a two-party system built and deployed together this
   bought nothing before — the add-on's only peers are the backend and its
   target. It would matter if a third party ever had to verify an add-on
   independently, and nothing plans that. It is the reason each add-on gets its
   own secret rather than a deployment-wide one.
2. **The scheme is non-standard.** mTLS is recognisable; this must be read to be
   trusted. The cost is paid in review attention, and §8 and §5's comment
   requirement are the mitigations.
3. **A weak configured secret weakens everything derived from it.** HKDF
   strengthens nothing. The generator must mint it; an operator must not choose
   it. This is also why §10's duplicate check is a start-up refusal rather than
   a warning — the two ways a secret goes wrong, weak and reused, are both
   invisible from anywhere else in the running system.
