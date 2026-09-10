package services

import (
	"context"
	"errors"
	"fmt"

	"syndra/internal/db"
)

// MissedOnboarding is a person Zitadel confirms is an active member who does
// not hold the welcome bundle. Not a row from onboarding_triggers — that
// table is only a log of what a webhook told Syndra, and a webhook that never
// arrived (dropped, misrouted, or an event Syndra never learned to translate)
// leaves no row there at all. This answers "who joined and got nothing" from
// the two things that are actually true right now: who Zitadel says exists,
// and who the welcome bundle was actually given to.
type MissedOnboarding struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

// MissedOnboardingReport is the reconciler's whole answer, not just the list.
//
// WelcomeBundleConfigured is a fact about right now, carried explicitly rather
// than left for the reader to infer from an empty Missed — an empty list means
// two very different things ("everybody has it" vs "there is nothing to check
// anybody against") and collapsing them into one shape was the exact mistake
// that let five real accounts read as "failed" instead of "nothing configured
// to give them" (migration 000046). A screen reading this can tell "N people
// got nothing" from "there is no default at all" without a second call.
type MissedOnboardingReport struct {
	WelcomeBundleConfigured bool               `json:"welcome_bundle_configured"`
	Missed                  []MissedOnboarding `json:"missed"`
}

// FindMissedOnboarding answers "who joined and got no welcome bundle" by
// reading current state, per the one-truth-many-checks rule: never trust
// that a trigger row's absence means nothing happened, ask Zitadel and the
// assignment table directly.
//
// Computed on read, not a periodic sweep, and that is a deliberate choice:
// this deployment's documented scale is a few hundred people (see the
// ponytail note on HolderCounts/GetAllKnownUserIDs), so one directory listing
// plus one assignment query is cheap enough to run whenever an operator opens
// the screen that shows it — the same trade the People index already makes.
// A sweep would add a schedule, a table to hold its findings, and staleness
// between runs, to save a call that is not expensive enough to be worth
// saving. Revisit if the roster ever reaches the size where a full listing
// stops being a cheap read (see HolderCounts's note on the same bound).
//
// No welcome bundle configured at all is not something Missed can answer
// against — see TriggerOnboarding and migration 000046: that state may be
// deliberate, and either way there is no assignment to have missed yet. Missed
// comes back empty rather than flagging every active person, and
// WelcomeBundleConfigured=false is how a caller still learns the gap exists.
func FindMissedOnboarding(ctx context.Context) (MissedOnboardingReport, error) {
	welcomeBundleID, err := svcGetWelcomeBundle(ctx)
	if err != nil {
		if errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
			return MissedOnboardingReport{WelcomeBundleConfigured: false, Missed: []MissedOnboarding{}}, nil
		}
		return MissedOnboardingReport{}, fmt.Errorf("find missed onboarding: %w", err)
	}

	holders, err := svcGetUsersForBundle(ctx, welcomeBundleID)
	if err != nil {
		return MissedOnboardingReport{}, fmt.Errorf("find missed onboarding: load welcome bundle holders: %w", err)
	}
	holderSet := make(map[string]struct{}, len(holders))
	for _, id := range holders {
		holderSet[id] = struct{}{}
	}

	people, err := svcDirectoryUsers(ctx)
	if err != nil {
		return MissedOnboardingReport{}, fmt.Errorf("find missed onboarding: list people: %w", err)
	}

	missed := []MissedOnboarding{}
	for _, p := range people {
		// Only active members are expected to hold the welcome bundle — someone
		// deactivated, locked or already gone is not "missed", they are gone.
		if p.Status != "active" {
			continue
		}
		if _, ok := holderSet[p.ID]; ok {
			continue
		}
		missed = append(missed, MissedOnboarding{UserID: p.ID, Name: p.Name, Email: p.Email})
	}
	return MissedOnboardingReport{WelcomeBundleConfigured: true, Missed: missed}, nil
}
