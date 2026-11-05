# One path: tasks

## W1b. Error envelope
- [x] Move the envelope to `handlers/errors.go`; sanitise raw upstream text at 502/503/504 `ready-for-agent`
- [x] Proxy errors use the envelope; gateway text never reaches the screen
- [x] `merge_findings.go` and `claim_shape.go` bodies use the envelope
- [ ] `writeUpstream` at every raw call site, then delete `rawUpstreamCodes` (needs W1a, W1e files) `needs-triage`
- [ ] Route `bundles.go`, `access.go`, `claimserve/door.go`, the add-on writer through one envelope type `needs-triage`
