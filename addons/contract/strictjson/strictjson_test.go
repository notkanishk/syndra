package strictjson

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type Inner struct {
	Name string `json:"name"`
}

type Promoted struct {
	Shared string `json:"shared"`
	Deep   string `json:"deep"`
}

type request struct {
	Version  int                        `json:"contract_version"`
	Subject  string                     `json:"subject"`
	Kind     string                     `json:"k,omitempty"`
	Hidden   string                     `json:"-"`
	Dash     string                     `json:"-,"`
	Untagged string                     // named Untagged, exactly
	Nested   *Inner                     `json:"nested"`
	List     []Inner                    `json:"list"`
	Desired  map[string]json.RawMessage `json:"desired"`
	Free     map[string]Inner           `json:"free"`
	Raw      json.RawMessage            `json:"raw"`
	Anything any                        `json:"anything"`
	Promoted
	Shared string `json:"shared"` // shallower than Promoted.Shared: this one wins
}

func decode(body string) (request, error) {
	var r request
	err := Decode([]byte(body), &r)
	return r, err
}

func TestExactNamesDecode(t *testing.T) {
	r, err := decode(`{"contract_version":2,"subject":"wwe-the-rock","k":"x","-":"d","Untagged":"u",
		"nested":{"name":"n"},"list":[{"name":"a"},{"name":"b"}],"desired":{"Group":["x"],"group":["y"]},
		"free":{"A":{"name":"x"},"a":{"name":"y"}},"raw":{"Any":1,"any":2},"anything":{"X":{"x":[1]}},
		"deep":"promoted","shared":"outer","subject":"wwe-the-rock"}`)
	if err == nil || !strings.Contains(err.Error(), `duplicate member "subject"`) {
		t.Fatalf("an escaped spelling of a name is that name (RFC 8259 §8.3), so twice is a duplicate; got %v", err)
	}
	r, err = decode(`{"contract_version":2,"subject":"wwe-the-rock","k":"x","-":"d","Untagged":"u",
		"nested":{"name":"n"},"list":[{"name":"a"},{"name":"b"}],"desired":{"Group":["x"],"group":["y"]},
		"free":{"A":{"name":"x"},"a":{"name":"y"}},"raw":{"Any":1,"any":2},"anything":{"X":{"x":[1]}},
		"deep":"promoted","shared":"outer"}`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Subject != "wwe-the-rock" || r.Dash != "d" || r.Untagged != "u" || r.Nested.Name != "n" || len(r.List) != 2 ||
		len(r.Desired) != 2 || len(r.Free) != 2 || r.Deep != "promoted" || r.Shared != "outer" || r.Promoted.Shared != "" {
		t.Errorf("decoded %+v", r)
	}
}

func TestNamesAreMatchedExactly(t *testing.T) {
	for _, tc := range []struct{ why, body, object, name string }{
		{"a case variant", `{"Subject":"wwe-the-rock"}`, "", "Subject"},
		{"a long s, which Unicode simple folding makes an s", `{"ſubject":"wwe-the-rock"}`, "", "ſubject"},
		{"the Kelvin sign, which folds to k", "{\"\u212a\":\"x\"}", "", "\u212a"},
		{"a case variant inside a nested object", `{"nested":{"Name":"x"}}`, "/nested", "Name"},
		{"a case variant inside an array element", `{"list":[{"name":"a"},{"NAME":"b"}]}`, "/list/1", "NAME"},
		{"a case variant inside a map's value", `{"free":{"Any key":{"nAme":"x"}}}`, "/free/Any key", "nAme"},
		{"the Go name of a tagged field", `{"Subject":"x","subject":"y"}`, "", "Subject"},
		{"a field tagged -", `{"Hidden":"x"}`, "", "Hidden"},
		{"an unknown name", `{"conformance_unknown_field":true}`, "", "conformance_unknown_field"},
		{"the pointer escapes / and ~ (RFC 6901)", `{"free":{"a/b~c":{"x":1}}}`, "/free/a~1b~0c", "x"},
	} {
		_, err := decode(tc.body)
		var unknown *UnknownMemberError
		if !errors.As(err, &unknown) || unknown.Object != tc.object || unknown.Name != tc.name {
			t.Errorf("%s: got %v, want unknown member %q in %q", tc.why, err, tc.name, tc.object)
			continue
		}
		if !strings.Contains(err.Error(), `"`+tc.name+`"`) {
			t.Errorf("%s: the error %q does not name the member", tc.why, err)
		}
	}
}

func TestADuplicateNameIsRefused(t *testing.T) {
	for _, tc := range []struct{ why, body, object, name string }{
		{"at the top level", `{"subject":"conformance-x","subject":"wwe-the-rock"}`, "", "subject"},
		{"inside a nested object", `{"nested":{"name":"a","name":"b"}}`, "/nested", "name"},
		{"as a map key", `{"desired":{"group":["a"],"group":[]}}`, "/desired", "group"},
		{"inside a RawMessage", `{"raw":{"a":1,"a":2}}`, "/raw", "a"},
		{"inside an any", `{"anything":[{"a":1,"a":2}]}`, "/anything/0", "a"},
	} {
		_, err := decode(tc.body)
		var dup *DuplicateMemberError
		if !errors.As(err, &dup) || dup.Object != tc.object || dup.Name != tc.name {
			t.Errorf("%s: got %v, want duplicate member %q in %q", tc.why, err, tc.name, tc.object)
		}
	}
}

// A type with its own UnmarshalJSON reads its own names; only a repeat is
// refused around it.
type custom struct{ got map[string]any }

func (c *custom) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &c.got) }

func TestACustomUnmarshalerTakesAnyNameButNoneTwice(t *testing.T) {
	var v struct {
		C custom `json:"c"`
	}
	if err := Decode([]byte(`{"c":{"Whatever":1}}`), &v); err != nil || v.C.got["Whatever"] == nil {
		t.Errorf("got %v, %v", err, v.C.got)
	}
	var dup *DuplicateMemberError
	if err := Decode([]byte(`{"c":{"x":1,"x":2}}`), &v); !errors.As(err, &dup) {
		t.Errorf("got %v, want a duplicate", err)
	}
}

// Two embedded structs naming the same member at one depth cancel, as in
// encoding/json: neither is a field, so the name is unknown.
func TestAnAmbiguousPromotedNameIsNoField(t *testing.T) {
	type A struct{ X string }
	type B struct{ X string }
	var v struct {
		A
		B
	}
	var unknown *UnknownMemberError
	if err := Decode([]byte(`{"X":"y"}`), &v); !errors.As(err, &unknown) {
		t.Errorf("got %v, want unknown member X", err)
	}
}

func TestMalformedAndTrailingInputIsRefused(t *testing.T) {
	for _, body := range []string{`{"subject":"x"}{}`, `{"subject":"x"} }`, `{"subject":`, `{"contract_version":"two"}`, ``} {
		if _, err := decode(body); err == nil {
			t.Errorf("%q was accepted", body)
		}
	}
	if _, err := decode(" {\"contract_version\":1e400} \n"); err == nil {
		t.Error("a number out of an int's range was accepted")
	}
}
