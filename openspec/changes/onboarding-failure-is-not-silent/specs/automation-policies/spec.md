## ADDED Requirements

### Requirement: A welcome-bundle assignment fault MUST NOT be reported as a delivered webhook

`processUserCreated` MUST NOT report an onboarding failure as a successful webhook delivery. A fault that a retry could plausibly resolve (a database error, a bundle-assignment failure) MUST propagate so the webhook event is marked failed and the caller retries. `ErrNoWelcomeBundleConfigured` is the one exception — retrying it changes nothing — and MUST still acknowledge the delivery, but MUST NOT be conflated with the fault case: the two differ in whether Zitadel retries, not in whether an operator can see what happened.

#### Scenario: A real fault is retried, not swallowed

- **WHEN** welcome-bundle assignment fails for a reason other than `ErrNoWelcomeBundleConfigured`
- **THEN** the webhook event MUST be marked failed
- **AND** the response MUST NOT be 200

#### Scenario: A missing bundle acknowledges without retrying

- **WHEN** welcome-bundle assignment fails with `ErrNoWelcomeBundleConfigured`
- **THEN** the webhook event MUST be marked completed and the response MUST be 200
- **AND** the onboarding trigger MUST NOT be marked `failed`

### Requirement: A missing welcome bundle MUST be distinguishable from a defect

`onboarding_triggers.status` MUST have a value for "understood, nothing to give" (`unconfigured`) distinct from `failed`. A deployment with no welcome bundle configured MUST NOT accumulate rows that read as defects; an operator queue MUST be able to tell a config gap from a fault without reading the error text.

#### Scenario: No welcome bundle configured is not a failure

- **WHEN** an onboarding trigger fires and no bundle is marked `is_welcome`
- **THEN** the trigger MUST be recorded with `status = 'unconfigured'`, never `status = 'failed'`

#### Scenario: A real fault is still a failure

- **WHEN** an onboarding trigger fires and welcome-bundle assignment fails for any other reason
- **THEN** the trigger MUST be recorded with `status = 'failed'`

### Requirement: The system MUST be able to answer "who joined and got no welcome bundle" from state

A reconciler MUST answer this question by reading current state — who the directory confirms is an active member, and who actually holds the welcome bundle — rather than relying on the onboarding trigger log, which reflects only events a webhook happened to report. A webhook that never arrives MUST NOT make a missed onboarding permanently invisible.

The answer MUST carry whether a welcome bundle is configured at all as an explicit fact, separate from the list of missed people: no bundle configured means nobody can be checked against a default that does not exist, which MUST NOT be reported the same way as "everybody has it."

#### Scenario: A silently missed webhook is still found

- **WHEN** a person is an active member of the directory and does not hold the welcome bundle, regardless of whether any onboarding_triggers row exists for them
- **THEN** the reconciler MUST include them in the missed list

#### Scenario: No welcome bundle configured reports the gap, not a false positive

- **WHEN** no bundle is marked `is_welcome`
- **THEN** the reconciler MUST report `welcome_bundle_configured = false` and an empty missed list, never every active person as "missed"

#### Scenario: An inactive person is not "missed"

- **WHEN** a person is deactivated, locked, or no longer in the directory
- **THEN** the reconciler MUST NOT include them in the missed list

### Requirement: A person who joined and got no welcome bundle MUST surface where operator queues live

The operator landing page MUST show both onboarding gaps this reconciler finds: the fact that no welcome bundle is configured (linking to where one is set), and each named person found holding none, following the same shape as every other queue block on that page (a count, a link into the thing it counts).

#### Scenario: The gap is counted in the queue

- **WHEN** no welcome bundle is configured, or the reconciler finds at least one missed person
- **THEN** the landing page's work queue MUST include this among the things needing attention, and MUST NOT report "nothing needs you"
