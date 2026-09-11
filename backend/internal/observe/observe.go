// Package observe is the only thing that asks Zitadel what is true.
//
// Everything else reads the store this writes. That is the whole point: a
// surface that calls Zitadel itself is a second pipe, and two pipes answering
// one question is how a role came to read "4 holders" on a list and "0 people
// hold this role" on the page it opens.
//
// A per-person read and an org-wide sweep are not two mechanisms. They are one
// act at two scopes, recorded the same way, and each carries whether its answer
// was COMPLETE — because presence can be concluded from any answer and absence
// only from a whole one.
package observe

import (
	"context"
	"log"
	"time"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// pageSize and cap bound a sweep. The cap is a real limit on what can be
// concluded, not a tuning knob: a listing that reaches it is truncated, says
// so, and may never delete a row.
const (
	pageSize = 500
	maxRows  = 20000
)

// Deps, injectable so the sweep is testable without a live Zitadel.
var (
	listAllGrants = func(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return zitadel.MgmtClient.ListAllGrants(ctx, p)
	}
	listUserGrants = func(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return zitadel.MgmtClient.ListUserGrants(ctx, userID, p)
	}
	recordOrg  = db.RecordOrgObservation
	recordUser = db.RecordUserObservation
	confirm    = db.ConfirmFromObservation
	clientOK   = func() bool { return zitadel.MgmtClient != nil }
)

// Org reads every grant in the organisation and records what it saw.
//
// Returns the observation rather than an error for a partial read: a sweep that
// got half way and stopped is a real observation of half the world, and saying
// so is more useful than discarding it. Only a transport failure with nothing
// read at all is nothing.
func Org(ctx context.Context) (db.Observation, error) {
	if !clientOK() {
		return db.Observation{}, nil
	}
	var seen []db.ObservedGrant
	complete, readErr := true, ""

	for offset := 0; ; offset += pageSize {
		res, err := listAllGrants(ctx, zitadel.SearchParams{Limit: pageSize, Offset: offset})
		if err != nil {
			// What was read stays read. The pass is incomplete, which is
			// exactly what stops it deleting anything.
			complete, readErr = false, err.Error()
			break
		}
		for _, g := range res.Items {
			seen = append(seen, db.ObservedGrant{
				GrantID: g.ID, UserID: g.UserID, ProjectID: g.ProjectID, RoleKeys: g.RoleKeys,
			})
		}
		if len(res.Items) < pageSize {
			break
		}
		if len(seen) >= maxRows {
			// The cap. Everything seen is real; nothing may be concluded about
			// what was not reached.
			complete, readErr = false, "the read reached its safety limit before the end of the directory"
			break
		}
	}

	if err := recordOrg(ctx, seen, complete, readErr); err != nil {
		return db.Observation{}, err
	}
	return db.Observation{
		Scope: "org", ObservedAt: time.Now().UTC(),
		Complete: complete, GrantsSeen: len(seen), Error: readErr,
	}, nil
}

// Result is an observation plus what it saw, for a caller that needs both.
//
// The drain's read-back is the case this exists for: it has to know whether the
// answer was WHOLE before it can conclude a role is gone, and it needs the
// grants themselves to say whether the one it just wrote is among them.
type Result struct {
	db.Observation
	Grants []db.ObservedGrant
}

// UserWithGrants is User, returning what was seen alongside the observation.
func UserWithGrants(ctx context.Context, userID string) (Result, error) {
	obs, grants, err := observeUser(ctx, userID)
	return Result{Observation: obs, Grants: grants}, err
}

// User reads one person's grants and records what it saw.
//
// The same act at a narrower scope. A surface that wants a person's true access
// asks for this rather than calling Zitadel, so the answer it gets is dated,
// shared with every other surface, and cannot disagree with theirs.
func User(ctx context.Context, userID string) (db.Observation, error) {
	obs, _, err := observeUser(ctx, userID)
	return obs, err
}

func observeUser(ctx context.Context, userID string) (db.Observation, []db.ObservedGrant, error) {
	if !clientOK() || userID == "" {
		return db.Observation{}, nil, nil
	}
	var seen []db.ObservedGrant
	complete, readErr := true, ""

	for offset := 0; ; offset += pageSize {
		res, err := listUserGrants(ctx, userID, zitadel.SearchParams{Limit: pageSize, Offset: offset})
		if err != nil {
			complete, readErr = false, err.Error()
			break
		}
		for _, g := range res.Items {
			seen = append(seen, db.ObservedGrant{
				GrantID: g.ID, UserID: userID, ProjectID: g.ProjectID, RoleKeys: g.RoleKeys,
			})
		}
		if len(res.Items) < pageSize {
			break
		}
		if len(seen) >= maxRows {
			// The same cap the org sweep carries, for the same reason. Nobody
			// holds twenty thousand grants, which is exactly why a loop that
			// could run forever if they did has no business being on the write
			// path — and reaching it makes the answer INCOMPLETE, so nothing
			// downstream can conclude an absence from it.
			complete, readErr = false, "the read reached its safety limit before the end of this person's grants"
			break
		}
	}

	if err := recordUser(ctx, userID, seen, complete, readErr); err != nil {
		return db.Observation{}, nil, err
	}
	return db.Observation{
		Scope: "user", SubjectID: userID, ObservedAt: time.Now().UTC(),
		Complete: complete, GrantsSeen: len(seen), Error: readErr,
	}, seen, nil
}

// Sweep is the periodic pass. Its interval is the bound on how stale any
// surface's answer can be, which is the number this whole design trades for.
func Sweep(ctx context.Context) error {
	o, err := Org(ctx)
	if err != nil {
		return err
	}
	if !o.Complete {
		log.Printf("[OBSERVE] incomplete sweep: %d grants seen, nothing removed — %s", o.GrantsSeen, o.Error)
		return nil
	}
	// One pipe: a complete listing is the only thing that may turn an accepted
	// write into a confirmed one, for adds by presence and revokes by absence.
	n, err := confirm(ctx)
	if err != nil {
		log.Printf("[OBSERVE] sweep complete: %d grants; confirmation failed: %v", o.GrantsSeen, err)
		return nil
	}
	log.Printf("[OBSERVE] sweep complete: %d grants, %d writes confirmed", o.GrantsSeen, n)
	return nil
}
