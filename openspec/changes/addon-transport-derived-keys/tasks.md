# The derived-transport task ledger

> **Read the contract vector first.** `addons/contract/transport_derivation.json`
> is the only thing making two separately deployed binaries agree about the
> derivation, and every failure this change can produce looks identical from
> either side: `the key is not the one derived from the deployment secret`, with
> both ends internally consistent. Group 1 lands before any implementation, on
> purpose — writing the vector after the code would pin whatever the code
> happened to do.
>
> **The encoding decision is the one to get wrong.** HKDF runs over the
> configured value's trimmed UTF-8 bytes, never hex-decoded, even though the
> value looks like hex. The precedent is the signing key that one end HMAC'd as
> file contents and the other as a path string.

## 1. The contract vector, before anything else

- [x] 1.1 `addons/contract/transport_derivation.json` written with real computed values: HKDF parameters, the secret in both UTF-8 and hex-of-those-bytes, and for `truenas` and `unifi` the seed, Ed25519 public key and HMAC key
- [x] 1.2 The vector carries one full signature entry — `unix`, method, path, body, derived HMAC key, resulting `mac_hex` — so it covers the existing MAC construction and not only the new derivation. `mac_hex` verified against the shipped `computeMAC`, not a reimplementation
- [x] 1.3 `addons/contract/README.md` documents the new file beside the three envelope fixtures, naming what each field group pins and why the encoding is stated twice
- [x] 1.4 A second target (`unifi`) is in the vector deliberately: it is what fails if a future implementation drops the salt, which would otherwise be invisible while only one add-on exists

## 2. The add-on end

- [ ] 2.1 `deriveTLSKey(secret, target)` — `hkdf.Key(sha256.New, secret, []byte(target), "syndra/addon-tls/v1", ed25519.SeedSize)` into `ed25519.NewKeyFromSeed`
- [ ] 2.2 `deriveHMACKey(secret, target)` under `"syndra/addon-hmac/v1"`, replacing the signing key read from a file
- [ ] 2.3 Self-signed certificate minted in memory at boot around the derived key. Never written to disk, never mounted, no expiry management. Serial and validity deliberately non-deterministic — only the key is pinned
- [ ] 2.4 `ListenAndServeTLS(certFile, keyFile)` becomes `ServeTLS` over the in-memory certificate. `TLS_CERT_FILE`, `TLS_KEY_FILE`, `TLS_CLIENT_CA_FILE` and `SIGNING_KEY*` are removed from config, and their absence is not a startup warning — they are gone, not optional
- [ ] 2.5 The `authenticator`'s mTLS branch is deleted. Signed mode is the only mode, so the "configure exactly one of" refusal goes with it — the failure it prevented cannot occur when there is one mode
- [ ] 2.6 The add-on refuses to start with no secret configured, with the same fail-closed reasoning as the current transport: a component holding the target credential must not answer an unauthenticated caller
- [ ] 2.7 Test: derivation matches `transport_derivation.json` for both targets in the vector
- [ ] 2.8 Test: a request signed with a key derived from a different secret is refused, and the refusal does not disclose which half failed

## 3. The backend end

- [ ] 3.1 The same two derivations, from `ADDON_<TARGET>_SECRET`. Test-asserted against the same vector file, which is the point of the file
- [ ] 3.2 `VerifyPeerCertificate` pins the derived Ed25519 public key; `InsecureSkipVerify: true` with a comment stating why it is not a downgrade — an exact key match replaces a name check, and a bare `InsecureSkipVerify` with no explanation is either "fixed" by the next reader or destroys their confidence in the path
- [ ] 3.3 `Registration` loses `ClientCertPath`, `ClientKeyPath`, `CAPath`, `SigningKeyPath`; `AuthMode()` collapses. The incomplete-mTLS warning goes — the state it described cannot exist
- [ ] 3.4 A target configured with no secret does not register, preserving the current fail-closed rule exactly
- [ ] 3.5 **Duplicate secrets refused at start-up.** `Init` compares the resolved secret bytes across configured targets and refuses the affected ones. The spec requires distinctness because reuse lets one compromised add-on derive another's credentials, and until now nothing checked it — minting fresh values covers first configuration, while a copied `.env` block or a rotation that reuses a value reintroduces the duplicate with no symptom. Neither target is preferred: registering one silently leaves an operator believing two targets are isolated when one credential opens both
- [ ] 3.6 Test: two targets configured with identical secret bytes both fail to register, whatever produced the values. Compare resolved bytes, not the configured strings — inline at one target and a `_FILE` path at the other is the case that would otherwise slip through
- [ ] 3.7 Test: a peer presenting a certificate derived from a different secret fails the handshake, and **fails before any request body is written** — the property the whole design turns on
- [ ] 3.8 Test: the pin rejects a correctly-named, otherwise-valid certificate. Name-based verification passing where key pinning must fail is the regression that would silently restore the weaker check

