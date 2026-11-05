# One standard path per operation

Owner, 5 Nov 2026. Source: the architecture audit (family 8 and the wave plan). Each wave names the files it owns.

## W1b. Error envelope (audit item 6)

Problem, verified on main: `jsonErrorResponse` hid `err.Error()` only at exactly 500 (`handlers/router.go:685-700`), so raw Zitadel and add-on text reached the screen at 502 and 503 (`discovery.go:136-432`, `drift.go:417`, `propagations.go:21,46`, `targets.go:513`, `inventory.go:161,181,226`, `entitlements.go:211`, `profile.go:26,69`). The UI proxy put prose in `error` (`route.ts` 403, 401, 400, 413, 502), and passed a gateway page through as text. `merge_findings.go:395` sent `err.Error()` in the `error` slot; `claim_shape.go:203` sent the developer string `save again with acknowledge_breaking` as the message.

Done:
- `handlers/errors.go` holds the envelope, `jsonErrorResponse`, `supportReference`, `jsonValidationErrorResponse`. A 500, and a 502/503/504 whose code is in `rawUpstreamCodes`, logs the raw text with a reference and sends a plain sentence plus `details.request_id`.
- The proxy speaks the same shape (`{error: CODE, message, details?}`); a non-JSON gateway error keeps its status and Retry-After and loses its text.
- `ApiError` never shows a bare string body at 5xx and has a fallback sentence per status. `MergeFindings` and `ShapeChooser` show the reference.
- The two hand-built bodies now embed `ErrorResponse` with typed extras.

Not done (files owned by other waves, so left alone): 5xx call sites that pass an authored sentence are not touched on purpose, because some carry safety ("the change may have applied"). The code list is the ceiling. The upgrade is a `writeUpstream(w, status, code, err)` at each raw call site once W1a/W1e land, and then `rawUpstreamCodes` goes away. `bundles.go:65,339`, `access.go:335`, `claimserve/door.go:338` and `addons/truenas/server.go:416` are still hand-built (UNVERIFIED whether they matter to the screen). `outcome.ts` code map (`CODES`, `isCode`) is not built: the envelope now carries the sentence, so the UI has no code-to-copy table to keep.
