package addons

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidParams is every way a parameter set can fail its schema. One error
// value rather than several, because the caller's action is the same in all of
// them — fix the request — and because distinguishing "unknown key" from "wrong
// type" to an unauthorised caller tells them what the backend expects.
var ErrInvalidParams = errors.New("addon: parameters do not match the operation's schema")

// ValidateParams checks a parameter set against the effective operation's
// schema — backend policy's schema, since the effective operation takes its
// parameters from policy and not from the manifest.
//
// Without this the schema is decorative. Policy declares that password.set
// takes one required string named `password`, and an unvalidated dispatch would
// happily send `{"password": 5, "shell": "/bin/sh"}` to an add-on that may well
// act on both — which is an add-on-specific input reaching the target without
// passing the backend boundary that is supposed to bound it. Unknown keys are
// rejected rather than dropped, because a caller sending a key the backend does
// not know is a caller with a different idea of the contract, and silently
// discarding it makes the two disagree without either finding out.
//
// No error mentions a VALUE. Parameter names are contract, values may be
// secrets, and an error string is one of the places a value would otherwise
// come to rest — an error is logged, returned to a client, and captured in a
// trace.
func ValidateParams(op EffectiveOperation, params map[string]any) error {
	var problems []string

	allowed := make(map[string]ParamSpec, len(op.Params))
	for _, spec := range op.Params {
		allowed[spec.Name] = spec
		// The schema itself is checked before any value is, and for every
		// declared parameter rather than only the supplied ones. Checked lazily,
		// a policy entry of a type this validator cannot verify would be
		// enforced when the parameter happened to be present and waved through
		// when it was omitted — so an optional parameter would make the whole
		// operation callable with the unverifiable field simply left out, which
		// is the fail-closed rule inverted by absence.
		if !supportedParamType(spec.Type) {
			problems = append(problems, fmt.Sprintf("parameter %q declares unsupported type %q", spec.Name, spec.Type))
		}
	}

	for name := range params {
		if _, ok := allowed[name]; !ok {
			problems = append(problems, fmt.Sprintf("unknown parameter %q", name))
		}
	}
	for _, spec := range op.Params {
		v, present := params[spec.Name]
		if !present || v == nil {
			if spec.Required {
				problems = append(problems, fmt.Sprintf("missing required parameter %q", spec.Name))
			}
			continue
		}
		if err := checkParamType(spec, v); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) == 0 {
		return nil
	}
	// Sorted so the message is stable across map iteration order: an error that
	// reorders itself between runs is one nobody can grep a log for.
	sort.Strings(problems)
	return fmt.Errorf("%w: %s: %s", ErrInvalidParams, op.ID, strings.Join(problems, "; "))
}

// checkParamType reports a type mismatch by name and expected type only.
//
// Values arrive decoded from JSON, so an integer is a float64 (or a
// json.Number under a decoder configured for it). Both are accepted when they
// are integral; a fractional value is not an int, and treating it as one would
// silently truncate a caller's mistake into a plausible-looking number.
func checkParamType(spec ParamSpec, v any) error {
	bad := func() error {
		return fmt.Errorf("parameter %q must be %s", spec.Name, spec.Type)
	}
	switch spec.Type {
	case "string":
		s, ok := v.(string)
		if !ok {
			return bad()
		}
		// A required parameter that is present and empty is a bug in the
		// caller, and on the one operation that has a required string it is a
		// bug that would set an empty password.
		if spec.Required && strings.TrimSpace(s) == "" {
			return fmt.Errorf("parameter %q is required and must not be empty", spec.Name)
		}
	case "string[]":
		items, ok := v.([]any)
		if !ok {
			if _, isStrings := v.([]string); isStrings {
				return nil
			}
			return bad()
		}
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("parameter %q must contain only strings", spec.Name)
			}
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return bad()
		}
	case "int":
		switch n := v.(type) {
		case int:
		case int64:
		case float64:
			if n != float64(int64(n)) {
				return bad()
			}
		case json.Number:
			if _, err := n.Int64(); err != nil {
				return bad()
			}
		default:
			return bad()
		}
	default:
		// Unreachable: ValidateParams refuses an unsupported type before any
		// value is inspected. Kept as a refusal anyway, because the two
		// branches must never disagree about what "I cannot check this" means.
		return fmt.Errorf("parameter %q declares unsupported type %q", spec.Name, spec.Type)
	}
	return nil
}

// supportedParamType reports whether checkParamType implements this type. The
// single list both consult, so a type can never be declarable but uncheckable.
func supportedParamType(t string) bool {
	switch t {
	case "string", "string[]", "bool", "int":
		return true
	default:
		return false
	}
}