## 4. `requireHTTPS`, kept and re-reasoned

- [ ] 4.1 The check stays. Its comment is rewritten: the reason is that the body carries declared `secret_params`, not that a client certificate is never presented — the latter becomes false in this change and a false reason invites removal of a live guard
- [ ] 4.2 **Two assertions, not one.** (a) `http://` is refused unconditionally, decided from configuration alone; (b) separately, the supported contracts do declare `secret_params`. An earlier draft of this row specified a single test — *"a target whose manifest declares `secret_params` cannot register over `http://`"* — which the implementation **cannot** perform: `requireHTTPS` is called inside `Init`'s target loop, from environment configuration, before the `Registration` is constructed and before any add-on is contacted. There is no manifest at that point. That row was the §32 defect written into a spec: a test name asserting a causal check that is not there, which passes for the wrong reason and stops the next reader looking
- [ ] 4.3 The "deliberately no development escape hatch" property is carried forward verbatim. An exemption for localhost is an exemption that ships

## 5. Deployment surface

- [ ] 5.1 One network per add-on (`addons_truenas`, …); the backend joins each. No add-on shares an L3 segment with another, which is the structural half of the isolation
- [ ] 5.2 **The backend gains the add-on's `secretValue` semantics**, so `ADDON_<TARGET>_SECRET` and `ADDON_<TARGET>_SECRET_FILE` mean the same thing at both ends. Today the asymmetry is real and has already cost a debugging session: the add-on accepts an inline value or a `_FILE` path, while the backend accepts only a path (`readSigningKey` over `SigningKeyPath`) — which is why `.env.example` has to warn that the signing key "is A PATH, matching the backend's … which is also a path". One helper, one naming convention. **The contract vector does NOT cover this** — an earlier draft of this row said it did, which was the same false-coverage claim §32.4 records: the vector pins derivation and MAC *bytes* given a secret, and is blind to how that secret was resolved. Configuration semantics need their own tests, in group 8
- [ ] 5.3 **The certificate mounts go; a secret mount stays — ONE file, mounted into both.** Removing file delivery outright would force the shared secret into the environment, where `docker inspect` and `/proc/1/environ` read it. But it is **one file per target**, not a copy per side: the scheme is symmetric, both ends hold the same bytes by definition, and neither holds a half the other must not see — so a second copy buys no confidentiality and creates a state where the two can disagree. The two-directory rule was written for the backend's **client key**, which this change deletes; it does not carry over to a shared secret. Compose top-level `secrets:` is the native mechanism — one host file, referenced by both services, mounted read-only. **5.2 and 5.3 are one decision: do not tick one without the other**
- [ ] 5.4 `.env.example`: one `ADDON_<TARGET>_SECRET` per target, with the `_FILE` form documented as preferred and an actual mount behind it
- [ ] 5.5 **Secret generation is its own privileged step, and `gen-prod-env.sh` must NOT invoke it.** An earlier draft said that script would "call it once per name in `ADDON_TARGETS` at first setup", which cannot work for two independent reasons. **Privilege:** `gen-prod-env.sh` runs as `su - runner` (`DEPLOY.md` step 3) and `DEPLOY.md:92` states the deploy user "must not run as root", while setting `root:65532` ownership requires exactly that — an unprivileged parent cannot invoke a child needing privilege it is forbidden to hold. **Input:** `ADDON_TARGETS` is not an input to that script at all; it is a line in the `.env` the script is in the middle of generating, so iterating it would read something just created empty. `scripts/gen-addon-secret.sh <target>` is a standalone step run per target under `sudo`, exactly as `gen-addon-certs.sh` is today, and `gen-prod-env.sh` gains nothing and loses its unprivileged property by knowing about it
- [ ] 5.6 That script's contract, written down rather than left to the implementation: **input** is the target name (validated against the same charset `ADDON_TARGETS` splits on); **output** is **one** file, `secrets/addon-<target>.key`, plus the configuration lines naming it; **mode** `0640` owned `root:65532`, so the backend reads it as owner and the add-on reads it by group — the add-on runs as uid 65532 against a read-only mount and cannot open a root-owned `0600` file, and `0644` would make the deployment's most sensitive value world-readable on the host; **value** is `openssl rand -hex 32`, since an operator-chosen value is the one input HKDF cannot strengthen
- [ ] 5.7 **Publication is exclusive-create, not check-then-rename.** An earlier draft specified "check it does not exist, then `rename` into place", which is a TOCTOU with a silent loser: POSIX `rename` replaces the destination, so two runs that both see nothing both publish and the later **destroys a secret the earlier may already have put into service** — manufacturing the split state between the two ends that this whole design exists to prevent. The guarantee must come from the publication step, not from a check preceding it. Use `link(2)` — `ln "$tmp" "$dest"` fails `EEXIST` and never clobbers, and the temporary is already on the destination filesystem for the ownership step, so the hard link is valid. Then unlink the temporary. A lock may serialise runs, but is not the guarantee: a lock not taken, or taken on a different path, leaves the clobber available
- [ ] 5.8 **Preflight and cleanup, around that primitive** — an earlier draft required two copies written "as one operation", which no filesystem provides: an interrupt or a failed `chown` between the writes left one copy, and refuse-if-either-exists then stranded the deployment. With one file that dissolves. What remains: preflight that the destination is writable and the ownership can be set, **failing before creating anything** and naming the missing privilege rather than the failed syscall; create the temporary with its restrictive mode from the outset and at a **unique** path (a run killed before a later `chmod` would leave key material readable; a deterministic path would let an abandoned temporary collide with the next run's own); apply ownership there; publish per 5.7; remove the temporary on every exit the process still reaches — not "on any failure path", which is impossible for `SIGKILL` and is the case this ledger explicitly models. The preflight is ergonomics — it stops a run that cannot succeed — and is explicitly *not* the safety property
- [ ] 5.9 **A stale temporary is not an input to the decision.** `link` and the removal of the temporary are separate steps, so a kill between them leaves a complete secret *and* a stale temporary — and an earlier draft's rule that a leftover temporary "must not prevent a subsequent run from succeeding" then contradicted the refusal that finished secret requires. Third time the same mistake: asserting an *outcome* where the invariant is that the temporary **does not affect** the outcome. The destination decides — publish where no secret exists, refuse where one does — and the temporary neither changes which happens nor fails the run on its own account
- [ ] 5.10 **Refusal must say which refusal it is.** Because publication is indivisible, an interruption leaves either no secret or a finished one — and in the second case the run did exactly what it existed to do, so the next run's refusal is the **successful** end of that story, not an error. An earlier draft's scenarios contradicted each other here: one required a subsequent run to "proceed without manual repair", the other required it to refuse, and after a post-publication interrupt both applied. "Proceed" meant "the operator is never stranded"; it read as "succeed". The generator distinguishes *this target already has a secret, nothing to do* from *insufficient privilege* and *destination unwritable*, because an operator who meets a bare "refused" after an interruption cannot tell completion from breakage, and the tempting next move is deleting a live secret
- [ ] 5.11 The file holds the secret with **no trailing newline ambiguity**: whatever is written, both ends trim, and 8.2 asserts the equivalence rather than the script guaranteeing a form
- [ ] 5.12 `docker-compose.yml` updated; `config_env_test.go` must still pass — every variable the add-on reads is passed by the service, and this change moves several
- [ ] 5.13 `scripts/gen-addon-certs.sh` deleted
- [ ] 5.14 Certificate-expiry surfacing removed from target health. There is no expiry; a health field reporting `unknown` forever is worse than its absence, because it reads as a probe that is failing

## 6. Documentation

- [ ] 6.1 `DEPLOY.md`: the certificate ceremony in "Bringing up the TrueNAS add-on" collapses to one generated value
- [ ] 6.2 **The privilege boundary is written into the step order.** Step 3 stays as it is — `su - runner … gen-prod-env.sh`, unprivileged, minting no add-on secret and not failing because a target has none. Minting is a later, per-target step run under `sudo`, in the add-on bring-up section where the target is actually being added. Stated as an ordering rather than left implicit, because the two scripts now have different privilege requirements and the wrong order fails after writing `.env` rather than before
- [ ] 6.3 **The rotation procedure recreates; it does not restart.** `docker compose restart` restarts a container with the environment it was created with, so "change the secret and restart both" would leave both ends on the previous value while reporting success. The command is `docker compose up -d --force-recreate backend truenas-addon`, followed by confirming registration and the first manifest read. State the window — the two ends cannot move atomically, so calls fail to authenticate for a bounded period rather than proceeding unauthenticated. Rollback is restoring the previous value and recreating again; nothing persistent depends on the secret, so it is clean
- [ ] 6.4 **The drain is a runtime call, not an environment edit.** An earlier draft of the rotation row said to set `TRUENAS_LIFECYCLE_STATE=draining` and wait. That variable is read once, by `envOr("LIFECYCLE_STATE", …)` at start-up; editing `.env` changes nothing in a running container. Worse, it was **circular** — applying the edit requires the recreation the drain was supposed to precede. The real control is `POST /lifecycle`, the authenticated runtime handler at `server.go:65`, with `GET /health`'s `drained` field as the completion signal — `lifecycle.go` says outright that `Drained` "is what an operator waits [on]"
- [ ] 6.5 **Ordering constraint, from the same fact:** `POST /lifecycle` is authenticated over the transport being rotated, so the drain MUST complete *before* the secret is replaced. Afterwards the operator can no longer reach the old container's lifecycle handler, and the only remaining lever is SIGTERM
- [ ] 6.6 The mTLS-versus-signed explanation goes; there is one mode
- [ ] 6.7 `addon-platform/design.md` §"The add-on's privilege" neighbours the transport section — check nothing there still claims a private CA
- [ ] 6.8 `NEXT.md` reflects the transport change and its sequencing behind the live TrueNAS bring-up

## 7. Verification

- [ ] 7.1 Both suites assert the same vector file. A change to it fails both, in two modules, which is what makes it a contract rather than a fixture
- [ ] 7.2 An end-to-end test: derived server, pinned client, signed request, real handshake — plus the negative cases from 3.7 and 3.8. A proof of the scheme exists at `hkdfdemo/main.go` from the design session and should become this test rather than being rewritten from memory
- [ ] 7.3 `go test ./... && go vet ./...` in `backend/` and `addons/truenas`
- [ ] 7.4 A live re-run of the bring-up in `DEPLOY.md`, since this change alters the leg that registration exercises first

## 8. Configuration semantics, which the vector cannot reach

The contract vector answers "given these bytes, what comes out". Every question
below is about **how the bytes were obtained**, and the vector is blind to all of
them — a deployment can satisfy it perfectly and still fail to start, or start
with the wrong secret. Each case is asserted in **both** modules, because the
whole point is that the two ends resolve configuration identically.

- [ ] 8.1 `_FILE` takes precedence over the inline value where both are set, and precedence is the same at both ends. Divergence here means the two ends read different secrets while each reports itself correctly configured
- [ ] 8.2 A file whose content ends in a newline and an inline value without one derive **identical** keys. This is the concrete form of the §3 trimming rule and the likeliest real-world mismatch, since almost every mounted secret carries a trailing newline
- [ ] 8.3 An empty or whitespace-only `_FILE` is an **error**, not a fallback to the inline value. `secretValue` already gets this right and the reason is recorded there: a mount that did not land would otherwise start the add-on under whichever value the other variable happened to hold
- [ ] 8.4 A `_FILE` naming a path that does not exist is an error naming the path. The failure to distinguish "no secret configured" from "the mount is missing" is what turns a five-minute fix into an afternoon
- [ ] 8.5 Duplicate detection operates on **resolved** bytes across mixed forms — target A inline, target B by file, same value — which is 3.6 stated as a configuration case rather than a crypto one
- [ ] 8.6 The variable names agree across the module boundary. The backend reads `ADDON_<TARGET>_SECRET[_FILE]` and the add-on reads `SECRET[_FILE]`; they are necessarily different strings, so what must be asserted is the **suffix convention and resolution order**, not the names. `config_env_test.go` covers the add-on's half — that every variable it reads is passed by Compose — and the backend has no equivalent guard today

## What this change deliberately does not do

- **The add-on → TrueNAS leg is untouched.** `TRUENAS_URL` and
  `TRUENAS_VERIFY_TLS` are a different connection with different reasoning, and
  changing both at once would make a bring-up failure ambiguous between them.
- **No mTLS fallback is retained.** A second mode is a second thing to
  misconfigure, and the reason the current code refuses two configured modes is
  that a caller would otherwise choose which one authenticates it.
- **No nonce store.** Replay protection continues to rest on the operation
  identifier and the plan fingerprints, exactly as specified today.
