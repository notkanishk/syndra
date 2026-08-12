## MODIFIED Requirements

### Requirement: Add-on transport MUST be mutually authenticated and bind the request

Calls between the backend and an add-on MUST be mutually authenticated, and the authentication MUST bind to the request rather than only to the caller. Both directions MUST derive from a single per-target secret: the backend MUST authenticate itself by signing each request over a timestamp, the method, the path and the body, and the add-on MUST prove possession of the same secret by presenting a TLS key derived from it, which the backend MUST pin. A bearer shared secret presented as a credential MUST NOT be sufficient, because it authenticates the caller without binding anything to the request, and an intercepted call replays verbatim.

The secret MUST be distinct per target. A deployment-wide value would let a compromised add-on derive the credentials of every other add-on and impersonate the backend to each, which is the failure the per-add-on isolation exists to prevent. Salting the derivation by target name MUST NOT be treated as satisfying this requirement: it prevents an accidental key collision, and anything holding the secret knows both the algorithm and the target names.

Distinctness MUST be enforced at start-up and MUST NOT be left to the generator that mints the values. Generation covers first configuration only; a hand-edited deployment, a copied block, or a rotation that reuses a value reintroduces the duplicate afterwards, and nothing about the running system would report it. A backend observing identical secret bytes configured for two targets MUST refuse to register the affected targets, and MUST NOT resolve the collision by preferring one of them — a silent preference leaves an operator believing two targets are isolated when one credential opens both.

The configured secret MUST be deliverable as a file at both ends, not only as an environment value. An environment value is readable from container inspection and from the process's own environment, and the add-on is the least trusted component in the deployment. Both ends MUST accept the same delivery forms under the same names: a secret that one end reads as a path and the other as a literal value is a defect this contract has already produced once, and its only symptom was a failure to authenticate.

The two derived keys MUST be domain-separated, and the configured secret MUST NOT be used directly as either. The derivation MUST be over the configured value's bytes after trimming surrounding whitespace, and MUST NOT decode the value even where it is expressed in a hexadecimal alphabet. A mounted secret ordinarily carries a trailing newline, and a one-byte difference in the derivation input is indistinguishable at both ends from a wrong secret.

Certificate verification against a private certificate authority MUST NOT be required, and pinning the derived key MUST be understood as replacing it rather than weakening it: a name check establishes that some authority vouched for a string, while the pin establishes that the peer holds the deployment secret. Where the pin is implemented by disabling the library's chain verification, the code MUST state that reason at the call site, because an unexplained verification bypass is either removed or distrusted by the next reader.

Every registered add-on's base URL MUST be HTTPS, and a target configured otherwise MUST NOT register. The rule MUST be unconditional and MUST be decidable from configuration alone. Registration occurs before any add-on is contacted, so the check cannot consult a manifest, and the requirement MUST NOT be written as though it could: a test asserting that a target *whose manifest declares secret parameters* is refused would describe a causal path the implementation does not have, and would pass for the wrong reason. There MUST be no exemption for local or development addresses.

The reason for the rule is confidentiality of the body — an operation declaring `secret_params` transmits a member's credential and, for the purge path, an elevated target credential, and a request signature establishes neither the confidentiality of that body nor the authenticity of the response. That reason is a property of the shipped contracts rather than of any one registration, so it MUST be anchored by a **separate** assertion that the supported contracts do declare such parameters. Two assertions, because there are two facts: the rule holds unconditionally, and the rule has a live reason.

The registered base URL MUST be the only authority the backend contacts for that target: an add-on's response MUST NOT redirect it. A followed redirect re-sends the body of a mutating call to a host the add-on chose, carrying the request signature that authenticates it there, and the redirect's own success would then be recorded against a target that never acted.

A registration lacking the material for the configured transport MUST NOT register at all, because calling the component that holds the target credential over an unauthenticated channel is worse than not having the target.

#### Scenario: A peer derived from a different secret cannot deliver a body

- **WHEN** the backend connects to a peer presenting a certificate whose key was not derived from that target's secret
- **THEN** the connection MUST fail during the handshake
- **AND** no request body MUST be written to it

#### Scenario: A correctly named certificate does not satisfy the pin

- **WHEN** a peer presents a certificate that is valid and carries the expected host name, but whose public key was not derived from the target's secret
- **THEN** the backend MUST refuse the connection

#### Scenario: A target with no configured secret does not register

- **WHEN** an add-on target is configured with no secret
- **THEN** it MUST NOT register
- **AND** the backend MUST NOT report it as authenticated by any mode

#### Scenario: A plaintext base URL is refused unconditionally

