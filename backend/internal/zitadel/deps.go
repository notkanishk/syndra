package zitadel

import (
	"context"
	"net/http"
	"time"

	"syndra/internal/db"
)

// Injectable vars for testing. Production code uses real implementations.
var (
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return client.Do(req)
	}
	timeNow                 = time.Now
	tokenHTTPClient         = &http.Client{Timeout: 10 * time.Second}
	dbGetActiveMappingRules = db.GetActiveMappingRules
)

// StillExpected reports whether Syndra still accounts for (project, role) for
// this person through ANY source — a direct grant, a bundle, or another rule.
//
// It is a hook rather than a call because the answer lives in `services`, which
// imports this package; wired in services' own init so it cannot be forgotten.
//
// RevokeMappingRules needs it and did not have it. That function removes a
// rule-derived grant from Zitadel when the rule's SOURCE role goes away, and it
// asked only "does the target grant exist upstream" — never "does anything else
// still give it". So somebody holding laser/operator from a bundle AND from a
// rule lost it from Zitadel the moment an operator removed the rule's source by
// hand. Nothing repaired it: no outbox row was written, the ledger did not
// change, and the sweep's syndra_only half does not conclude absence for
// bundle- or rule-derived roles. Access destroyed, silently, from a routine
// edit.
//
// The closure path (services.effectiveClosure) has always had this check —
// "a role still covered by another source stays in `after` and is never
// revoked" — and this is the pre-closure path that never got it.
//
// Nil means unwired, and unwired means SKIP the revocation. That direction is
// deliberate: a revocation not performed leaves access the drift sweep will
// raise for triage, while one performed wrongly destroys access with no record
// that it happened.
var StillExpected func(ctx context.Context, userID, projectID, roleKey string) (bool, error)
