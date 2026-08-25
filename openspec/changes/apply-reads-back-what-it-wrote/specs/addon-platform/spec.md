## ADDED Requirements

### Requirement: An apply MUST report the state the target holds, never the state it was asked for

An add-on MUST read the account as the target holds it after every successful mutation, and MUST derive the reported fingerprint from that read. A state assembled from the requested values MUST NOT be reported as the subject's state after the call, on any path.

A reported state assembled from the request agrees with the request by construction. The fingerprint exists to answer whether the subject has moved since the approved diff, and computed from intent it answers that question about the request, which cannot have moved — so the check passes over exactly the differences it exists to catch. Targets normalise values on write, refuse fields conditionally, and coerce; each of those is invisible to an answer built from what was asked.

The managed fields as the target reported them MUST be available to the caller. Fields the deployment does not manage for that subject MUST NOT be reported among them: an unmanaged field is out of scope rather than unchanged.

#### Scenario: The target stores something other than what was written

- **WHEN** an apply updates an account and the target stores a different value
- **THEN** the reported fingerprint MUST equal the fingerprint of a fresh read
- **AND** the reported observed values MUST be the target's, not the request's

#### Scenario: Only managed fields are reported as observed

- **WHEN** an apply manages one field of a subject
- **THEN** only that field MUST appear among the observed values

### Requirement: A mutation whose result cannot be read MUST say both things

When a mutation succeeds and the read that verifies it fails, the outcome MUST report that the change was applied AND that its result is unverified. It MUST carry no fingerprint and no observed values.

Reporting it as a failure invites a retry of a mutation already performed. Reporting it as an ordinary success hands the next plan a fingerprint nobody read, which is the failure this requirement exists to prevent, reached through the error path.

The statement MUST travel in a field the caller decodes. A consequence carried only in a field the caller ignores reaches no operator surface.

#### Scenario: The target stops answering between the write and the read

- **WHEN** a mutation succeeds and the account cannot then be read
- **THEN** the outcome MUST report the change as applied and unverified
- **AND** MUST carry no fingerprint and no observed values
- **AND** MUST NOT repeat the mutation
