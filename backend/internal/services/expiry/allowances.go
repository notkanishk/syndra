package expiry

import (
	"context"
	"log"
)

// SweepAllowances resolves subtractive allowances whose expiry has passed.
//
// A denial is normally a time-boxed suspension and is supposed to end on its
// own — that is why the schema refuses one with neither an expiry nor a review
// date. Ending it means two things and this pass does both: the row stops
// applying, recorded rather than deleted, and the subject re-converges so the
// access the suspension was withholding actually comes back.
//
// Re-convergence is what makes this a sweep rather than a tidy-up. The resolver
// already excludes a lapsed allowance — `AllowancesInForce` compares the expiry
// in its predicate — so effective access is correct the instant the date
// passes. What is NOT correct is the target: nothing has told it, and it will
// keep the person out until something re-resolves them.
//
// And today NOTHING tells it. `reconvergeSubject` resolves the set and stops
// there, because queueing an apply needs a plan subject to cite and a
// system-initiated re-convergence has no plan — the open design question
// NEXT.md item 1 names. The drift sweep is not the fallback either: it is
// `const target = db.TargetZitadel`, and add-on drift is unbuilt, so for the
// only target class this change adds there is no second path.
//
// So the accurate claim is: the suspension ends here, Syndra's answer is right
// from this moment, and the TARGET stays as it was until an operator drives a
// change through the entitlement plane. Said plainly because the two weaker
// versions of this sentence — "the access actually comes back", then "the drift
// sweep tells it" — were both wrong, and a doc that is confidently wrong about
// what restores somebody's access is worse than one that says nothing.
//
// Deliberately NOT the same pass as grant expiry. That one removes access and
// this one restores it, they have different failure directions, and a batch
// that aborted halfway through the first would silently skip the second — on
// the half that gives access back to somebody who is owed it.
func SweepAllowances(ctx context.Context, batchSize int) {
	lapsed, err := dbLapsedAllowances(ctx, batchSize)
	if err != nil {
		log.Printf("[SCHEDULER] Failed to fetch lapsed allowances: %v", err)
		return
	}
	if len(lapsed) == 0 {
		return
	}
	log.Printf("[SCHEDULER] Allowance sweep starting: candidates=%d", len(lapsed))

	var resolved, failed int
	for _, a := range lapsed {
		if err := ctx.Err(); err != nil {
			// Stop where we are rather than finishing the list on a cancelled
			// context: the writes below would fail one by one and the log would
			// read as an outage.
			log.Printf("[SCHEDULER] Allowance sweep cancelled after %d: %v", resolved, err)
			return
		}
		// Recorded first, converged second. The record is the conditional write
		// — it matches only a row that is still in force and still lapsed — so
		// a renewal landing in this window takes the whole thing with it, and
		// nothing re-converges a subject whose suspension was just extended.
		if err := dbResolveLapsedAllowance(ctx, a.ID); err != nil {
			log.Printf("[SCHEDULER] Could not resolve lapsed allowance %s: %v", a.ID, err)
			failed++
			continue
		}
		resolved++

		// One subject's re-convergence must not cost another's. The suspension
		// has already ended in Syndra either way — the failure here is that the
		// target has not been told, which the drift sweep raises and an operator
		// can resume.
		if err := reconvergeSubject(ctx, a.SubjectID, a.Target); err != nil {
			log.Printf("[SCHEDULER] Allowance lapsed for %s on %s but re-convergence failed: %v",
				a.SubjectID, a.Target, err)
		}
	}
	log.Printf("[SCHEDULER] Allowance sweep complete: resolved=%d failed=%d", resolved, failed)
}
