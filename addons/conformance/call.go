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
	"strings"
	"time"
)

// CallConfig names one add-on and the secret its keys derive from.
type CallConfig struct {
	URL, Target string
	Secret      []byte
}

// callActor labels the request in the add-on's log, as the kit's own is.
const callActor = "syndra-addon-call"

var settingPath = regexp.MustCompile(`^/settings/[A-Za-z0-9_-]+$`)

// checkCall is the whole of what one call may be, decided before anything is
// sent (so before a key is even derived): a settings write of one value, the
// release of one subject the kit's prefix names, or a read of the read routes.
// Deleting stays with a person on the NAS, so account.purge and every other
// write operation is refused by the same rule that allows nothing else: an
// allow-list, never a deny-list.
//
// Nothing the operator typed is passed through. A body is parsed token by
// token and may hold exactly one key, byte-equal to the one allowed (Go's
// decoder matches keys case-insensitively with Unicode folding, so a "ſubject"
// beside "subject" would be read by the add-on as the same field and win); the
// payload is then BUILT from that typed value. The returned fields are those,
// and Call adds the ones the tool sets itself.
func checkCall(method, path string, body []byte) (map[string]any, error) {
	switch {
	case method == http.MethodGet && len(bytes.TrimSpace(body)) == 0 &&
		(path == "/health" || path == "/capabilities" || path == "/subjects" || path == "/settings"):
		return nil, nil
	case method == http.MethodPut && settingPath.MatchString(path):
		v, err := onlyString(body, "value")
		return map[string]any{"value": v}, err
	case method == http.MethodPost && path == "/operations/account.release":
		subject, err := onlyString(body, "subject")
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(subject, subjectPrefix) || len(subject) == len(subjectPrefix) {
			return nil, fmt.Errorf("account.release is refused for subject %q: the subject must begin %q (the kit's own, never a member's)", subject, subjectPrefix)
		}
		return map[string]any{"subject": subject, "operation": "account.release", "params": map[string]any{}}, nil
	}
	return nil, fmt.Errorf("%s %s is refused: this command sends only PUT /settings/<name>, POST /operations/account.release, "+
		"and GET /health, /capabilities, /subjects, /settings; deleting stays with a person on the NAS", method, path)
}

// onlyString reads body as one JSON object whose only key is exactly key, once,
// with a string value, and nothing after the object.
func onlyString(body []byte, key string) (string, error) {
	bad := fmt.Errorf("the body must be exactly {%q: <string>} and nothing else", key)
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return "", bad
	}
	var got string
	seen := false
	for dec.More() {
		tok, err := dec.Token()
		if k, _ := tok.(string); err != nil || k != key || seen {
			return "", bad
		}
		var v any
		if err := dec.Decode(&v); err != nil {
			return "", bad
		}
		if got, seen = v.(string); !seen {
			return "", bad
		}
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return "", bad
	}
	if _, err := dec.Token(); err != io.EOF { // trailing data
		return "", bad
	}
	if !seen {
		return "", bad
	}
	return got, nil
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
