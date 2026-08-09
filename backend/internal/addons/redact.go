package addons

// redactedValue replaces a secret parameter's value everywhere the parameter
// set leaves the request path — audit rows, outbox payloads, log lines, error
// strings. It is a fixed token rather than a length-preserving mask: a mask
// that preserves length leaks length, and length is most of what an attacker
// wants to know about a password they cannot read.
const redactedValue = "[REDACTED]"

// RedactedParams returns a copy of params safe to persist or log.
//
// The secret set is resolved from the effective operation — policy ∩ manifest —
// not taken from the caller. That is the whole point: a caller that forgot to
// list its secret parameter is exactly the caller whose secret would otherwise
// be written, and the backend already knows which parameters are secret because
// its own policy table declares them.
//
// It fails closed. If the operation cannot be resolved — unregistered target,
// no manifest yet, unknown id — every value is redacted rather than none. The
// state in which the backend does not know what is secret is the state in which
// everything is treated as secret; the keys survive, so a diagnostic still shows
// which parameters were present.
func RedactedParams(target, operation string, params map[string]any) map[string]any {
	op, err := ResolveOperation(target, operation)
	if err != nil {
		return redactParams(params, nil, true)
	}
	return redactParams(params, op.SecretParams, false)
}

// redactParams copies params, replacing values whose key names a secret. With
// all set, every value is replaced regardless of the name list.
//
// It recurses into nested maps and slices. Policy declares flat parameters, so
// nesting should not occur — but "should not occur" is not a property the
// redactor can rely on when the cost of being wrong is a password in an audit
// row, and the cost of walking is a dozen lines.
func redactParams(params map[string]any, secret []string, all bool) map[string]any {
	if params == nil {
		return nil
	}
	names := make(map[string]struct{}, len(secret))
	for _, s := range secret {
		names[s] = struct{}{}
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		if _, isSecret := names[k]; isSecret || all {
			out[k] = redactedValue
			continue
		}
		out[k] = redactValue(v, names, all)
	}
	return out
}

func redactValue(v any, names map[string]struct{}, all bool) any {
	switch t := v.(type) {
	case map[string]any:
		inner := make(map[string]any, len(t))
		for k, iv := range t {
			if _, isSecret := names[k]; isSecret || all {
				inner[k] = redactedValue
				continue
			}
			inner[k] = redactValue(iv, names, all)
		}
		return inner
	case []any:
		inner := make([]any, len(t))
		for i, iv := range t {
			inner[i] = redactValue(iv, names, all)
		}
		return inner
	default:
		return v
	}
}
