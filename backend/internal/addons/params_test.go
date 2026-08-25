package addons

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func opWith(specs ...ParamSpec) EffectiveOperation {
	return EffectiveOperation{ID: "test.op", Available: true, Params: specs}
}

// P1 — backend policy's parameter schema is enforced rather than declared. An
// unvalidated dispatch would send whatever the caller wrote to an add-on that
// may act on all of it, which is an add-on-specific input reaching the target
// without passing the boundary meant to bound it.
func TestParameterSchemaIsEnforced(t *testing.T) {
	spec := opWith(
		ParamSpec{Name: "password", Type: "string", Required: true, Secret: true},
		ParamSpec{Name: "groups", Type: "string[]"},
		ParamSpec{Name: "enabled", Type: "bool"},
		ParamSpec{Name: "quota", Type: "int"},
	)

	valid := []struct {
		name   string
		params map[string]any
	}{
		{"required only", map[string]any{"password": "hunter2"}},
		{"all of them", map[string]any{
			"password": "hunter2", "groups": []any{"lab", "printing"},
			"enabled": true, "quota": float64(10),
		}},
		{"native go slice", map[string]any{"password": "x", "groups": []string{"lab"}}},
		{"json number integer", map[string]any{"password": "x", "quota": json.Number("7")}},
		{"optional explicitly null", map[string]any{"password": "x", "enabled": nil}},
	}
	for _, tc := range valid {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			if err := ValidateParams(spec, tc.params); err != nil {
				t.Fatalf("rejected a valid set: %v", err)
			}
		})
	}

	invalid := []struct {
		name   string
		params map[string]any
	}{
		{"unknown key", map[string]any{"password": "x", "shell": "/bin/sh"}},
		{"missing required", map[string]any{"groups": []any{"lab"}}},
		{"required explicitly null", map[string]any{"password": nil}},
		{"required present but blank", map[string]any{"password": "   "}},
		{"string given a number", map[string]any{"password": float64(5)}},
		{"bool given a string", map[string]any{"password": "x", "enabled": "true"}},
		{"int given a fraction", map[string]any{"password": "x", "quota": 1.5}},
		{"array of non-strings", map[string]any{"password": "x", "groups": []any{"lab", 3}}},
		{"array given a scalar", map[string]any{"password": "x", "groups": "lab"}},
	}
	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			err := ValidateParams(spec, tc.params)
			if !errors.Is(err, ErrInvalidParams) {
				t.Fatalf("err = %v, want ErrInvalidParams", err)
			}
		})
	}
}

// P1 — no refusal names a value. Parameter names are contract; values may be
// secrets, and an error string is logged, returned to a client, and captured in
// a trace.
func TestAValidationErrorNeverNamesAValue(t *testing.T) {
	spec := opWith(
		ParamSpec{Name: "password", Type: "string", Required: true, Secret: true},
		ParamSpec{Name: "quota", Type: "int"},
	)
	err := ValidateParams(spec, map[string]any{
		"password": "correct-horse-battery-staple",
		"quota":    "not-a-number-but-a-memorable-string",
		"smuggled": "another-memorable-string",
	})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	for _, value := range []string{"correct-horse", "not-a-number-but", "another-memorable"} {
		if strings.Contains(err.Error(), value) {
			t.Fatalf("the refusal echoed a submitted value: %v", err)
		}
	}
	// It must still be actionable: the names of the offending parameters are
	// the contract, and withholding them would make the error useless.
	for _, name := range []string{"quota", "smuggled"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the refusal does not name the parameter %q: %v", name, err)
		}
	}
}

// P1 — an operation declaring no parameters accepts none. The empty schema is a
// schema, not an absence of one.
func TestAnOperationWithNoParametersAcceptsNone(t *testing.T) {
	if err := ValidateParams(opWith(), nil); err != nil {
		t.Fatalf("an empty set against an empty schema must pass: %v", err)
	}
	if err := ValidateParams(opWith(), map[string]any{"anything": 1}); !errors.Is(err, ErrInvalidParams) {
		t.Fatal("an operation declaring no parameters accepted one")
	}
}

// P1 — a type this validator cannot check is refused, not waved through. "I do
// not know how to verify this" and "this is fine" must never be the same
// branch.
func TestAnUncheckableTypeIsRefused(t *testing.T) {
	spec := opWith(ParamSpec{Name: "thing", Type: "matrix", Required: true})
	if err := ValidateParams(spec, map[string]any{"thing": "whatever"}); !errors.Is(err, ErrInvalidParams) {
		t.Fatal("a parameter of an unsupported type was accepted")
	}
}

// 2.32 — every parameter the real policy table declares uses a type this
// validator implements. A policy entry whose type the validator cannot check
// would refuse every invocation of that operation, at runtime, in production.
func TestEveryPolicyParameterTypeIsCheckable(t *testing.T) {
	for id, p := range operationPolicy {
		for _, spec := range p.Params {
			// A value of the wrong shape for any implemented type; what matters
			// is that the failure is a type mismatch and not "unsupported type".
			err := checkParamType(spec, struct{}{})
			if err != nil && strings.Contains(err.Error(), "unsupported type") {
				t.Errorf("policy %s declares parameter %q of type %q, which ValidateParams cannot check — "+
					"every call to this operation would be refused", id, spec.Name, spec.Type)
			}
		}
	}
}

// P2 — the schema is checked before any value is, and for every declared
// parameter rather than only the supplied ones. Checked lazily, an OPTIONAL
// parameter of an unverifiable type would be enforced when present and waved
// through when omitted, so leaving it out would make the whole operation
// callable — the fail-closed rule inverted by absence.
func TestAnUnsupportedTypeIsRefusedEvenWhenTheValueIsOmitted(t *testing.T) {
	spec := opWith(
		ParamSpec{Name: "password", Type: "string", Required: true},
		ParamSpec{Name: "shape", Type: "matrix"}, // optional, and uncheckable
	)

	// The value is absent, which is exactly the case that used to pass.
	if err := ValidateParams(spec, map[string]any{"password": "x"}); !errors.Is(err, ErrInvalidParams) {
		t.Fatal("an operation declaring an unverifiable optional parameter was callable by omitting it")
	}
	// And still refused when supplied, so neither branch is the lenient one.
	if err := ValidateParams(spec, map[string]any{"password": "x", "shape": "whatever"}); !errors.Is(err, ErrInvalidParams) {
		t.Fatal("an unverifiable parameter was accepted when supplied")
	}
}

// P2 — the two places that decide what a type means agree. A type declarable
// but uncheckable, or checkable but undeclarable, is a disagreement that shows
// up as an operation nobody can call.
func TestTheSupportedTypeListMatchesTheChecker(t *testing.T) {
	for _, typ := range []string{"string", "string[]", "bool", "int"} {
		if !supportedParamType(typ) {
			t.Errorf("checkParamType implements %q but supportedParamType rejects it", typ)
		}
		err := checkParamType(ParamSpec{Name: "p", Type: typ}, struct{}{})
		if err != nil && strings.Contains(err.Error(), "unsupported type") {
			t.Errorf("supportedParamType accepts %q but checkParamType cannot verify it", typ)
		}
	}
	if supportedParamType("matrix") {
		t.Error("supportedParamType accepted a type the checker does not implement")
	}
}