- **WHEN** a target is configured with an `http://` base URL
- **THEN** it MUST NOT register
- **AND** the refusal MUST hold for loopback and private addresses
- **AND** the refusal MUST NOT depend on having read the add-on's manifest

#### Scenario: The rule's reason is asserted separately from the rule

- **WHEN** the supported operation contracts are examined
- **THEN** at least one MUST declare `secret_params`
- **AND** this assertion MUST be independent of the registration check it justifies

#### Scenario: Two targets are configured with the same secret

- **WHEN** two add-on targets are configured with identical secret bytes
- **THEN** the affected targets MUST NOT register
- **AND** the backend MUST NOT register one of them in preference to the other
- **AND** the refusal MUST NOT depend on how the values were produced

#### Scenario: The secret is delivered as a file

- **WHEN** either end is configured to read its secret from a mounted file
- **THEN** it MUST derive the same keys as an end configured with the same value inline
- **AND** both ends MUST accept that delivery form under the same variable naming

#### Scenario: A trailing newline in the configured secret does not change the derivation

- **WHEN** one end reads the secret from a file ending in a newline and the other from an environment value without one
- **THEN** both MUST derive identical keys

## ADDED Requirements

### Requirement: The transport derivation MUST be fixed by a contract artifact asserted from both ends

The derivation MUST be pinned by a versioned artifact carrying a fixed secret and its expected outputs — for at least two distinct targets, the derived key material and public key, and at least one complete request signature with its resulting MAC. Both the backend and every add-on MUST assert against that same artifact.

The artifact MUST express the fixed secret unambiguously in both its textual form and the exact bytes entering the derivation, so that the encoding decision cannot be misread from the artifact itself. It MUST cover more than one target, because a derivation that ignored the target salt would otherwise pass every assertion while only one add-on exists.

This requirement exists because the derivation creates a place where two separately deployed binaries MUST agree byte for byte and nothing otherwise makes them. A disagreement about the hash, the salt, the domain-separation string, or the input encoding presents identically to a wrong secret, with each side internally consistent. A shared module MUST NOT be introduced to satisfy this: the two are separately deployed and a shared constant would conceal exactly the version skew the contract version exists to surface.

#### Scenario: One end changes the derivation

- **WHEN** either end's derivation stops matching the artifact
- **THEN** that module's test suite MUST fail
- **AND** the failure MUST name the derivation rather than presenting as an authentication error

#### Scenario: The salt is dropped

- **WHEN** an implementation derives keys without the target salt
- **THEN** the assertion MUST fail on at least one target in the artifact

### Requirement: Add-on transport keys MUST NOT be persisted or carry an operational expiry

The add-on's transport key MUST be derived at start-up and held in memory. It MUST NOT be written to disk, mounted, or distributed, and MUST NOT depend on a certificate authority that an operator maintains. Rotation MUST be replacement of the configured secret followed by **recreation** of both ends, and MUST NOT require a certificate ceremony.

The documented procedure MUST recreate the containers rather than restart them. A restart in place preserves the environment a container was created with, so a procedure written as "restart both" leaves both ends running the previous secret while reporting a successful rotation — the failure would be silent and the operator's belief about their own trust boundary would be wrong. The procedure MUST also state the ordering and the window: both ends cannot move atomically, so a bounded period exists in which one has rotated and the other has not, during which calls fail to authenticate rather than proceeding unauthenticated.

Where the procedure quiesces the add-on first, it MUST do so through the runtime lifecycle operation and MUST wait on the add-on's own drained signal. It MUST NOT instruct an operator to edit a start-up environment variable for this purpose: that value is read once at start-up, so editing it changes nothing in a running container, and applying it would require the very recreation the quiesce is meant to precede. Because the lifecycle operation travels over the transport being rotated, it MUST complete before the secret is replaced; afterwards the operator cannot reach the running add-on's lifecycle surface at all.

The signal-driven shutdown drain is a separate concern from this requirement and is specified by `addon-shutdown-grace-period`. Rotation MUST NOT depend on it: the quiesce above is a runtime operation completed before anything is stopped.

No transport certificate expiry MUST be surfaced on target health once no expiry exists. A health field that can only ever report an unknown or absent expiry is worse than its absence, because it reads as a probe that is failing rather than as a property that no longer applies.

#### Scenario: Restart produces a working transport with no stored material

- **WHEN** the add-on restarts
- **THEN** it MUST derive its key again from the configured secret
- **AND** the backend MUST connect without any material having been re-distributed

#### Scenario: Rotation

