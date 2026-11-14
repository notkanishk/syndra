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
// sent (so before a key is even derived): a settings write, the release of a
// subject the kit's prefix names, or a read of the read routes. Deleting
// stays with a person on the NAS, so account.purge and every other write
// operation is refused by the same rule that allows nothing else: an
// allow-list, never a deny-list.
func checkCall(method, path string, body map[string]any) error {
	switch {
	case method == http.MethodGet && body == nil &&
		(path == "/health" || path == "/capabilities" || path == "/subjects" || path == "/settings" || settingPath.MatchString(path)):
		return nil
	case method == http.MethodPut && settingPath.MatchString(path):
		return nil
	case method == http.MethodPost && path == "/operations/account.release":
		subject, _ := body["subject"].(string)
		if !strings.HasPrefix(subject, subjectPrefix) || len(subject) == len(subjectPrefix) {
			return fmt.Errorf("account.release is refused for subject %q: the subject must begin %q (the kit's own, never a member's)", subject, subjectPrefix)
		}
		if op, set := body["operation"]; set && op != "account.release" {
			return fmt.Errorf("the body names operation %v, not account.release", op)
		}
		return nil
	}
	return fmt.Errorf("%s %s is refused: this command sends only PUT or GET /settings/<name>, POST /operations/account.release, "+
		"and GET /health, /capabilities, /subjects, /settings; deleting stays with a person on the NAS", method, path)
}

// Call sends exactly one signed request over the kit's own transport (pinned
// TLS, derived keys, the HMAC) and returns the status and body as answered.
// body is a JSON object or empty; contract_version and a fresh call_id are
// added to a write as the kit does, and so are the actor and, for the one
// operation, its name and an empty params.
func Call(ctx context.Context, cfg CallConfig, method, path string, body []byte) (int, []byte, error) {
	var doc map[string]any
	if len(bytes.TrimSpace(body)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil || doc == nil {
			return 0, nil, errors.New("the body must be one JSON object")
		}
	}
	if err := checkCall(method, path, doc); err != nil {
		return 0, nil, err
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return 0, nil, fmt.Errorf("-url must be https://host:port, got %q", cfg.URL)
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
		if doc == nil {
			doc = map[string]any{}
		}
		doc["contract_version"] = Major
		if _, set := doc["call_id"]; !set {
			var b [8]byte
			_, _ = rand.Read(b[:])
			doc["call_id"] = fmt.Sprintf("%s-%d-%s", callActor, time.Now().Unix(), hex.EncodeToString(b[:]))
		}
		if _, set := doc["actor"]; !set {
			doc["actor"] = callActor
		}
		if method == http.MethodPost {
			doc["operation"] = "account.release"
			if _, set := doc["params"]; !set {
				doc["params"] = map[string]any{}
			}
		}
		if payload, err = json.Marshal(doc); err != nil {
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
