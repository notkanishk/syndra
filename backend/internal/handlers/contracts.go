package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"syndra/addons/strictjson"
)

// decodeJSONStrict decodes one JSON value from body into dst by the one rule
// for every JSON input to Syndra: member names exactly as dst spells them
// (RFC 8259 §4), never twice in one object (RFC 7493 §2.3), nothing unknown
// and nothing after the value. strictjson is the decoder the add-ons use
// too. An empty body is io.EOF, as before, for the routes whose body is
// optional; every error is the caller's 400.
func decodeJSONStrict(body io.Reader, dst interface{}) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	return strictjson.Decode(data, dst)
}

// decodeJSONLenient accepts JSON with unknown fields. Reserve for endpoints
// whose payload shape is owned by an external system that may extend it
// (e.g. Zitadel Actions v2 function-trigger bodies). Trailing-token guard
// is preserved — the laxity is only on field set, not on body hygiene.
func decodeJSONLenient(body io.Reader, dst interface{}) error {
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil {
		return err
	}

	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple JSON values in body")
	}

	return nil
}

func trimmedNonEmpty(v string) bool {
	return strings.TrimSpace(v) != ""
}
