package conformance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CallConfig names one add-on and the secret its keys derive from.
type CallConfig struct {
	URL, Target string
	Secret      []byte
}

// callActor labels the request in the add-on's log, as the kit's own is.
const callActor = "syndra-addon-call"

var (
	settingPath = regexp.MustCompile(`^/settings/[A-Za-z0-9_-]+$`)
	// adoptUsername is the kit's own account name: conformance- and at least
	// one of [a-z0-9-], nothing around it (the add-on trims, so a trailing
	// space would name another account than the one checked here).
	adoptUsername = regexp.MustCompile(`^conformance-[a-z0-9-]+$`)
	accountID     = regexp.MustCompile(`^[0-9]+$`)
)

// adoptSubject is the one subject an adoption may name: the staging test
// subject the owner's 6.10 checks link a test home to.
const adoptSubject = "rey_student"

// maxCallBody caps what the tool reads of an operator's body.
const maxCallBody = 64 << 10

// checkCall is the whole of what one call may be, decided before anything is
// sent (so before a key is even derived): a settings write of one value, the
// release of one subject that is not a person's, the adoption of one account
// the kit's prefix names for the one test subject, or a read of the read routes.
// Deleting stays with a person on the NAS, so account.purge and every other
// write operation is refused by the same rule that allows nothing else: an
// allow-list, never a deny-list.
//
// Subjects are an allow-list too, never a guess at what a person's looks
// like (Zitadel accepts custom user ids, so a member's need not be digits):
// adopt names exactly rey_student; release names exactly rey_student or a
// subject beginning conformance- (the kit's own, released in step 4b).
// Compared byte for byte, after refusing invalid UTF-8, surrounding
// whitespace and control characters.
//
// Nothing the operator typed is passed through. A body is parsed token by
// token and may hold exactly the keys allowed, each byte-equal and once (Go's
// decoder matches keys case-insensitively with Unicode folding, so a "ſubject"
// beside "subject" would be read by the add-on as the same field and win); the
// payload is then BUILT from those typed values. The returned fields are those,
// and Call adds the ones the tool sets itself.
func checkCall(method, path string, body []byte) (map[string]any, error) {
	// One byte past the cap is read, so an over-long body is seen, not cut.
	if body, _ = io.ReadAll(io.LimitReader(bytes.NewReader(body), maxCallBody+1)); len(body) > maxCallBody {
		return nil, fmt.Errorf("the body is refused: it is over %d bytes", maxCallBody)
	}
	switch {
	case method == http.MethodGet && len(bytes.TrimSpace(body)) == 0 &&
		(path == "/health" || path == "/capabilities" || path == "/subjects" || path == "/settings"):
		return nil, nil
	case method == http.MethodPut && settingPath.MatchString(path):
		f, err := strictStrings(body, "value")
		return map[string]any{"value": f["value"]}, err
	case method == http.MethodPost && path == "/operations/account.release":
		f, err := strictStrings(body, "subject")
		if err != nil {
			return nil, err
		}
		if err := testSubject("account.release", f["subject"], true); err != nil {
			return nil, err
		}
		return map[string]any{"subject": f["subject"], "operation": "account.release", "params": map[string]any{}}, nil
	case method == http.MethodPost && path == "/operations/account.adopt":
		top, err := strictObject(body, "subject", "params")
		if err != nil {
			return nil, err
		}
		subject, ok := asString(top["subject"])
		if !ok {
			return nil, errors.New(`account.adopt is refused: "subject" must be a string`)
		}
		if err := testSubject("account.adopt", subject, false); err != nil {
			return nil, err
		}
		p, err := strictStrings(top["params"], "username", "account_id")
		if err != nil {
			return nil, fmt.Errorf("account.adopt is refused: params: %w", err)
		}
		if !adoptUsername.MatchString(p["username"]) {
			return nil, fmt.Errorf("account.adopt is refused for username %q: it must be %q then a-z, 0-9 or -, nothing around it (the kit's own account, never a member's)", p["username"], subjectPrefix)
		}
		if !accountID.MatchString(p["account_id"]) {
			return nil, fmt.Errorf("account.adopt is refused for account_id %q: it must be digits only (the precondition the add-on checks)", p["account_id"])
		}
		return map[string]any{"subject": subject, "operation": "account.adopt",
			"params": map[string]any{"username": p["username"], "account_id": p["account_id"]}}, nil
	}
	return nil, fmt.Errorf("%s %s is refused: this command sends only PUT /settings/<name>, POST /operations/account.release "+
		"and /operations/account.adopt, and GET /health, /capabilities, /subjects, /settings; deleting stays with a person on the NAS", method, path)
}

