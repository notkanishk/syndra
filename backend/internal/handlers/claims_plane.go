package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"

	"syndra/internal/claimserve"
	"syndra/internal/services/periodic"
)

// The claims data plane (claims-delivery-integrity S2, ADR-004): its own
// listener, its own mux, an in-memory config snapshot and one bounded Redis
// read per call. Nothing here reads Postgres or Zitadel; the write side below
// publishes into Redis and the plane follows within a second.

// claimPlane answers the Actions v2 calls that reach this process (the :8080
// route, the fallback until Caddy routes to the claims container on :8090),
// and is what the detectors read the snapshot from. Before StartClaimsPlane it
// has no config and answers degraded.
var claimPlane = claimserve.New(nil, nil)

// StartClaimsPlane builds the plane over rdb and keeps its snapshot current
// until ctx ends. Called first in main, before migrations: it needs only
// Redis, and serves degraded (counted) until Redis answers.
func StartClaimsPlane(ctx context.Context, rdb claimserve.Redis) {
	claimPlane = claimserve.New(rdb, nil)
	_ = claimPlane.Poll(ctx)
	go func() {
		for ctx.Err() == nil {
			func() {
				defer periodic.Recover("CLAIMS-SNAPSHOT")
				claimPlane.Run(ctx)
			}()
		}
	}()
}

// ---------------------------------------------------------------------------
// The write side: every shape save, restore, seed and failure-mode write goes
// through publishClaimConfig, which resolves the project from Postgres and
// writes it through (claimserve.Publish moves claims:gen). PublishAllClaimConfig
// runs at start and every minute, so a write whose publish failed, a failure
// mode edited in SQL, or a project new in Zitadel lands within a minute.
// ---------------------------------------------------------------------------

// publishClaimConfig resolves and publishes projects. A project that can't be
// resolved is left as it was, never published as a guess.
func publishClaimConfig(ctx context.Context, projectIDs ...string) error {
	return publishProjects(ctx, nil, projectIDs...)
}

// publishProjects resolves projectIDs and publishes them with extra (already
// resolved) in one write, so one pass moves claims:gen at most once.
func publishProjects(ctx context.Context, extra map[string]claimserve.Project, projectIDs ...string) error {
	projects := make(map[string]claimserve.Project, len(projectIDs)+len(extra))
	for id, p := range extra {
		projects[id] = p
	}
	var errs []error
	for _, id := range projectIDs {
		if id == "" {
			continue
		}
		shapes, err := dbResolveProjectShapes(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("project %s shapes: %w", id, err))
			continue
		}
		m, err := dbGetClaimMode(ctx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("project %s failure mode: %w", id, err))
			continue
		}
		projects[id] = claimserve.Project{Shapes: shapes, Mode: claimserve.Mode{
			Mode: m.Mode, MinimalSafeClaims: m.MinimalSafeClaims, LKGMaxAgeS: m.LKGMaxAgeSeconds,
		}}
	}
	rdb := claimConfigRedis()
	if rdb == nil {
		return errors.Join(append(errs, errors.New("redis is not connected"))...)
	}
	if _, err := claimserve.Publish(ctx, rdb, projects); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// publishAfterSave publishes after a save already committed in Postgres. A
// failure is logged, not returned: the save stands, and the minute pass
// publishes it.
func publishAfterSave(ctx context.Context, projectIDs ...string) {
	if err := publishClaimConfig(ctx, projectIDs...); err != nil {
		log.Printf("[CLAIM-SHAPE] ERROR the data plane has not got this save yet (the next minute's publish retries): %v", err)
	}
}

// PublishAllClaimConfig publishes every project Syndra or Zitadel knows. Only
// what changed is written, so a quiet minute costs the data plane nothing.
func PublishAllClaimConfig(ctx context.Context) error {
	seen := map[string]bool{}
	var ids []string
	add := func(list []string) {
		for _, id := range list {
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	fromDB, dbErr := dbClaimConfigProjects(ctx)
	add(fromDB)
	// Zitadel's projects too, so one with no config of its own is published
	// with the built-in shape instead of answering fail closed as unknown.
	fromDir, dirErr := dirProjectIDs(ctx)
	add(fromDir)
	if dirErr != nil {
		log.Printf("[CLAIM-SHAPE] directory unavailable, publishing the %d projects Syndra holds config for: %v", len(ids), dirErr)
	}
	// The L1 canary's project rides along (S6): published like any other, so
	// the probe exercises the same write-through and reload.
	canary := map[string]claimserve.Project{claimserve.CanaryProjectID: claimserve.CanaryProject()}
	return errors.Join(dbErr, publishProjects(ctx, canary, ids...))
}
