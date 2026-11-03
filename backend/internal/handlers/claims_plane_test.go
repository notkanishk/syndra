package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// publishStore points the write side at an in-memory Redis and records every
// project it resolved. Each resolve returns a new default source, so every
// publish is a change the snapshot must take.
func publishStore(t *testing.T) (*claimservetest.Redis, *[]string) {
	t.Helper()
	rs, cm, rd := dbResolveProjectShapes, dbGetClaimMode, claimConfigRedis
	t.Cleanup(func() { dbResolveProjectShapes, dbGetClaimMode, claimConfigRedis = rs, cm, rd })
	r := claimservetest.New()
	claimConfigRedis = func() claimserve.Redis { return r }
	var mu sync.Mutex
	var published []string
	dbResolveProjectShapes = func(_ context.Context, p string) (services.ProjectShapes, error) {
		mu.Lock()
		defer mu.Unlock()
		published = append(published, p)
		return services.ProjectShapes{Default: claims.Builtin(), DefaultSource: fmt.Sprintf("v%d", len(published))}, nil
	}
	dbGetClaimMode = func(context.Context, string) (db.ClaimMode, error) { return db.ClaimMode{Mode: "fail_closed"}, nil }
	return r, &published
}

func publishRecorder(t *testing.T) *[]string {
	t.Helper()
	_, p := publishStore(t)
	return p
}

// ADR-004: "A shape or mode save must write through and bump claims:gen. A
// save that skips this is a bug." Every path that writes claim config moves
// the generation once, and the data plane's next poll answers from it.
func TestEverySavePathMovesTheGeneration(t *testing.T) {
	r, _ := publishStore(t)
	plane := claimserve.New(r, nil)
	t.Setenv("ZITADEL_DOMAIN", "")
	origs := []any{svcSaveProjectClaimProfile, svcGetProjectClaimShape, svcSaveAppClaimShape, svcRestoreAppClaimShape,
		svcGetAppClaimShape, svcSeedClaimShapes, dbClaimConfigProjects, dirProjectIDs, dbInsertAuditLog}
	t.Cleanup(func() {
		svcSaveProjectClaimProfile = origs[0].(func(context.Context, string, services.ProjectDefaultForm) error)
		svcGetProjectClaimShape = origs[1].(func(context.Context, string) (services.ProjectClaimShape, error))
		svcSaveAppClaimShape = origs[2].(func(context.Context, string, string, services.SaveClaimShapeRequest, int) (services.SaveResult, error))
		svcRestoreAppClaimShape = origs[3].(func(context.Context, string, string, int, bool) (services.SaveResult, error))
		svcGetAppClaimShape = origs[4].(func(context.Context, string) (services.AppClaimShape, bool, error))
		svcSeedClaimShapes = origs[5].(func(context.Context) ([]string, error))
		dbClaimConfigProjects = origs[6].(func(context.Context) ([]string, error))
		dirProjectIDs = origs[7].(func(context.Context) ([]string, error))
		dbInsertAuditLog = origs[8].(func(context.Context, string, string, string, string) error)
	})
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
	saved := services.SaveResult{Version: db.ClaimShapeVersion{ApplicationID: "aNB", ProjectID: "pNet"}}
	svcSaveProjectClaimProfile = func(context.Context, string, services.ProjectDefaultForm) error { return nil }
	svcGetProjectClaimShape = func(context.Context, string) (services.ProjectClaimShape, error) {
		return services.ProjectClaimShape{}, nil
	}
	svcSaveAppClaimShape = func(context.Context, string, string, services.SaveClaimShapeRequest, int) (services.SaveResult, error) {
		return saved, nil
	}
	svcRestoreAppClaimShape = func(context.Context, string, string, int, bool) (services.SaveResult, error) { return saved, nil }
	svcGetAppClaimShape = func(context.Context, string) (services.AppClaimShape, bool, error) {
		return services.AppClaimShape{Application: models.ApplicationCatalog{ProjectID: "pNet"}}, true, nil
	}
	svcSeedClaimShapes = func(context.Context) ([]string, error) { return []string{"pNet"}, nil }
	dbClaimConfigProjects = func(context.Context) ([]string, error) { return []string{"pNet"}, nil }
	dirProjectIDs = func(context.Context) ([]string, error) { return nil, nil }

	call := func(method, path string, h http.HandlerFunc, body string, pathValues ...string) int {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		for i := 0; i+1 < len(pathValues); i += 2 {
			req.SetPathValue(pathValues[i], pathValues[i+1])
		}
		h(rr, req)
		return rr.Code
	}
	paths := []struct {
		name string
		run  func() int
	}{
		{"project default saved", func() int {
			return call(http.MethodPut, "/", handleSetProjectClaimProfile, `{}`, "id", "pNet")
		}},
		{"app shape saved", func() int {
			return call(http.MethodPut, "/", handleSaveAppClaimShape, `{"profile":{"claims":[],"lands":"both"}}`, "id", "aNB")
		}},
		{"app shape restored", func() int {
			return call(http.MethodPost, "/", handleRestoreAppClaimShape, ``, "id", "aNB", "n", "2")
		}},
		{"seeded on read", func() int { return call(http.MethodGet, "/", handleGetAppClaimShape, ``, "id", "aNB") }},
		{"seeded hourly", func() int {
			if SeedClaimShapes(context.Background()) != nil {
				return 500
			}
			return 200
		}},
		{"failure mode (or anything) edited in SQL, minute pass", func() int {
			if PublishAllClaimConfig(context.Background()) != nil {
				return 500
			}
			return 200
		}},
	}
	for i, p := range paths {
		if code := p.run(); code != http.StatusOK {
			t.Fatalf("%s: HTTP %d", p.name, code)
		}
		if got := r.Count("INCR"); got != i+1 {
			t.Fatalf("%s: claims:gen moved %d times in all, want %d: this save path skips the snapshot", p.name, got, i+1)
		}
		if err := plane.Poll(context.Background()); err != nil || plane.Config().Gen != int64(i+1) {
			t.Fatalf("%s: the data plane did not reload: %v %+v", p.name, err, plane.Config())
		}
		if _, ok := plane.Config().Projects["pNet"]; !ok {
			t.Fatalf("%s: pNet not in the snapshot", p.name)
		}
	}

	// A refused save publishes nothing.
	if code := call(http.MethodPut, "/", handleSaveAppClaimShape, `{"surprise":1}`, "id", "aNB"); code != http.StatusBadRequest || r.Count("INCR") != len(paths) {
		t.Fatalf("a refused save moved the generation: %d, %d", code, r.Count("INCR"))
	}
}