- **WHEN** the configured secret is replaced and both containers are recreated
- **THEN** the transport MUST work with no certificate issuance
- **AND** the procedure MUST NOT rely on an in-place restart, which does not re-read the configuration

#### Scenario: Only one end has rotated

- **WHEN** the secret has been replaced at one end and not the other
- **THEN** calls MUST fail to authenticate
- **AND** MUST NOT proceed under the previous secret

#### Scenario: The add-on is quiesced before the secret is replaced

- **WHEN** a rotation procedure quiesces the add-on first
- **THEN** it MUST use the runtime lifecycle operation and wait on the add-on's drained signal
- **AND** MUST NOT rely on editing a start-up environment variable, which a running container does not re-read

### Requirement: The per-target secret MUST be minted by a defined generator

Each target's secret MUST be produced by a generator rather than chosen, because neither weakness nor reuse is observable anywhere in the running system once configured. The generator MUST take the target name as an explicit argument and MUST be usable for a target added to an existing deployment: a generator that can only run at first setup does not cover the case it is most needed for, since targets arrive one at a time.

**Minting a target's secret MUST be a separate, explicitly privileged step, and MUST NOT be invoked from the unprivileged environment bootstrap.** The two have different privilege requirements and different inputs, and composing them is not possible rather than merely untidy: the environment bootstrap runs as the unprivileged deployment user — which the deployment requires, since that user also drives automated deploys — while setting the secret's ownership requires privilege that user does not have and must not be granted. The set of targets is also not available at bootstrap: it is a value the operator records in the generated environment file afterwards, so a bootstrap that iterated over it would be reading something it had just created empty.

The privileged step MUST NOT be satisfied by granting the deployment user membership of the add-on's group. That would make every add-on secret readable by the account that runs automated deploys, which is the opposite of what per-target isolation is for, and it would do so permanently to avoid a one-time action.

A target's secret MUST exist as **one** file, mounted read-only into both the backend and that add-on. It MUST NOT be written as two copies to be kept identical. The two ends hold the same bytes by definition — the scheme is symmetric, and neither end holds a half the other must not see — so a second copy carries no confidentiality and introduces a state in which the two disagree. Requiring two files to be created "as one operation" would specify an atomicity the filesystem cannot provide: an interrupt, a failed ownership change, or a host crash between the writes leaves one copy, and a generator that then refuses to run has stranded the deployment in exactly the split state it exists to prevent.

The generator MUST create that file atomically: written to a temporary path on the destination filesystem, given its final ownership and mode there, and only then published under its final name — so that no observer, and no subsequent run, can encounter a partially written or wrongly owned secret. It MUST remove its temporary artefacts on every exit path where it still runs — success, refusal, or error it can observe. It MUST NOT be specified as removing them on *any* failure, since this requirement elsewhere models a run killed without the chance to clean up; that case is covered by the temporary's restrictive mode and by the irrelevance rule below, not by a cleanup the process never reaches.

**Publication MUST use a primitive that fails when the destination exists.** Checking for absence and then renaming is not sufficient and MUST NOT be specified: POSIX `rename` replaces the destination silently, so two runs that both observe no file will both publish, and the later one destroys a secret the earlier may already have put into service — producing the split state between the two ends that this design exists to make impossible. The exclusive-creation guarantee MUST come from the publication step itself rather than from a preceding check, because any check performed before publication is separated from it by a window. A lock MAY additionally serialise concurrent runs, but MUST NOT be relied on as the guarantee: a lock that is not taken, or is taken on a different path, leaves the clobber available.

It MUST verify before writing anything that the destination is writable and that it holds whatever privilege the required ownership needs, and MUST fail before creating any file when it does not. That preflight is an ergonomic guard against a run that cannot possibly succeed; it is not the safety property, which is carried by the exclusive publication above.

The file's ownership and mode MUST be readable by both consumers as they actually run — the backend and an add-on that runs as an unprivileged uid against a read-only mount — and MUST NOT be world-readable on the host. The generator MUST refuse when the file already exists, and MUST emit the configuration lines naming it, so that no step of the mapping is left to be recalled.

**An interrupted run MUST leave exactly one of two states, and refusal is the correct outcome of one of them.** Because publication is exclusive and indivisible, an interruption falls either before it — leaving no secret, so a subsequent run publishes one — or after it, leaving a complete and correctly owned secret. In the second case the run produced the very thing it existed to produce, so the target is provisioned and a subsequent run MUST refuse. That refusal MUST be specified as the successful terminal state of the interrupted case rather than as a failure to be worked around: an operator who reads a bare refusal after an interruption cannot distinguish completion from breakage, and the tempting next action is to delete a live secret. The refusal MUST therefore distinguish *this target already has a secret and nothing needs doing* from every other reason the generator declines. Neither state MUST require manual repair, and there is no third state to repair.

