package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Insights usage (I6): who has used what lately, and what is held but never
// used. Read-only here; the rows are written by the webhook.

const (
	usageDefaultDays = 90
	usageMaxDays     = 365
	// usageKeepFor is how long a row survives without being refreshed. Longer
	// than the longest window a person can ask for, so the prune never removes
	// a row a query could still want.
	usageKeepFor = 400 * 24 * time.Hour
)

// ErrInvalidUsageDays is any ?days= outside 1..365.
var ErrInvalidUsageDays = errors.New("days must be a whole number from 1 to 365")

// ParseUsageDays defaults to 90 when none is given.
func ParseUsageDays(s string) (int, error) {
	if s == "" {
		return usageDefaultDays, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > usageMaxDays {
		return 0, fmt.Errorf("%w: %q", ErrInvalidUsageDays, s)
	}
	return n, nil
}

// The reads, as seams, so the report's logic needs no database.
var (
	usageNow    = time.Now
	usageLatest = db.UsageLatestByKind
	usageUsed   = db.UsersWhoUsedProjects
	usagePrune  = db.PruneUsage
	// usageHolders is "project:role" -> the people Zitadel shows holding it
	// (observed, never recorded), minus people who have left. observed is false
	// when nothing has ever been read, in which case there are no holders to
	// report and no absence to conclude from.
	usageHolders = observedRoleHolders
)

func observedRoleHolders(ctx context.Context) (map[string][]string, bool, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, false, err
	}
	basis, err := snap.Basis()
	if err != nil {
		return nil, false, err
	}
	if basis.ReadAt == nil {
		return nil, false, nil
	}
	out := map[string][]string{}
	for _, u := range snap.Users() {
		if leftStatuses[strings.ToLower(u.Status)] {
			continue // somebody who has left holding access is its own finding
		}
		held, err := snap.Observed(u.ID)
		if err != nil {
			return nil, false, err
		}
		for key := range held {
			out[key] = append(out[key], u.ID)
		}
	}
	return out, true, nil
}

// InsightsUsage answers the usage report for the last `days` days.
//
// Approximation: a sign-in to any application of project P counts as that
// person using every role they hold in P. The token does not say which role was
// exercised, so "held but not used" means "never signed in to anything in the
// project", which is the strongest claim the signal supports and no stronger.
func InsightsUsage(ctx context.Context, days int) (models.InsightsUsage, error) {
	since := usageNow().Add(-time.Duration(days) * 24 * time.Hour)
	latest, err := usageLatest(ctx, since)
	if err != nil {
		return models.InsightsUsage{}, err
	}
	source := func(kind string) models.UsageSource {
		at, ok := latest[kind]
		if !ok {
			return models.UsageSource{}
		}
		return models.UsageSource{Receiving: true, LastAt: &at}
	}
	out := models.InsightsUsage{
		Sources:     models.UsageSources{SignIn: source(db.UsageSignIn), Badge: source(db.UsageBadge)},
		HeldNotUsed: []models.HeldNotUsed{},
		WindowDays:  days,
	}
	// Never report "unused" from silence: with no sign-ins arriving, everyone
	// would look unused.
	if !out.Sources.SignIn.Receiving {
		return out, nil
	}
	holders, observed, err := usageHolders(ctx)
	if err != nil {
		return models.InsightsUsage{}, err
	}
	out.HoldersObserved = observed
	if !observed {
		return out, nil
	}
	used, err := usageUsed(ctx, db.UsageSignIn, since)
	if err != nil {
		return models.InsightsUsage{}, err
	}
	for key, people := range holders {
		project, role, ok := strings.Cut(key, ":")
		if !ok {
			continue
		}
		idle := 0
		for _, id := range people {
			if !used[id][project] {
				idle++
			}
		}
		if idle > 0 {
			out.HeldNotUsed = append(out.HeldNotUsed, models.HeldNotUsed{ProjectID: project, RoleKey: role, People: idle})
		}
	}
	sort.Slice(out.HeldNotUsed, func(i, j int) bool {
		a, b := out.HeldNotUsed[i], out.HeldNotUsed[j]
		if a.People != b.People {
			return a.People > b.People
		}
		if a.ProjectID != b.ProjectID {
			return a.ProjectID < b.ProjectID
		}
		return a.RoleKey < b.RoleKey
	})
	return out, nil
}

// PruneUsageLastSeen drops usage rows nobody has refreshed in 400 days. Run
// daily by the periodic runner.
func PruneUsageLastSeen(ctx context.Context) error {
	n, err := usagePrune(ctx, usageNow().Add(-usageKeepFor))
	if err != nil {
		return err
	}
	if n > 0 {
		log.Printf("[FINDINGS] pruned %d usage rows older than 400 days", n)
	}
	return nil
}
