package handlers

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

func setupEnrichDeps(t *testing.T) {
	t.Helper()
	prevGet := dbGetGrantIndex
	prevList := dbListUserGrantsLive
	t.Cleanup(func() {
		dbGetGrantIndex = prevGet
		dbListUserGrantsLive = prevList
	})
}

func TestEnrichGrantPayload_NonGrantEvent_NoOp(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		t.Fatalf("must not call index for non-grant events")
		return db.ZitadelGrantIndex{}, nil
	}
	in := WebhookPayload{EventType: "user_created", UserID: "u-1"}
	out := enrichGrantPayload(context.Background(), in)
	if !reflect.DeepEqual(out, in) {
		t.Errorf("payload mutated for non-grant event: %+v", out)
	}
}

func TestEnrichGrantPayload_GrantChanged_LocalIndexFillsProject(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, gid string) (db.ZitadelGrantIndex, error) {
		if gid != "g-1" {
			t.Fatalf("got grantID=%q", gid)
		}
		return db.ZitadelGrantIndex{GrantID: "g-1", UserID: "u-1", ProjectID: "p-77", RoleKeys: []string{"old"}}, nil
	}
	in := WebhookPayload{EventType: "grant_changed", UserID: "u-1", GrantID: "g-1", RoleKeys: []string{"new"}}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-77" {
		t.Errorf("SourceProject = %q, want p-77", out.SourceProject)
	}
	if !reflect.DeepEqual(out.RoleKeys, []string{"new"}) {
		t.Errorf("RoleKeys = %v, want [new] (event value must win over index)", out.RoleKeys)
	}
}

func TestEnrichGrantPayload_GrantRemoved_LocalIndexFillsBoth(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: "g-1", UserID: "u-1", ProjectID: "p-77", RoleKeys: []string{"alpha", "beta"}}, nil
	}
	in := WebhookPayload{EventType: "grant_removed", UserID: "u-1", GrantID: "g-1"}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-77" {
		t.Errorf("SourceProject = %q, want p-77", out.SourceProject)
	}
	if !reflect.DeepEqual(out.RoleKeys, []string{"alpha", "beta"}) {
		t.Errorf("RoleKeys = %v, want [alpha beta]", out.RoleKeys)
	}
	if out.RoleKey != "alpha" {
		t.Errorf("RoleKey = %q, want alpha", out.RoleKey)
	}
}

func TestEnrichGrantPayload_LocalMiss_ZitadelFallbackFillsBoth(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrGrantIndexNotFound
	}
	dbListUserGrantsLive = func(_ context.Context, _, _ string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{ID: "g-1", UserID: "u-1", ProjectID: "p-99", RoleKeys: []string{"x"}}, nil
	}
	in := WebhookPayload{EventType: "grant_removed", UserID: "u-1", GrantID: "g-1"}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "p-99" {
		t.Errorf("SourceProject = %q, want p-99", out.SourceProject)
	}
	if !reflect.DeepEqual(out.RoleKeys, []string{"x"}) {
		t.Errorf("RoleKeys = %v, want [x]", out.RoleKeys)
	}
}

func TestEnrichGrantPayload_BothFail_LeavesPayloadUnenriched(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrGrantIndexNotFound
	}
	dbListUserGrantsLive = func(_ context.Context, _, _ string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{}, errors.New("zitadel down")
	}
	in := WebhookPayload{EventType: "grant_changed", UserID: "u-1", GrantID: "g-1", RoleKeys: []string{"x"}}
	out := enrichGrantPayload(context.Background(), in)
	if out.SourceProject != "" {
		t.Errorf("SourceProject = %q, want empty (both lookups failed)", out.SourceProject)
	}
	if !reflect.DeepEqual(out.RoleKeys, []string{"x"}) {
		t.Errorf("RoleKeys = %v, want [x] preserved from event", out.RoleKeys)
	}
}

func TestEnrichGrantPayload_NothingNeeded_SkipsLookup(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		t.Fatal("must not query index when payload already complete")
		return db.ZitadelGrantIndex{}, nil
	}
	in := WebhookPayload{EventType: "grant_changed", UserID: "u-1", GrantID: "g-1", SourceProject: "p-7", RoleKeys: []string{"x"}}
	out := enrichGrantPayload(context.Background(), in)
	if !reflect.DeepEqual(out, in) {
		t.Errorf("payload mutated despite no enrichment needed: %+v", out)
	}
}

func TestEnrichGrantPayload_MissingGrantID_NoOp(t *testing.T) {
	setupEnrichDeps(t)
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		t.Fatal("must not query index without GrantID")
		return db.ZitadelGrantIndex{}, nil
	}
	in := WebhookPayload{EventType: "grant_removed", UserID: "u-1"}
	out := enrichGrantPayload(context.Background(), in)
	if !reflect.DeepEqual(out, in) {
		t.Errorf("payload mutated despite missing GrantID: %+v", out)
	}
}
