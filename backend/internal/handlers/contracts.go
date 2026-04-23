package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func decodeJSONStrict(body io.Reader, dst interface{}) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}

	// Ensure there is no trailing token after the first JSON value.
	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("multiple JSON values in body")
	}

	return nil
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
