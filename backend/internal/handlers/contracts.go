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

func trimmedNonEmpty(v string) bool {
	return strings.TrimSpace(v) != ""
}
