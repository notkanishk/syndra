package zitadel

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// W1d (one-path): the Management client retries through extcall. Reads are
// retried; a write is repeated only after a 429; any other failure of a write
// is ambiguous and is settled by a read-back where one exists.

func fastPolicy(t *testing.T) {
	t.Helper()
	orig := zitadelPolicy
	zitadelPolicy.Base, zitadelPolicy.Max = time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() { zitadelPolicy = orig })
}

var lostAnswer = &url.Error{Op: "Post", URL: "https://zitadel.example.org", Err: &net.DNSError{IsTimeout: true}}

// The orphan-role case: Zitadel created the role but the answer was a 503.
// Before, the client resent the POST and the caller saw a 409 or a failure and
// rolled back its own row. Now the POST is sent once and the read-back finds
// the role, so the caller sees success.
func TestAddProjectRole_AmbiguousAnswerIsSettledByReadBack(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	posts := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/management/v1/projects/p1/roles":
			posts++
			return jsonResp(503, map[string]string{"message": "unavailable"}), nil
		case "/management/v1/projects/p1/roles/_search":
			return jsonResp(200, map[string]any{
				"details": map[string]string{"totalResult": "1"},
				"result":  []ProjectRoleResult{{Key: "laser"}},
			}), nil
		}
		t.Fatalf("unexpected call %s %s", req.Method, req.URL.Path)
		return nil, nil
	}

	if err := client.AddProjectRole(context.Background(), "p1", "laser", "Laser", ""); err != nil {
		t.Fatalf("a role the read-back found must not be reported as failed: %v", err)
	}
	if posts != 1 {
		t.Fatalf("the create was sent %d times; a write whose outcome is unknown is never resent", posts)
	}
}

func TestAddProjectRole_ReadBackThatDoesNotFindItKeepsTheFailure(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	posts := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/_search") {
			return jsonResp(200, map[string]any{"result": []ProjectRoleResult{{Key: "other"}}}), nil
		}
		posts++
		return jsonResp(503, map[string]string{"message": "unavailable"}), nil
	}

	err := client.AddProjectRole(context.Background(), "p1", "laser", "Laser", "")
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 503 {
		t.Fatalf("want the 503 back for the caller to classify, got %v", err)
	}
	if posts != 1 {
		t.Fatalf("the create was sent %d times", posts)
	}
}

func TestAddUserGrant_LostAnswerIsSettledByReadBack(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	posts := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/management/v1/users/grants/_search" {
			return jsonResp(200, map[string]any{
				"details": map[string]string{"totalResult": "1"},
				"result":  []UserGrant{{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"a", "b"}}},
			}), nil
		}
		posts++
		return nil, lostAnswer
	}

	if err := client.AddUserGrant(context.Background(), "u1", "p1", []string{"a", "b"}); err != nil {
		t.Fatalf("a grant the read-back found must not be reported as failed: %v", err)
	}
	if posts != 1 {
		t.Fatalf("the grant was sent %d times", posts)
	}
}

// A write with no read-back is sent once on an ambiguous failure, and the
// status survives for the outbox, which owns that retry.
func TestWritesWithoutReadBackAreNotResentOn503(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	calls := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		return jsonResp(503, map[string]string{"message": "unavailable"}), nil
	}

	for name, write := range map[string]func() error{
		"update grant": func() error { return client.UpdateUserGrant(context.Background(), "u1", "g1", []string{"a"}) },
		"remove grant": func() error { return client.RemoveUserGrant(context.Background(), "u1", "g1") },
		"delete role":  func() error { return client.DeleteProjectRole(context.Background(), "p1", "laser") },
	} {
		calls = 0
		err := write()
		var se *StatusError
		if !errors.As(err, &se) || se.Code != 503 {
			t.Errorf("%s: want a 503 StatusError, got %v", name, err)
		}
		if calls != 1 {
			t.Errorf("%s: sent %d times", name, calls)
		}
	}
}

// Reads are retried, including on a lost answer, which used to fail at once.
func TestReadsRetryOn503AndLostAnswers(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	calls := 0
	var ids []string
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		calls++
		ids = append(ids, req.Header.Get("X-Request-Id"))
		switch calls {
		case 1:
			return jsonResp(503, map[string]string{"message": "unavailable"}), nil
		case 2:
			return nil, lostAnswer
		}
		return jsonResp(200, map[string]any{"result": []UserGrant{}}), nil
	}

	if _, err := client.ListUserGrants(context.Background(), "u1", SearchParams{}); err != nil {
		t.Fatalf("a read must be retried through a blip: %v", err)
	}
	if calls != 3 {
		t.Fatalf("tries = %d, want 3", calls)
	}
	if ids[0] == "" || ids[0] != ids[2] {
		t.Fatalf("every try of one call carries the same request id, got %q", ids)
	}
}

// A 401 after a 429 used to fail without refreshing: the refresh was allowed
// on the first try only.
func TestA401AfterA429StillRefreshesTheToken(t *testing.T) {
	resetDeps(t)
	fastPolicy(t)
	client, _ := testClient(t)

	mgmt := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/oauth/v2/token") {
			return jsonResp(200, tokenResponse{AccessToken: "fresh-token", ExpiresIn: 3600}), nil
		}
		mgmt++
		switch mgmt {
		case 1:
			return jsonResp(429, map[string]string{"message": "slow down"}), nil
		case 2:
			return jsonResp(401, map[string]string{"message": "expired"}), nil
		}
		if got := req.Header.Get("Authorization"); got != "Bearer fresh-token" {
			t.Errorf("the resend must carry the refreshed token, got %q", got)
		}
		return jsonResp(200, map[string]string{}), nil
	}

	if err := client.AddUserGrant(context.Background(), "u1", "p1", []string{"a"}); err != nil {
		t.Fatalf("429 then 401 must refresh and succeed: %v", err)
	}
	if mgmt != 3 {
		t.Fatalf("management calls = %d, want 3", mgmt)
	}
}

func TestIdempotentIsReadsOnly(t *testing.T) {
	for _, c := range []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/management/v1/projects/p1/apps/a1", true},
		{http.MethodPost, "/management/v1/users/grants/_search", true},
		{http.MethodPost, "/v2/actions/targets/search", true},
		{http.MethodPost, "/management/v1/users/u1/grants", false},
		{http.MethodPost, "/v2/actions/targets/t1", false},
		{http.MethodPut, "/management/v1/users/u1/grants/g1", false},
		{http.MethodDelete, "/management/v1/users/u1/grants/g1", false},
	} {
		if got := idempotent(c.method, c.path); got != c.want {
			t.Errorf("idempotent(%s %s) = %v, want %v", c.method, c.path, got, c.want)
		}
	}
}
