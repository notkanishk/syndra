package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/db"
	"syndra/internal/services"
)

// welcomeSwitch is the stubbed setting and what the change history recorded.
type welcomeSwitch struct {
	value   string
	set     bool
	audited []string
}

// stubWelcomeSwitch keeps the switch in memory, and WelcomeClaims follows it.
func stubWelcomeSwitch(t *testing.T) *welcomeSwitch {
	t.Helper()
	gm, sc, au, wc := dbGetConfigSettingMeta, dbSetConfigSetting, dbInsertAuditLog, svcWelcomeClaims
	t.Cleanup(func() {
		dbGetConfigSettingMeta, dbSetConfigSetting, dbInsertAuditLog, svcWelcomeClaims = gm, sc, au, wc
	})
	s := &welcomeSwitch{}
	dbGetConfigSettingMeta = func(_ context.Context, key string) (db.ConfigSetting, bool, error) {
		if key != services.ConfigKeyWelcomeClaims || !s.set {
			return db.ConfigSetting{}, false, nil
		}
		return db.ConfigSetting{Value: s.value, UpdatedBy: "op1", UpdatedAt: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}, true, nil
	}
	dbSetConfigSetting = func(_ context.Context, key, value, _ string) error {
		if key == services.ConfigKeyWelcomeClaims {
			s.value, s.set = value, true
		}
		return nil
	}
	dbInsertAuditLog = func(_ context.Context, _, _, action, _ string) error {
		s.audited = append(s.audited, action)
		return nil
	}
	svcWelcomeClaims = func(context.Context) (claimserve.Welcome, error) {
		if s.value != "true" {
			return claimserve.Welcome{}, nil
		}
		return claimserve.Welcome{On: true}, nil
	}
	return s
}

func putWelcome(body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handlePutWelcomeClaims(rr, httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body)))
	return rr
}

func TestWelcomeClaimsSwitch_SavedAuditedOnChangeAndPublished(t *testing.T) {
	r := claimservetest.New()
	cr := claimConfigRedis
	t.Cleanup(func() { claimConfigRedis = cr })
	claimConfigRedis = func() claimserve.Redis { return r }
	s := stubWelcomeSwitch(t)

	// Off until someone turns it on.
	rr := httptest.NewRecorder()
	handleGetWelcomeClaims(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	var view welcomeClaimsView
	_ = json.Unmarshal(rr.Body.Bytes(), &view)
	if rr.Code != http.StatusOK || view.On || view.SetBy != nil {
		t.Fatalf("never set: %d %s", rr.Code, rr.Body)
	}

	if rr := putWelcome(`{"on":true}`); rr.Code != http.StatusOK {
		t.Fatalf("on: %d %s", rr.Code, rr.Body)
	}
	if v, _ := r.Value(claimserve.WelcomeKey); v != `{"on":true}` || r.Count("INCR") != 1 {
		t.Fatalf("not written through: %q, gen moved %d", v, r.Count("INCR"))
	}
	putWelcome(`{"on":true}`) // the same again: nothing new to record
	putWelcome(`{"on":false}`)
	if strings.Join(s.audited, ",") != "welcome_claims.turned_on,welcome_claims.turned_off" {
		t.Fatalf("audited %v", s.audited)
	}
	if v, _ := r.Value(claimserve.WelcomeKey); v != `{"on":false}` {
		t.Fatalf("off not published: %q", v)
	}

	rr = httptest.NewRecorder()
	handleGetWelcomeClaims(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	_ = json.Unmarshal(rr.Body.Bytes(), &view)
	if view.On || view.SetBy == nil || *view.SetBy != "op1" {
		t.Fatalf("read back: %s", rr.Body)
	}
}

func TestWelcomeClaimsSwitch_RefusesWhatIsNotAnAnswer(t *testing.T) {
	s := stubWelcomeSwitch(t)
	for _, body := range []string{`{}`, `{"on":"yes"}`, `{"on":true,"extra":1}`, `not json`} {
		if rr := putWelcome(body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, rr.Code)
		}
	}
	if s.set || len(s.audited) != 0 {
		t.Fatal("a refused request changed the setting")
	}
}
