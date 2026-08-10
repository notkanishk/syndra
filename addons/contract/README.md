# The add-on wire contract, as artifacts

These files are the request bodies the backend sends and the add-on decodes.
They exist because the two are separately compiled modules with no shared type,
and the failure that follows from that is not theoretical: the backend's
envelopes carried `contract_version` (and `operation`, `plan_id`, `fingerprint`
on the operation leg) while the add-on's structs declared none of them. The
add-on decodes strictly, so **every real `/apply` and `/operations/*` call would
have been answered `400 BAD_REQUEST`** — and neither side's tests could see it,
because each was exercised against its own fake and the two fakes agreed with
each other. That is the same shape as the defect §13.1 records against
`truenas_api.Client.Call`.

So each fixture is held from both ends:

| File | Backend assertion | Add-on assertion |
|---|---|---|
| `apply_request.json` | `json.Marshal(applyEnvelope{…})` is byte-equal after normalisation | `decodeStrict` into `ApplyRequest` succeeds and every field lands |
| `operation_request.json` | `json.Marshal(callEnvelope{…})` is byte-equal after normalisation | `decodeStrict` into `OperationRequest` succeeds and every field lands |
| `plan_request.json` | `json.Marshal(planEnvelope{…})` is byte-equal after normalisation | `decodeStrict` into `PlanRequest` succeeds and every field lands |

Every field is populated on purpose. A field the sender drops and a field the
receiver never declared both fail, because the comparison is over the whole
document rather than over the fields somebody remembered to check.

Changing one of these files is a wire-contract change. It fails two test suites
in two modules until both ends move, which is the point.