// The minute pass publishes Syndra's projects and Zitadel's, writes only what
// changed, and never publishes a project it could not resolve.
func TestPublishAllClaimConfig(t *testing.T) {
	r := claimservetest.New()
	rs, cm, rd, cp, dp := dbResolveProjectShapes, dbGetClaimMode, claimConfigRedis, dbClaimConfigProjects, dirProjectIDs
	t.Cleanup(func() {
		dbResolveProjectShapes, dbGetClaimMode, claimConfigRedis, dbClaimConfigProjects, dirProjectIDs = rs, cm, rd, cp, dp
	})
	claimConfigRedis = func() claimserve.Redis { return r }
	dbClaimConfigProjects = func(context.Context) ([]string, error) { return []string{"pA", "pBad"}, nil }
	dirProjectIDs = func(context.Context) ([]string, error) { return []string{"pA", "pZ"}, nil }
	dbResolveProjectShapes = func(_ context.Context, p string) (services.ProjectShapes, error) {
		if p == "pBad" {
			return services.ProjectShapes{}, errors.New("row unreadable")
		}
		return services.ProjectShapes{Default: claims.Builtin()}, nil
	}
	dbGetClaimMode = func(_ context.Context, p string) (db.ClaimMode, error) {
		return db.ClaimMode{Mode: "minimal_safe", MinimalSafeClaims: map[string]any{"k": p}, LKGMaxAgeSeconds: 900}, nil
	}

	if err := PublishAllClaimConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "pBad") {
		t.Fatalf("a project that could not be resolved must be reported: %v", err)
	}
	plane := claimserve.New(r, nil)
	_ = plane.Poll(context.Background())
	var got []string
	for id := range plane.Config().Projects {
		got = append(got, id)
	}
	sort.Strings(got)
	if fmt.Sprint(got) != "[pA pZ]" {
		t.Fatalf("published %v, want pA and pZ (never a guess for pBad)", got)
	}
	if m := plane.Config().Projects["pA"].Mode; m.LKGMaxAge() != 15*time.Minute || m.Claims()[0].Value != "pA" {
		t.Fatalf("mode: %+v", m)
	}
	_ = PublishAllClaimConfig(context.Background())
	if r.Count("INCR") != 1 {
		t.Fatalf("an unchanged pass moved the generation (%d moves)", r.Count("INCR"))
	}

	// The directory down: Syndra's own projects still publish.
	dirProjectIDs = func(context.Context) ([]string, error) { return nil, errors.New("zitadel down") }
	dbClaimConfigProjects = func(context.Context) ([]string, error) { return []string{"pNew"}, nil }
	_ = PublishAllClaimConfig(context.Background())
	_ = plane.Poll(context.Background())
	if _, ok := plane.Config().Projects["pNew"]; !ok {
		t.Fatal("a directory outage stopped Syndra's own projects publishing")
	}
}

