# Tasks

## 1. The read

- [x] 1.1 `converge` calls `readBack` after a successful `user.update` and fingerprints the READ. The projection — `applied := *current` with the requested values written over it — is gone, and with it the only path that reported intent in the shape of an observation
- [x] 1.2 `observed` carries the managed fields as the target reported them, on both the update and the create path. Managed fields only: an unmanaged field is out of scope rather than unchanged
- [x] 1.3 A failed read-back sets `unverified`, omits the fingerprint and omits `observed`. The effect stays `applied`, because the write happened — calling it a failure invites a retry of a mutation already performed
- [x] 1.4 The unverified sentence is in `detail`, which the backend decodes, not only in `consequence`, which it does not

## 2. Coverage

- [x] 2.1 The fake target now applies writes to its own fixture, and `divergeOn` makes it store something else. It did not before, which is a fake agreeing with the defect: a fixture that never moved was indistinguishable from one that had
- [x] 2.2 A divergent post-write state — the write says unlock, the target keeps it locked — asserts the reported fingerprint equals a fresh read's, which is the invariant stated as the next plan would state it
- [x] 2.3 `observed` carries only managed fields
- [x] 2.4 A write followed by an unreadable target reports applied + unverified, with no fingerprint, no observed, the sentence an operator sees, and exactly ONE write
- [x] 2.5 The backend tolerates both new fields on decode, asserted on the backend's own decoder rather than assumed. The one defect class this boundary has produced twice is a field one side sends and the other's decoder refuses
- [x] 2.6 Mutation-checked: restoring the projection fails 2.2 and 2.4
- [x] 2.7 `addons/contract/apply_response.json`, asserted from both ends — the add-on encodes it, the backend decodes it and keeps every field it reads — plus the leniency itself, so a field the backend does not declare is proven not to break the call rather than assumed not to. Mutation-checked by editing the fixture, which fails both suites in both modules. Also asserts that an unverified outcome carries neither `observed` nor `fingerprint`, which is a contract statement rather than an implementation detail: a consumer storing merge bases must be unable to receive one from a write nobody read back
