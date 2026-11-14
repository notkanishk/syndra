package strictjson

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
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
		if !strings.Contains(err.Error(), strconv.QuoteToASCII(tc.name)) {
			t.Errorf("%s: the error %q does not name the member", tc.why, err)
		}
	}
}

// An error names the member in ASCII, so a look-alike shows as what it is,
// and repeats at most 64 bytes of anything the caller sent.
func TestAnErrorShowsTheNameAsItIsAndNoMore(t *testing.T) {
	_, err := decode("{\"\u212a\":\"x\"}")
	if err == nil || err.Error() != `unknown member "\u212a"` {
		t.Errorf("the Kelvin sign is shown as %v, want its escape", err)
	}
	long := strings.Repeat("a", 63) + "ſ" + strings.Repeat("b", 1000)
	_, err = decode(`{"free":{"` + long + `":{"` + long + `":1}}}`)
	want := `unknown member "` + strings.Repeat("a", 63) + `"... in "/free/` + strings.Repeat("a", 58) + `"...` // "/free/" is 6 of the 64 bytes; the name stops before ſ, which byte 64 is inside
	if err == nil || err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

// A diamond (A1 and A2 both embed B, B embeds C) keeps C's field, as
// encoding/json does: B's own fields cancel, the types it embeds do not.
func TestADiamondKeepsTheSharedFieldAsEncodingJSONDoes(t *testing.T) {
	type C struct{ Z string }
	type B struct {
		C
		Y string
	}
	type A1 struct{ B }
	type A2 struct{ B }
	var v struct {
		A1
		A2
	}
	if err := Decode([]byte(`{"Z":"z"}`), &v); err != nil || v.A1.Z != "z" {
		t.Errorf("Z: %v, decoded %+v", err, v)
	}
	var unknown *UnknownMemberError
	if err := Decode([]byte(`{"Y":"y"}`), &v); !errors.As(err, &unknown) {
		t.Errorf("Y, B's own field reached twice at one depth, got %v, want unknown", err)
	}
	if json.Unmarshal([]byte(`{"Z":"z"}`), &v) != nil || v.A1.Z != "z" {
		t.Error("encoding/json no longer keeps Z: this test's premise moved")
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
	// Cut short at a token boundary is not an empty body: a caller reads
	// io.EOF as "no body sent" (a route whose body is optional).
	for _, body := range []string{`{"subject":`, `{"subject":"x"`, `{"subject":"x",`, `{"list":[`, `{`, `[`} {
		if _, err := decode(body); err == nil || errors.Is(err, io.EOF) {
			t.Errorf("%q: got %v, want an error other than io.EOF", body, err)
		}
	}
	for _, body := range []string{``, " \n\t"} {
		if _, err := decode(body); err != io.EOF {
			t.Errorf("%q: got %v, want io.EOF", body, err)
		}
	}
	if _, err := decode(" {\"contract_version\":1e400} \n"); err == nil {
		t.Error("a number out of an int's range was accepted")
	}
}

// AllowUnknown skips what it does not know, a case or fold variant included,
// and never folds it into a field; the object stays JSON wherever the skipped
// member stood (first, last, alone, between), and a duplicate is still refused.
func TestAllowUnknownSkipsVariantsAndStillRefusesDuplicates(t *testing.T) {
	lenient := DecodeOptions{AllowUnknown: true}
	for _, tc := range []struct{ why, body, subject, name string }{
		{"a case variant, first", `{"Subject":"wwe-the-rock","subject":"wwe-edge"}`, "wwe-edge", ""},
		{"a case variant, last", `{"subject":"wwe-edge" , "Subject":"wwe-the-rock"}`, "wwe-edge", ""},
		{"a case variant, alone", `{ "Subject" : "wwe-the-rock" }`, "", ""},
		{"a fold variant", `{"ſubject":"wwe-the-rock"}`, "", ""},
		{"two skipped, then kept", `{"x-vendor":{"a":[1,2]},"Subject":"wwe-the-rock",
			"subject":"wwe-edge"}`, "wwe-edge", ""},
		{"kept, skipped, kept", `{"subject":"wwe-edge","x":1,"nested":{"Name":"x","name":"n"}}`, "wwe-edge", "n"},
		{"a variant in an array element", `{"list":[{"NAME":"b","name":"a"},{"name":"c","Name":"d"}]}`, "", ""},
	} {
		var r request
		if err := lenient.Decode([]byte(tc.body), &r); err != nil {
			t.Errorf("%s: %v", tc.why, err)
			continue
		}
		if r.Subject != tc.subject || (r.Nested != nil) != (tc.name != "") || r.Nested != nil && r.Nested.Name != tc.name {
			t.Errorf("%s: decoded %+v", tc.why, r)
		}
	}
	var r request
	if err := lenient.Decode([]byte(`{"list":[{"NAME":"b","name":"a"},{"name":"c","Name":"d"}]}`), &r); err != nil ||
		len(r.List) != 2 || r.List[0].Name != "a" || r.List[1].Name != "c" {
		t.Errorf("array: %v, %+v", err, r.List)
	}
	for _, body := range []string{
		`{"subject":"wwe-edge","subject":"wwe-the-rock"}`,
		`{"x":1,"x":2}`,
		`{"x":{"a":1,"a":2}}`,
	} {
		var dup *DuplicateMemberError
		if err := lenient.Decode([]byte(body), &r); !errors.As(err, &dup) {
			t.Errorf("%s: got %v, want a duplicate", body, err)
		}
	}
	if err := lenient.Decode([]byte(`{"x":1}{}`), &r); err == nil {
		t.Error("trailing input was accepted")
	}
	body := []byte(`{"Subject":"wwe-the-rock"}`)
	_ = lenient.Decode(body, &r)
	if string(body) != `{"Subject":"wwe-the-rock"}` {
		t.Errorf("the caller's bytes were changed: %s", body)
	}
}

// No deeper than encoding/json goes (10000 levels), and the walk itself
// stops there: it recurses per level, before encoding/json decodes, on
// json.Decoder.Token's own limit (Go 1.27's v2-backed decoder; Go 1.25's
// Token kept none, and GOEXPERIMENT=nojsonv2 still keeps none: this fails).
func TestNestingIsBoundedAsInEncodingJSON(t *testing.T) {
	deep := []string{strings.Repeat("[", 10001) + strings.Repeat("]", 10001),
		strings.Repeat(`{"a":`, 10001) + "1" + strings.Repeat("}", 10001)}
	for _, body := range deep {
		w := walker{dec: json.NewDecoder(strings.NewReader(body))}
		if err := w.walk(nil); err == nil {
			t.Errorf("%.8s x10001: the walk went all the way down", body)
		}
		for _, opts := range []DecodeOptions{{}, {AllowUnknown: true}} {
			var v any
			if err := opts.Decode([]byte(body), &v); err == nil {
				t.Errorf("%+v, %.8s x10001: accepted", opts, body)
			}
		}
	}
	var v any
	if err := Decode([]byte(strings.Repeat("[", 10000)+strings.Repeat("]", 10000)), &v); err != nil {
		t.Errorf("10000 levels: %v", err)
	}
}
