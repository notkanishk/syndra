package handlers

import (
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

func trimmedNonEmpty(v string) bool {
	return strings.TrimSpace(v) != ""
}
