package claimtel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// The payload fingerprint is the set of JSON key paths in a v2 body, array
// indexes collapsed (user_grants[].project_id), sorted and hashed. A new one
// means Zitadel's payload changed shape: an upgrade renamed or retyped a field
// (assessment Z12, P1-P4), and the first body of the new shape is kept,
// redacted, as the fixture for a contract test.
//
// Depth is chosen so the fingerprint follows Zitadel's version, not the
// person: full depth where Syndra's contract lives (user_grants, application),
// one level for user (user.id), and top-level names only elsewhere. userinfo
// and user_metadata vary with the app's scopes and the person's data, and
// hashing their insides would raise a "changed shape" for every new user.

// fullDepth are the top-level keys walked to every leaf.
var fullDepth = map[string]bool{"user_grants": true, "application": true}

// oneLevel are the top-level keys whose immediate children count.
var oneLevel = map[string]bool{"user": true}

// Paths is the fingerprint's key paths, sorted. Nil when body is not a JSON object.
func Paths(body []byte) []string {
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil || top == nil {
		return nil
	}
	set := map[string]bool{}
	for k, raw := range top {
		set[k] = true
		var v any
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		switch {
		case fullDepth[k]:
			walk(k, v, set, -1)
		case oneLevel[k]:
			walk(k, v, set, 1)
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// walk adds every key path under prefix, depth levels deep (-1: no limit).
func walk(prefix string, v any, set map[string]bool, depth int) {
	if depth == 0 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			p := prefix + "." + k
			set[p] = true
			walk(p, child, set, depth-1)
		}
	case []any:
		for _, child := range t {
			walk(prefix+"[]", child, set, depth)
		}
	}
}

// Fingerprint hashes a sorted path list.
func Fingerprint(paths []string) string {
	sum := sha256.Sum256([]byte(strings.Join(paths, "\n")))
	return hex.EncodeToString(sum[:8])
}

// keepValue are the paths whose string values survive redaction: the contract
// fields a fixture needs to reproduce an answer. Nothing here names a person.
var keepValue = map[string]bool{
	"function":                   true,
	"application.client_id":      true,
	"user_grants[].project_id":   true,
	"user_grants[].projectId":    true,
	"user_grants[].project_name": true,
	"user_grants[].roles[]":      true,
	"user_grants[].state":        true,
}

// Redacted is body with every string value replaced by "redacted" except the
// contract fields above; keys, numbers and booleans are kept so the example
// still decodes as the payload it was. Nil for a body that is not JSON.
func Redacted(body []byte) json.RawMessage {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return nil
	}
	out, err := json.Marshal(redact("", v))
	if err != nil {
		return nil
	}
	return out
}

func redact(path string, v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			t[k] = redact(p, child)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = redact(path+"[]", child)
		}
		return t
	case string:
		if keepValue[path] {
			return t
		}
		return "redacted"
	}
	return v
}