// testSubject allows adoptSubject and, for a release (kit), a subject the
// kit's prefix names; nothing else, so never a member's.
func testSubject(op, subject string, kit bool) error {
	ok := utf8.ValidString(subject) && strings.TrimSpace(subject) == subject &&
		!strings.ContainsFunc(subject, unicode.IsControl) &&
		(subject == adoptSubject || kit && strings.HasPrefix(subject, subjectPrefix) && len(subject) > len(subjectPrefix))
	if !ok {
		allowed := fmt.Sprintf("%q", adoptSubject)
		if kit {
			allowed += fmt.Sprintf(" or one beginning %q", subjectPrefix)
		}
		return fmt.Errorf("%s is refused for subject %q: only %s may be named, never a member", op, subject, allowed)
	}
	return nil
}

// strictObject reads body as one JSON object holding every one of keys, each
// byte-equal and once, no other key, and nothing after the object. The values
// come back raw, for the caller to read by type.
func strictObject(body []byte, keys ...string) (map[string]json.RawMessage, error) {
	bad := fmt.Errorf("the body must be exactly one object with %q, each once, and nothing else", keys)
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, bad
	}
	got := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		k, _ := tok.(string)
		if _, dup := got[k]; err != nil || dup || !slices.Contains(keys, k) {
			return nil, bad
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, bad
		}
		got[k] = v
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return nil, bad
	}
	if _, err := dec.Token(); err != io.EOF { // trailing data
		return nil, bad
	}
	if len(got) != len(keys) {
		return nil, bad
	}
	return got, nil
}

// strictStrings is strictObject whose every value is a string (never null).
func strictStrings(body []byte, keys ...string) (map[string]string, error) {
	raw, err := strictObject(body, keys...)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, v := range raw {
		s, ok := asString(v)
		if !ok {
			return nil, fmt.Errorf("the body must be exactly one object with %q, each a string, and nothing else", keys)
		}
		out[k] = s
	}
	return out, nil
}

// asString reads one raw JSON value that must be a string (null is not).
func asString(raw json.RawMessage) (string, bool) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// checkURL: the base URL is an origin and nothing else, so the path sent is
// the one checkCall judged.
func checkURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("-url must be https://host:port, got %q", raw)
	}
	if (u.Path != "" && u.Path != "/") || strings.ContainsAny(raw, "?#") || u.User != nil {
		return nil, fmt.Errorf("-url must be https://host:port only: no path, query, fragment or credentials, got %q", raw)
	}
	return u, nil
}

// Call sends exactly one signed request over the kit's own transport (pinned
// TLS, derived keys, the HMAC) and returns the status and body as answered.
// contract_version, a fresh call_id and the actor are the tool's own, never
// the operator's.
func Call(ctx context.Context, cfg CallConfig, method, path string, body []byte) (int, []byte, error) {
	fields, err := checkCall(method, path, body)
	if err != nil {
		return 0, nil, err
	}
	u, err := checkURL(cfg.URL)
	if err != nil {
		return 0, nil, err
	}
	if strings.TrimSpace(cfg.Target) == "" || len(bytes.TrimSpace(cfg.Secret)) == 0 {
		return 0, nil, errors.New("-target and a non-empty -secret-file are required")
	}
	pub, key, err := deriveKeys(bytes.TrimSpace(cfg.Secret), cfg.Target)
	if err != nil {
		return 0, nil, err
	}
	var payload []byte
	if method != http.MethodGet {
		var b [8]byte
		_, _ = rand.Read(b[:])
		fields["contract_version"] = Major
		fields["call_id"] = fmt.Sprintf("%s-%d-%s", callActor, time.Now().Unix(), hex.EncodeToString(b[:]))
		fields["actor"] = callActor
		if payload, err = json.Marshal(fields); err != nil {
			return 0, nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(u.String(), "/")+path, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	now := time.Now().Unix()
	req.Header.Set(signatureHeader, fmt.Sprintf("t=%d,v1=%s", now, mac(now, method, signedTarget(req.URL), payload, key)))
	resp, err := pinnedClient(pub).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	reply, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, reply, err
}