A temporary MUST be created with its restrictive mode from the outset rather than widened or narrowed afterwards, since a run killed between creation and a later mode change would leave key material readable by anyone the interim mode allowed.

**A leftover temporary MUST NOT influence a subsequent run's outcome, which MUST be determined by the destination alone.** It MUST NOT be specified as permitting a subsequent run to succeed: publication and the removal of the temporary are separate steps, so a run killed between them leaves a complete secret *and* a stale temporary at once, and a rule that such a temporary permits success would contradict the refusal the finished secret requires. The correct invariant is that the temporary is not an input to the decision — where no secret exists the next run publishes one, where a secret exists it refuses, and in neither case does the temporary change which of those happens or cause a failure of its own.

Temporary paths MUST therefore be unique per run. A deterministic temporary path is the mechanism by which a stale one would become an input to the decision: the next run's exclusive creation of its own temporary would collide with the abandoned file and fail for a reason that has nothing to do with the state of the secret.

#### Scenario: A target is added to an existing deployment

- **WHEN** a new add-on target is configured on a deployment that already has others
- **THEN** minting its secret MUST be the same operation as for the first target
- **AND** MUST NOT require regenerating any existing target's secret

#### Scenario: The generator is interrupted before publication

- **WHEN** the generator fails or is interrupted at any point before publishing
- **THEN** no secret file MUST exist for that target
- **AND** a subsequent run MUST publish one successfully, without manual repair

#### Scenario: The generator is interrupted after publication

- **WHEN** the generator is interrupted after publishing but before completing
- **THEN** the secret MUST be complete and correctly owned
- **AND** the target MUST be treated as provisioned
- **AND** a subsequent run MUST refuse, reporting that the target already has a secret and that nothing needs doing
- **AND** no manual repair MUST be required to reach that state

#### Scenario: A killed run leaves a temporary behind

- **WHEN** a run is killed without executing its cleanup
- **THEN** any temporary it left MUST NOT be readable more widely than the finished secret would be
- **AND** a subsequent run's outcome MUST be determined by the destination alone
- **AND** the temporary MUST NOT cause a subsequent run to fail for a reason of its own

#### Scenario: A run is killed between publishing and removing its temporary

- **WHEN** a run is killed after `link` has published the secret but before the temporary is removed
- **THEN** both a complete secret and a stale temporary exist
- **AND** a subsequent run MUST refuse, because the destination holds a finished secret
- **AND** that refusal MUST report the target as already provisioned rather than blaming the temporary

#### Scenario: The destination cannot be written or owned as required

- **WHEN** the destination is not writable, or the required ownership cannot be set
- **THEN** the generator MUST fail before creating any file
- **AND** the failure MUST name the privilege the run lacked, not only the operation that failed

#### Scenario: The unprivileged bootstrap runs

- **WHEN** the environment bootstrap is run as the deployment user
- **THEN** it MUST NOT attempt to mint any add-on secret
- **AND** it MUST NOT fail on account of an add-on target having no secret yet

#### Scenario: A secret is minted without the required privilege

- **WHEN** the per-target generator is run by a user that cannot set the required ownership
- **THEN** it MUST refuse before creating anything
- **AND** MUST NOT fall back to weaker ownership or a wider mode

#### Scenario: The secret already exists

- **WHEN** the generator is run for a target whose secret file is present
- **THEN** it MUST refuse
- **AND** MUST NOT overwrite it
- **AND** the refusal MUST be distinguishable from a refusal caused by insufficient privilege or an unwritable destination

#### Scenario: Two generator runs race for the same target

- **WHEN** two runs for one target both observe no existing secret and both proceed to publish
- **THEN** exactly one MUST succeed and the other MUST fail
- **AND** the surviving file MUST be the one the successful run published
- **AND** neither run MUST replace a secret that is already in place

### Requirement: Add-on network segments MUST NOT be shared between add-ons

Each add-on MUST occupy a network segment shared only with the backend. Add-ons MUST NOT be co-resident on one segment with each other, because a container on a shared segment can observe traffic to its peers, and an add-on is the least trusted component in the deployment.

This is a structural control and MUST NOT be treated as replacing the transport authentication above, nor the reverse: the segmentation is what holds if the transport is misconfigured, and the transport is what holds if a segment is later widened.

#### Scenario: A second add-on is deployed

- **WHEN** a further add-on is added to the deployment
- **THEN** it MUST receive its own segment
- **AND** MUST NOT be able to reach or observe another add-on's traffic