func TestClaimsServer_OwnDoor(t *testing.T) {
	t.Setenv("SYNDRA_CLAIMS_ADDR", "")
	if ClaimsAddr() != ":8090" {
		t.Fatalf("default addr %q", ClaimsAddr())
	}
	t.Setenv("SYNDRA_CLAIMS_ADDR", "127.0.0.1:9999")
	if ClaimsAddr() != "127.0.0.1:9999" {
		t.Fatalf("env addr %q", ClaimsAddr())
	}
	s := NewClaimsServer(":8090")
	if s.ReadTimeout != 2*time.Second || s.WriteTimeout != 2*time.Second || s.ReadHeaderTimeout != 2*time.Second || s.MaxHeaderBytes != 16<<10 {
		t.Fatalf("claims server limits: %+v", s)
	}

	// Its mux has the data plane and its health, and nothing else.
	builtinPlane(t)
	devMode(t)
	h := NewClaimsRouter()
	for path, want := range map[string]int{"/healthz": 404, "/api/v1/projects": 404, "/healthz/claims": 0} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if want != 0 && rr.Code != want {
			t.Errorf("GET %s on the claims door: %d, want %d", path, rr.Code, want)
		}
		if want == 0 && rr.Code != 200 && rr.Code != 503 {
			t.Errorf("GET %s on the claims door: %d", path, rr.Code)
		}
	}
	// The body cap: a payload past 1MiB is refused, not read.
	rr := httptest.NewRecorder()
	big := `{"user":{"id":"u1"},"x":"` + strings.Repeat("a", 2<<20) + `"}`
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(big)))
	if rr.Code != http.StatusBadRequest {
		t.Errorf("an oversized body: HTTP %d", rr.Code)
	}
}

// The admin side panicking and slow must not touch the data plane's door:
// with the admin server holding 40 slow requests and answering 40 panics,
// every claims call on :8090 is served within budget.
func TestClaimsDoor_UnaffectedByAdminPanicsAndSlowness(t *testing.T) {
	devMode(t)
	t.Setenv("SYNDRA_API_KEY", "test-key")
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	captureCalls(t)
	orig := svcGetProjectClaimShape
	t.Cleanup(func() { svcGetProjectClaimShape = orig })
	release := make(chan struct{})
	svcGetProjectClaimShape = func(ctx context.Context, _ string) (services.ProjectClaimShape, error) {
		select { // a stuck admin handler: a lock, a slow sweep
		case <-release:
		case <-ctx.Done():
		}
		return services.ProjectClaimShape{}, nil
	}

	admin := httptest.NewServer(NewRouter())
	defer admin.Close()
	cs := httptest.NewUnstartedServer(nil)
	cs.Config = NewClaimsServer("")
	cs.Start()
	defer cs.Close()
	var once sync.Once
	unstick := func() { once.Do(func() { close(release) }) }
	defer unstick()

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(2)
		go func() { // slow
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, admin.URL+"/api/v1/projects/p1/claim-shape", nil)
			req.Header.Set("Authorization", "Bearer test-key")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if resp, err := http.DefaultClient.Do(req.WithContext(ctx)); err == nil {
				resp.Body.Close()
			}
		}()
		go func() { // panics: /healthz pings a database that is not there
			defer wg.Done()
			if resp, err := http.Get(admin.URL + "/healthz"); err == nil {
				resp.Body.Close()
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)

	body := `{"function":"function/preaccesstoken","user":{"id":"u1"},"user_grants":[{"project_id":"p1"}]}`
	var worst time.Duration
	for i := 0; i < 100; i++ {
		start := time.Now()
		resp, err := http.Post(cs.URL+"/api/action/inject", "application/json", strings.NewReader(body))
		took := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		var out ActionV2Response
		_ = decodeJSONLenient(resp.Body, &out)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(out.AppendClaims) != 1 {
			t.Fatalf("call %d under admin load: HTTP %d %v", i, resp.StatusCode, out)
		}
		if took > worst {
			worst = took
		}
	}
	t.Logf("measured: 100 claims calls beside 40 stuck and 40 panicking admin requests, worst %s", worst.Round(time.Microsecond))
	if worst > faultBudget {
		t.Errorf("worst claims call %s under admin load, budget %s", worst, faultBudget)
	}
	unstick()
	wg.Wait()
}
