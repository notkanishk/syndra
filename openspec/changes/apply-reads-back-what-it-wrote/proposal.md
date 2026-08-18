# An apply reports what the target holds, not what it was asked for

## What is wrong

`convergeExisting` — the path every update to an account that already exists
goes through — did not read the account back. After `user.update` returned it
built the answer itself:

```go
applied := *current                      // then overwrote the managed fields
applied.Groups = desired.groups          // with the values it had REQUESTED
applied.Enabled = desired.enabled
...
Fingerprint: fingerprintSubject(&applied)
```

So the reported state agreed with the request by construction, and the
fingerprint the next plan checks staleness against digested a claim rather than
a reading. The create path has always read back, and its own comment states the
rule the update path broke:

> a fingerprint computed from a state this add-on invented is a fingerprint the
> next plan verifies against nothing

## Why it matters

The fingerprint exists to answer one question: has the subject moved since the
diff an operator approved? Computed from intent, it answers that question about
Syndra's own request, which cannot have moved. The check passes over exactly the
differences it exists to catch.

The divergence is ordinary, not exotic. TrueNAS normalises values on write. It
refuses `smb` on an account with no password. A middleware that coerces a field
answers `200` and stores something else. In every one of those the projection
reports a clean, confirmed write, and no read anywhere agrees with it.

It also blocks `reconciliation-as-merge`, which assumed this read already
happened on every path. A merge base written from a projection equals the
desired state by construction and can never produce a conflict — the failure
that change exists to remove, arriving through its own foundation.

## What changes

**Read back after every successful mutation.** Both paths converge on
`readBack`, and the fingerprint is computed from the read on both. The plan path
already reads every subject, so this is a second read per applied subject rather
than a new class of work.

**Report the observed managed fields.** The field is additive and its safety is
asserted rather than assumed — the backend's decoder is tested against an
outcome carrying it. `observed` carries the managed half of
the subject as the target reported it, for the consumer that will store merge
bases. Managed fields only: an unmanaged field is not "unchanged", it is out of
scope, and reporting it would invite a base claiming authority over something
Syndra never set.

**Never fall back to the projection.** When the write lands and the read after
it fails, the outcome says both things: the effect is `applied`, because it was,
and `unverified` is set with no fingerprint and no observed values. Reporting it
as a failure would invite a retry of a mutation already performed; reporting it
as an ordinary success would hand the next plan a fingerprint nobody read, which
is the defect arriving through the error path.

The operator-facing sentence travels in `detail`, because that is the field the
backend decodes — one carried only in `consequence` reaches no surface at all.

## What this is not

Not a retry. The add-on does not re-read in a loop and does not re-write: the
mutation is done, and the next plan reads the subject fresh anyway.

Not the merge base. Storing `observed` is `reconciliation-as-merge`; this change
is what makes there be something honest to store.
