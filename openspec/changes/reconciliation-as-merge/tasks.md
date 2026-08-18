# Tasks

## 1. The base

- [ ] 1.1 A `merge_base` per `(subject, target)`, written from the read-back the apply already performs. Intent MUST NOT be recorded as the base: a base equal to `OURS` by construction can never produce a conflict, which is today's behaviour with more machinery
- [ ] 1.2 Migration adds the column; every existing binding has none, and a missing base converges exactly as today. Inventing one either fabricates agreement or raises a conflict for every managed subject on the first pass
- [ ] 1.3 The base is recorded on `already merged` too, or a hand-made change that matched Syndra's intent is re-detected forever

## 2. Classification

- [ ] 2.1 Per-FIELD classification into the six outcomes, over managed fields only. A whole-account comparison manufactures conflicts between a group change and a password change, which touched nothing of each other's
- [ ] 2.2 `already merged` is evaluated before `theirs-only`. Somebody who made the change Syndra was going to make has not drifted, and telling them so is how a system trains people to ignore it
- [ ] 2.3 A sweep applies `fast-forward` and records `already merged`, and resolves nothing else
- [ ] 2.4 Tests for all six, including the two that are indistinguishable without a base — `theirs-only` versus `conflict` — which is the pair that justifies the whole change

## 3. Findings and resolution

- [ ] 3.1 `conflict` and `deleted upstream` become operator findings carrying all three values, because "what did it used to be" is the question an operator asks first and cannot currently answer
- [ ] 3.2 Keep-ours / take-theirs / edit, each recording the base afterwards
- [ ] 3.3 Take-theirs changes the DESIRED state, not an ignore flag. An ignore re-raises next sweep; adopting agrees
- [ ] 3.4 No automatic resolution, and no flag for one — a flag would be set on the deployment where being wrong costs most

## 4. Surfaces

- [ ] 4.1 Conflicts on the target page beside the drift count, with the three values shown
- [ ] 4.2 The governance summary counts them, so a conflict cannot sit unseen behind a landing page that says nothing needs you
- [ ] 4.3 The audit record names which side won and who chose it — today it records only that a value changed
