package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"syndra/internal/textsort"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/access"
	"syndra/internal/cache"
	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

type roleKey struct {
	projectID string
	roleKey   string
}

type userRoles struct {
	roleMap map[roleKey]*models.EffectiveRole
	bundles []models.Bundle
}

// collectUserRolesHook is the indirection accessSnapshot calls. Production
// code points it at collectUserRoles; tests override it to count
// invocations. Single-process global — fine because tests run sequentially
// in this package and t.Cleanup restores the original.
var collectUserRolesHook = collectUserRoles

// accessSnapshot is a request-scoped lazy cache for (user → effective roles).
//
// The role-resolution helper collectUserRoles is fast but repeatedly called
// from views that iterate users — ListApplications walks N×M (per user × per
// app), ListProjects walks N×P. The snapshot computes-and-memoises each
// user once per request so cross-view aggregate fan-out collapses to O(N).
//
// The snapshot is NOT process-wide: no mutex, no invalidation, no expiry.
// It lives for the lifetime of one HTTP handler call and is GC'd when the
// request returns.
type accessSnapshot struct {
	ctx   context.Context
	users []models.UserProfile
	roles map[string]userRoles

	// observed is the lazy, memoised twin of roles: what the observation store
	// (not Syndra's own tables) confirmed for each user, keyed the same way
	// roleKey is — "projectID:roleKey". Populated by Observed().
	observed map[string]map[string]bool
	// basis is the one observation-basis fact for the whole snapshot — see
	// Basis(). Computed at most once per request, same as everything else here.
	basis         models.ObservationBasis
	basisComputed bool
	// standings is the lazy, memoised standing of each user's roles, from
	// Observed, BasisFor and the user's outbox rows. See Standing.
	standings map[string]Standings
	// deliveries and ownReads are read once for every user of the snapshot
	// (or once for the one person of a personSnapshot): see primeFor.
	deliveries map[string]db.Deliveries
	ownReads   map[string]db.Observation
	primed     map[string]bool
}

// newAccessSnapshot primes the user list (single directory call) but defers
// role resolution to lazy For() calls.
func newAccessSnapshot(ctx context.Context) (*accessSnapshot, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return nil, err
	}
	return &accessSnapshot{
		ctx:      ctx,
		users:    users,
		roles:    make(map[string]userRoles, len(users)),
		observed: make(map[string]map[string]bool, len(users)),
	}, nil
}

// Standing is the standing of each of userID's roles: the one answer to "does
// this person hold this role", from what Zitadel shows (Observed, under
// BasisFor) and the latest outbox row for the role. Lazy and memoised like For.
func (s *accessSnapshot) Standing(userID string) (Standings, error) {
	if st, ok := s.standings[userID]; ok {
		return st, nil
	}
	observed, err := s.Observed(userID)
	if err != nil {
		return Standings{}, err
	}
	basis, err := s.BasisFor(userID)
	if err != nil {
		return Standings{}, err
	}
	if s.standings == nil {
		s.standings = map[string]Standings{}
	}
	st := newStandings(s.deliveries[userID], observed, basis)
	s.standings[userID] = st
	return st, nil
}

// BasisFor is the footing for one person's absences (personBasis): the newer
// of the org sweep and their own latest read.
func (s *accessSnapshot) BasisFor(userID string) (models.ObservationBasis, error) {
	org, err := s.Basis()
	if err != nil {
		return models.ObservationBasis{}, err
	}
	if err := s.primeFor(userID); err != nil {
		return models.ObservationBasis{}, err
	}
	if own, ok := s.ownReads[userID]; ok {
		return personBasis(org, &own), nil
	}
	return personBasis(org, nil), nil
}

// primeFor reads the outbox rows and own reads behind standings once: for
// every user of the snapshot in one read each (a role's holder list asks for
// many), or for the one person a personSnapshot is about.
func (s *accessSnapshot) primeFor(userID string) error {
	if s.primed[userID] {
		return nil
	}
	ids := []string{userID}
	if len(s.users) > 0 {
		ids = make([]string, 0, len(s.users)+1)
		for _, u := range s.users {
			ids = append(ids, u.ID)
		}
		ids = append(ids, userID) // asked about, whether or not the directory lists them
	}
	deliveries, err := svcRoleDeliveries(s.ctx, ids)
	if err != nil {
		return err
	}
	own, err := svcLatestUserObservations(s.ctx, ids)
	if err != nil {
		return err
	}
	if s.primed == nil {
		s.primed, s.deliveries, s.ownReads = map[string]bool{}, map[string]db.Deliveries{}, map[string]db.Observation{}
	}
	for _, id := range ids {
		s.primed[id] = true
		if d, ok := deliveries[id]; ok {
			s.deliveries[id] = d
		}
		if o, ok := own[id]; ok {
			s.ownReads[id] = o
		}
	}
	return nil
}

// personSnapshot is a snapshot for one person's page: no directory listing,
// since For, Observed, Basis and Standing need only the id.
func personSnapshot(ctx context.Context) *accessSnapshot {
	return &accessSnapshot{ctx: ctx, roles: map[string]userRoles{}, observed: map[string]map[string]bool{}}
}

// For returns the cached (roleMap, bundles) for userID, computing-and-
// caching on first access.
func (s *accessSnapshot) For(userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	if r, ok := s.roles[userID]; ok {
		return r.roleMap, r.bundles, nil
	}
	roleMap, bundles, err := collectUserRolesHook(s.ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	s.roles[userID] = userRoles{roleMap: roleMap, bundles: bundles}
	return roleMap, bundles, nil
}

// HolderFacts is three readings of ONE snapshot, each keyed "project:role".
//
// Given is Recorded — Syndra decided somebody should hold it. Observed is what
// Zitadel actually holds, as of Basis, whoever gave it. Confirmed is their
// intersection. A surface that says "who holds this" shows Observed; Given and
// Confirmed only explain it — given by Syndra, not yet in Zitadel, unexplained.
// Observed and Confirmed are nil, not empty, when Basis.ReadAt is nil: nothing
// has been read, so there is no absence to report.
type HolderFacts struct {
	Given     map[string]int
	Confirmed map[string]int
	Observed  map[string]int
	Basis     models.ObservationBasis
}

// NobodyObserved is the only way to conclude that no one holds key: a complete,
// current observation in which the key does not appear. A nil map (never
// observed) or a truncated read cannot say so — Go's zero for a missing map
// entry must never be read as Zitadel's answer.
func (f HolderFacts) NobodyObserved(key string) bool {
	return f.Observed != nil && f.Basis.Current && !f.Basis.Truncated && f.Observed[key] == 0
}

// RoleHolderCounts is the recorded reading of RoleHolderFacts — how many people
// each role was GIVEN to. Never presentable as somebody having access.
func RoleHolderCounts(ctx context.Context) (map[string]int, error) {
	f, err := RoleHolderFacts(ctx)
	return f.Given, err
}

// RoleHolderFacts answers every holder question from ONE snapshot.
//
// There used to be two counting mechanisms and they disagreed on screen; then
// one snapshot read twice, which cost double for nothing. Now one snapshot,
// three readings, and no surface can show a number the others cannot derive.
func RoleHolderFacts(ctx context.Context) (HolderFacts, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return HolderFacts{}, err
	}
	given, err := snap.HolderCounts()
	if err != nil {
		return HolderFacts{}, err
	}
	confirmed, basis, err := snap.ConfirmedHolderCounts()
	if err != nil {
		return HolderFacts{}, err
	}
	observed, err := snap.ObservedHolderCounts()
	if err != nil {
		return HolderFacts{}, err
	}
	return HolderFacts{Given: given, Confirmed: confirmed, Observed: observed, Basis: basis}, nil
}

// HolderCounts is how many people hold each role, keyed "project:role".
//
// The ONE place a holder count is computed. There used to be a second —
// `GetEffectiveUserCounts`, a UNION over the ledger and the bundle tables — and
// the two disagreed in the field: a role listed with "4 holders" on /roles
// whose own page said "0 people hold this role". Both numbers were arrived at
// honestly and neither said which question it had answered.
//
// They could not agree, because the SQL could not see what this can. It counted
// ledger rows for people the directory no longer returns, and it resolved no
// mapping rules at all — its own comment admitted the second. Deriving the
// count from the same snapshot that answers "who holds this" makes the list and
// the page the same statement at two resolutions, which is the only arrangement
// in which they cannot contradict each other.
//
// ponytail: one directory listing plus a collectUserRoles per user, memoised
// for the request. At this deployment's documented ~200-user scale that is the
// right trade against a count that can be wrong; past it, cache the snapshot
// across requests rather than reintroducing a second way to count.
func (s *accessSnapshot) HolderCounts() (map[string]int, error) {
	counts := make(map[string]int)
	for _, u := range s.users {
		roleMap, _, err := s.For(u.ID)
		if err != nil {
			return nil, err
		}
		for key := range roleMap {
			counts[key.projectID+":"+key.roleKey]++
		}
	}
	return counts, nil
}

// Users returns the list primed at construction. Cheap accessor so view
// functions don't re-fetch the directory.
func (s *accessSnapshot) Users() []models.UserProfile {
	return s.users
}

// Observed is the "projectID:roleKey" pairs the observation store confirms
// for this user — Zitadel's own answer, not Syndra's. Lazy and memoised like
// For, so a caller already visiting every user for HolderCounts pays one more
// cheap read per user, not a second pass over the snapshot.
func (s *accessSnapshot) Observed(userID string) (map[string]bool, error) {
	if o, ok := s.observed[userID]; ok {
		return o, nil
	}
	grants, err := svcObservedGrantsFor(s.ctx, userID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(grants))
	for _, g := range grants {
		for _, rk := range g.RoleKeys {
			set[g.ProjectID+":"+rk] = true
		}
	}
	s.observed[userID] = set
	return set, nil
}

// ObservedRoles is the person's roles in a project as Syndra last read them
// from Zitadel, as one claims.RoleSet: what a token's payload would list. Every
// preview takes a token's roles from here, as the data plane takes them from
// the payload, so neither reads Syndra's records for them (ADR-004). absent is
// true only when there are none and the last sweep was complete and current:
// the one footing for "Zitadel gives them nothing here" (NobodyObserved's rule).
func ObservedRoles(ctx context.Context, userID, projectID string) (roles []string, absent bool, err error) {
	grants, err := svcObservedGrantsFor(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	for _, g := range grants {
		if g.ProjectID == projectID {
			roles = append(roles, g.RoleKeys...)
		}
	}
	if roles = claims.RoleSet(roles); len(roles) > 0 {
		return roles, false, nil
	}
	basis, err := observationBasis(ctx)
	if err != nil {
		return nil, false, err
	}
	return nil, basis.ReadAt != nil && basis.Current && !basis.Truncated, nil
}

// Basis is the one observation-basis fact for this snapshot's counts: whether
// the org has ever been swept, how long ago, and whether that sweep was
// complete. db.ErrNoObservation becomes the zero value — ReadAt nil — which is
// the "not checked yet" case every caller must handle before trusting a
// confirmed count of zero.
func (s *accessSnapshot) Basis() (models.ObservationBasis, error) {
	if s.basisComputed {
		return s.basis, nil
	}
	basis, err := observationBasis(s.ctx)
	if err != nil {
		return models.ObservationBasis{}, err
	}
	s.basis = basis
	s.basisComputed = true
	return s.basis, nil
}

// observationBasis is the one place that turns the latest org sweep into the
// footing a number rests on. Every surface that reports an observed count cites
// it, so there is exactly one answer to "how current is this" in the product.
//
// Nothing observed yet is not an error: it is a zero-value basis with a nil
// ReadAt, which every reader is required to render as "not checked yet".
func observationBasis(ctx context.Context) (models.ObservationBasis, error) {
	obs, err := svcLatestOrgObservation(ctx)
	if errors.Is(err, db.ErrNoObservation) {
		return models.ObservationBasis{}, nil
	}
	if err != nil {
		return models.ObservationBasis{}, err
	}
	return basisOf(obs), nil
}

// basisOf is one read as the footing a number rests on, for the org sweep and
// a person's own read alike.
func basisOf(obs db.Observation) models.ObservationBasis {
	at := obs.ObservedAt
	return models.ObservationBasis{
		ReadAt: &at,
		// Current is false only when the read itself failed. A capped-but-
		// clean read is still current; Truncated says the cap part on its own.
		Current:   obs.Error == "",
		Truncated: !obs.Complete,
	}
}

// personBasis is the footing for one person: the newer of the org sweep and
// their own latest read. A complete read of one person rewrites all of their
// rows (db.RecordUserObservation), so when it is the newer it settles their
// absences as a complete sweep would; an older one never overrides a newer
// sweep, which may have seen them change.
func personBasis(org models.ObservationBasis, own *db.Observation) models.ObservationBasis {
	if own == nil || (org.ReadAt != nil && !own.ObservedAt.After(*org.ReadAt)) {
		return org
	}
	return basisOf(*own)
}

// ConfirmedHolderCounts is HolderCounts' companion: of the people HolderCounts
// says hold each role, how many the observation store also shows holding it —
// plus the basis that number rests on. Nil counts (not an empty map) when the
// basis has no ReadAt: nothing has ever been observed, so there is nothing to
// report as confirmed, and reporting zero would read as a checked absence.
func (s *accessSnapshot) ConfirmedHolderCounts() (map[string]int, models.ObservationBasis, error) {
	basis, err := s.Basis()
	if err != nil {
		return nil, models.ObservationBasis{}, err
	}
	if basis.ReadAt == nil {
		return nil, basis, nil
	}
	counts := make(map[string]int)
	for _, u := range s.users {
		roleMap, _, err := s.For(u.ID)
		if err != nil {
			return nil, basis, err
		}
		observed, err := s.Observed(u.ID)
		if err != nil {
			return nil, basis, err
		}
		for key := range roleMap {
			if observed[key.projectID+":"+key.roleKey] {
				counts[key.projectID+":"+key.roleKey]++
			}
		}
	}
	return counts, basis, nil
}

// ObservedHolderCounts is how many people Zitadel shows holding each role,
// whoever gave it — the only reading a screen may call "holders". Nil when
// nothing has been observed, for the same reason ConfirmedHolderCounts is.
func (s *accessSnapshot) ObservedHolderCounts() (map[string]int, error) {
	basis, err := s.Basis()
	if err != nil {
		return nil, err
	}
	if basis.ReadAt == nil {
		return nil, nil
	}
	counts := make(map[string]int)
	for _, u := range s.users {
		observed, err := s.Observed(u.ID)
		if err != nil {
			return nil, err
		}
		for key := range observed {
			counts[key]++
		}
	}
	return counts, nil
}

// hasObservedProject is hasProjectRole's observed twin, over the store's
// "projectID:roleKey" keys rather than Syndra's role map.
func hasObservedProject(observed map[string]bool, projectID string) bool {
	for key := range observed {
		if strings.HasPrefix(key, projectID+":") {
			return true
		}
	}
	return false
}

func Catalog(ctx context.Context) (models.CatalogResponse, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	projects, err := directory.Default.Projects(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return models.CatalogResponse{}, err
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return models.CatalogResponse{}, fmt.Errorf("read service accounts: %w", err)
	}
	users = flagServiceAccounts(users, marks)
	// The catalog resolves names for every signed-in reader; when someone
	// joined is the People list's business, not theirs.
	for i := range users {
		users[i].Joined = nil
	}
	return models.CatalogResponse{
		Users:        users,
		Projects:     projects,
		Applications: apps,
	}, nil
}

func ListUsers(ctx context.Context, query string) ([]models.UserListItem, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	attention, err := loadAttention(ctx)
	if err != nil {
		return nil, err
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("read service accounts: %w", err)
	}
	snap.users = flagServiceAccounts(snap.users, marks)
	return listUsersFromSnapshot(snap, query, attention)
}

// attentionIndex is the "needs attention" side of the People index, loaded once
// per request rather than per row: three cheap whole-table reads instead of
// three queries times N people.
type attentionIndex struct {
	expiring    map[string]int
	soonest     map[string]time.Time
	requests    map[string]int
	unexplained map[string]int
	unseen      map[string]int
}

func loadAttention(ctx context.Context) (attentionIndex, error) {
	idx := attentionIndex{
		expiring:    map[string]int{},
		soonest:     map[string]time.Time{},
		requests:    map[string]int{},
		unexplained: map[string]int{},
		unseen:      map[string]int{},
	}

	grants, err := svcGetExpiringDirectGrants(ctx, reviewHorizon)
	if err != nil {
		return idx, fmt.Errorf("load expiring grants: %w", err)
	}
	for _, g := range grants {
		idx.expiring[g.UserID]++
		if g.ExpiresAt == nil {
			continue
		}
		if cur, ok := idx.soonest[g.UserID]; !ok || g.ExpiresAt.Before(cur) {
			idx.soonest[g.UserID] = *g.ExpiresAt
		}
	}

	requests, err := svcGetAccessRequests(ctx, "pending")
	if err != nil {
		return idx, fmt.Errorf("load pending requests: %w", err)
	}
	for _, r := range requests {
		idx.requests[r.RequesterID]++
	}

	// Drift is operator-only data; a failure here must not blank the whole
	// index. It is one column of one screen, and "we couldn't count" is better
	// rendered as zero than as a 500 on the People page.
	drift, err := svcGetPendingDriftItems(ctx)
	if err == nil {
		for _, d := range drift {
			idx.unexplained[d.UserID]++
		}
	}

	// Writes Zitadel accepted and no read has seen back: the same read as Home's
	// "Sent, not seen back", so the card and the filter it opens count alike. Same
	// tolerance as drift: a failure is one filter's worth of zeros, not a 500.
	if unconfirmed, err := svcGetUnconfirmedWrites(ctx); err == nil {
		idx.unseen = unconfirmed.Users
		if idx.unseen == nil {
			idx.unseen = map[string]int{}
		}
	}

	return idx, nil
}

func listUsersFromSnapshot(snap *accessSnapshot, query string, attention attentionIndex) ([]models.UserListItem, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]models.UserListItem, 0, len(snap.Users()))
	basis, err := snap.Basis()
	if err != nil {
		return nil, fmt.Errorf("observation basis: %w", err)
	}

	for _, user := range snap.Users() {
		roleMap, bundles, err := snap.For(user.ID)
		if err != nil {
			return nil, err
		}

		// Search matches role keys as well as names and emails: "who has
		// `trained` in the laser lab" gets typed here before anyone thinks to
		// go to Roles, and a search that silently ignores it looks broken.
		if query != "" && !matchesUser(user, query) && !holdsMatchingRole(roleMap, query) {
			continue
		}

		keyProjects := make([]string, 0, len(roleMap))
		keyProjectIDs := make([]string, 0, len(roleMap))
		seenProjects := make(map[string]bool)
		for _, role := range roleMap {
			if seenProjects[role.ProjectID] {
				continue
			}
			seenProjects[role.ProjectID] = true
			keyProjects = append(keyProjects, role.ProjectName)
			keyProjectIDs = append(keyProjectIDs, role.ProjectID)
		}
		textsort.Strings(keyProjects)
		sort.Strings(keyProjectIDs)

		bundleNames := make([]string, 0, len(bundles))
		bundleVersions := make(map[string]int, len(bundles))
		for _, b := range bundles {
			bundleNames = append(bundleNames, b.Name)
			bundleVersions[b.Name] = b.PinnedVersion
		}
		textsort.Strings(bundleNames)

		item := models.UserListItem{
			User:               user,
			BundleCount:        len(bundles),
			BundleNames:        bundleNames,
			BundleVersions:     bundleVersions,
			EffectiveRoleCount: len(roleMap),
			ProjectCount:       len(keyProjects),
			KeyProjects:        keyProjects,
			KeyProjectIDs:      keyProjectIDs,
			ExpiringCount:      attention.expiring[user.ID],
			OpenRequestCount:   attention.requests[user.ID],
			UnexplainedCount:   attention.unexplained[user.ID],
			UnseenCount:        attention.unseen[user.ID],
			Observation:        basis,
		}
		if basis.ReadAt != nil {
			observed, err := snap.Observed(user.ID)
			if err != nil {
				return nil, err
			}
			projects := map[string]bool{}
			for key := range observed {
				projects[strings.SplitN(key, ":", 2)[0]] = true
			}
			roles, projs := len(observed), len(projects)
			item.ObservedRoleCount = &roles
			item.ObservedProjectCount = &projs
		}
		if when, ok := attention.soonest[user.ID]; ok {
			soonest := when
			item.SoonestExpiry = &soonest
		}
		items = append(items, item)
	}

	textsort.ByName(items, func(i models.UserListItem) string { return i.User.Name }, func(i models.UserListItem) string { return i.User.ID })

	return items, nil
}

// holdsMatchingRole reports whether any role this person holds matches the
// query by key or display name.
func holdsMatchingRole(roleMap map[roleKey]*models.EffectiveRole, query string) bool {
	for key := range roleMap {
		if strings.Contains(strings.ToLower(key.roleKey), query) {
			return true
		}
	}
	return false
}

// allowanceBand builds the third access band for a subject.
//
// Every allowance ever recorded, not only the ones in force. The question a
// surface asks here is "what has been decided about this person", and an answer
// showing only what still applies would erase every suspension that ended —
// which is precisely the history this layer keeps attached to the person rather
// than erasing into an absence.
func allowanceBand(ctx context.Context, userID string) ([]models.AllowanceBand, error) {
	rows, err := svcAllowancesForSubject(ctx, userID)
	if err != nil {
		return []models.AllowanceBand{}, err
	}
	now := time.Now()
	out := make([]models.AllowanceBand, 0, len(rows))
	for _, a := range rows {
		band := models.AllowanceBand{
			ID: a.ID, Target: a.Target, Field: a.Field, Value: a.Value,
			Direction: a.Direction, ActorID: a.ActorID, Reason: a.Reason,
			// Derived here rather than read from a column, so "in force" cannot
			// be stale while the date it depends on passes.
			InForce:   a.InForce(now),
			ReviewDue: a.ReviewDue(now),
			CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
		}
		switch {
		case a.LiftedAt != nil:
			band.Ended, band.EndedBy = a.LiftedAt.UTC().Format(time.RFC3339), a.LiftedBy
		case a.ExpiresAt != nil && !a.ExpiresAt.After(now):
			// Lapsed is not lifted. The row keeps `lifted_at` NULL until the
			// sweep records it, so an allowance that ran out and one somebody
			// ended stay distinguishable forever — and the band says which.
			band.Ended, band.EndedBy = a.ExpiresAt.UTC().Format(time.RFC3339), "the expiry date"
		}
		out = append(out, band)
	}
	return out, nil
}

func ExplainUserAccess(ctx context.Context, userID string) (models.UserAccessView, error) {
	user, ok, err := directory.Default.FindUser(ctx, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}
	if !ok {
		return models.UserAccessView{}, fmt.Errorf("user %q not found in directory", userID)
	}

	roleMap, bundles, err := collectUserRoles(ctx, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}

	// What Zitadel shows this person holding. Read here, beside the records,
	// so one response carries both — a screen that fetched the records and
	// then asked a second endpoint what was real could render the two against
	// each other, and the member landing did exactly that.
	snap := personSnapshot(ctx)
	observed, basis, err := observedRolesByProject(snap, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}
	// Presence may be concluded from any read; absence only from a complete
	// one. Everything below that states "you hold nothing here" checks this.
	completeRead := basis.ConcludesAbsence()
	// Each role's standing, from the same read and the person's outbox rows:
	// the one answer the screens render (design.md, W3b).
	standings, err := snap.Standing(userID)
	if err != nil {
		return models.UserAccessView{}, err
	}
	// Here and not inside collectUserRoles. Delivery is a fact this VIEW owes
	// its reader; the drift detector and the entitlement resolver call the same
	// collector and want Syndra's records exactly as recorded — and neither
	// should pay a per-user query for a marker it never renders. A source is
	// marked only while the role's latest row is a give on its way
	// (standings.Giving), so the marker and the standing never disagree.
	if err := markQueuedDeliveries(ctx, userID, roleMap, standings.Giving); err != nil {
		return models.UserAccessView{}, err
	}
	direct, err := directGrantsByRole(ctx, userID)
	if err != nil {
		return models.UserAccessView{}, err
	}

	projectBuckets := make(map[string]*models.ProjectAccessView)
	for _, role := range roleMap {
		bucket := projectBuckets[role.ProjectID]
		if bucket == nil {
			bucket = &models.ProjectAccessView{
				ProjectID:           role.ProjectID,
				ProjectName:         role.ProjectName,
				ProjectNameResolved: role.ProjectNameResolved,
				SourceRoles:         []models.EffectiveRole{},
				DerivedRoles:        []models.EffectiveRole{},
				EffectiveRoleKeys:   []string{},
			}
			projectBuckets[role.ProjectID] = bucket
		}

		if role.IsSource {
			bucket.SourceRoles = append(bucket.SourceRoles, *role)
		} else {
			bucket.DerivedRoles = append(bucket.DerivedRoles, *role)
		}
		bucket.EffectiveRoleKeys = append(bucket.EffectiveRoleKeys, role.RoleKey)
	}

	// A project Zitadel holds for them that Syndra has no reason for still
	// belongs on this page. It is access they have — the operator sees it as
	// "unexplained", and the person holding it should not be the only one who
	// cannot see it at all.
	for projectID, roleKeys := range observed {
		if projectBuckets[projectID] != nil {
			continue
		}
		name := projectID
		if resolved, nameErr := directory.Default.ProjectName(ctx, projectID); nameErr == nil {
			name = resolved
		}
		projectBuckets[projectID] = &models.ProjectAccessView{
			ProjectID:           projectID,
			ProjectName:         name,
			ProjectNameResolved: name != projectID,
			SourceRoles:         []models.EffectiveRole{},
			DerivedRoles:        []models.EffectiveRole{},
			EffectiveRoleKeys:   []string{},
			ObservedRoleKeys:    roleKeys,
		}
	}

	projects := make([]models.ProjectAccessView, 0, len(projectBuckets))
	for _, bucket := range projectBuckets {
		if bucket.SourceRoles == nil {
			bucket.SourceRoles = []models.EffectiveRole{}
		}
		if bucket.DerivedRoles == nil {
			bucket.DerivedRoles = []models.EffectiveRole{}
		}
		if bucket.EffectiveRoleKeys == nil {
			bucket.EffectiveRoleKeys = []string{}
		}
		sort.Slice(bucket.SourceRoles, func(i, j int) bool {
			return bucket.SourceRoles[i].RoleKey < bucket.SourceRoles[j].RoleKey
		})
		sort.Slice(bucket.DerivedRoles, func(i, j int) bool {
			return bucket.DerivedRoles[i].RoleKey < bucket.DerivedRoles[j].RoleKey
		})
		sort.Strings(bucket.EffectiveRoleKeys)
		if bucket.ObservedRoleKeys == nil && observed != nil {
			bucket.ObservedRoleKeys = observed[bucket.ProjectID]
			// Nothing observed on this project. Saying so out loud is an
			// absence claim, so it needs a read that saw everything: a capped
			// sweep upserts only what it reached and deletes nothing, which
			// leaves a person it never got to looking exactly like a person
			// who holds nothing. Presence above is safe either way — seeing a
			// grant is proof it is there, whatever the read missed.
			if bucket.ObservedRoleKeys == nil && completeRead {
				bucket.ObservedRoleKeys = []string{}
			}
		}
		bucket.Standings = map[string]*models.RoleStanding{}
		bucket.Holdings = map[string]models.RequestHolding{}
		for _, keys := range [][]string{bucket.EffectiveRoleKeys, bucket.ObservedRoleKeys} {
			for _, key := range keys {
				st := standings.Of(bucket.ProjectID, key)
				bucket.Standings[key] = st
				bucket.Holdings[key] = holdingFrom(st, standings.Taking(bucket.ProjectID, key), direct[access.Role{ProjectID: bucket.ProjectID, RoleKey: key}])
			}
		}
		projects = append(projects, *bucket)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ProjectName < projects[j].ProjectName
	})

	// The third band. Read failure is non-fatal and says so rather than
	// silently returning an access view with no carve-outs in it: an empty band
	// and an unread band look identical to a surface, and one of them means
	// "this person is suspended from something and we could not tell you".
	allowances, allowanceErr := allowanceBand(ctx, userID)

	hints := make([]string, 0, 2)
	if allowanceErr != nil {
		hints = append(hints, "Carve-outs could not be read, so this view may not show every restriction in force.")
	}
	// Said only of somebody who holds something: for a person who holds nothing
	// "everything they hold was given directly" contradicts the empty view.
	if len(bundles) == 0 && len(roleMap) > 0 {
		hints = append(hints, "No bundle yet: everything they hold was given directly.")
	}
	if len(roleMap) >= 5 {
		hints = append(hints, "This user spans several systems; review whether every downstream permission is still required.")
	}

	// A total is a statement about everything, so it rests on a read that saw
	// everything. Under a capped or failed sweep the honest answer is "we have
	// not finished looking", not a number that would read as the whole of what
	// this person holds.
	var observedRoleCount *int
	if completeRead {
		total := 0
		for _, roleKeys := range observed {
			total += len(roleKeys)
		}
		observedRoleCount = &total
	}

	return models.UserAccessView{
		Allowances:        allowances,
		User:              user,
		Bundles:           ensureBundles(bundles),
		Projects:          projects,
		CleanupHints:      hints,
		ObservedRoleCount: observedRoleCount,
		Observation:       basis,
	}, nil
}

// observedRolesByProject is what the observation store holds for one person,
// folded to project → roles, with the basis it rests on: the person's own
// (BasisFor), so the page's read line, its observed lists and its standings all
// stand on one footing. A person with two grant rows on one project keeps the
// roles of both.
//
// Returns a nil map — never an empty one — when nothing has ever been observed.
// The distinction is the whole point: an empty map is "we looked and they hold
// nothing", and nil is "we have not looked", and a screen that confuses the two
// tells somebody their access is gone when it is merely unread.
func observedRolesByProject(snap *accessSnapshot, userID string) (map[string][]string, models.ObservationBasis, error) {
	basis, err := snap.BasisFor(userID)
	if err != nil {
		return nil, models.ObservationBasis{}, err
	}
	if basis.ReadAt == nil {
		return nil, basis, nil
	}
	seen, err := snap.Observed(userID)
	if err != nil {
		return nil, models.ObservationBasis{}, fmt.Errorf("observed grants for %s: %w", userID, err)
	}
	byProject := make(map[string][]string, len(seen))
	for key := range seen {
		projectID, roleKey, _ := strings.Cut(key, ":")
		byProject[projectID] = append(byProject[projectID], roleKey)
	}
	for _, keys := range byProject {
		sort.Strings(keys)
	}
	return byProject, basis, nil
}

func ListApplications(ctx context.Context) ([]models.ApplicationView, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	views, err := listApplicationsFromSnapshot(snap)
	if err != nil {
		return nil, err
	}
	attachClaimLines(ctx, views)
	return views, nil
}

func listApplicationsFromSnapshot(snap *accessSnapshot) ([]models.ApplicationView, error) {
	apps, err := directory.Default.Applications(snap.ctx)
	if err != nil {
		return nil, err
	}
	views := make([]models.ApplicationView, 0, len(apps))
	basis, err := snap.Basis()
	if err != nil {
		return nil, fmt.Errorf("observation basis: %w", err)
	}

	for _, app := range apps {
		assignedCount := 0
		confirmedCount := 0
		observedCount := 0
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			var observed map[string]bool
			if basis.ReadAt != nil {
				if observed, err = snap.Observed(user.ID); err != nil {
					return nil, err
				}
				if hasObservedProject(observed, app.ProjectID) {
					observedCount++
				}
			}
			if hasProjectRole(roleMap, app.ProjectID) {
				assignedCount++
				// The overlap: given AND observed. Without it the app list had
				// to pass "confirmed: unknown" to holdersLine, which reads it
				// as zero — so an app whose holders all match showed "N
				// unexplained" and "N not in Zitadel yet" at once, about the
				// same N people.
				if observed != nil && hasObservedProjectRole(roleMap, observed, app.ProjectID) {
					confirmedCount++
				}
			}
		}

		consumedRoles, err := directory.Default.RoleKeysForProject(snap.ctx, app.ProjectID)
		if err != nil {
			return nil, err
		}

		view := models.ApplicationView{
			Application:       app,
			ConsumedRoles:     consumedRoles,
			AssignedUserCount: assignedCount,
			Observation:       basis,
		}
		if basis.ReadAt != nil {
			confirmed, observed := confirmedCount, observedCount
			view.ConfirmedUserCount = &confirmed
			view.ObservedUserCount = &observed
		}
		views = append(views, view)
	}

	return views, nil
}

func SimulateApplication(ctx context.Context, appID, userID string) (models.ApplicationSimulation, error) {
	app, ok, err := directory.Default.FindApplication(ctx, appID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	if !ok {
		return models.ApplicationSimulation{}, fmt.Errorf("application %q not found", appID)
	}

	user, ok, err := directory.Default.FindUser(ctx, userID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	if !ok {
		return models.ApplicationSimulation{}, fmt.Errorf("user %q not found", userID)
	}

	// Computed, not stored: the token path serves what the write paths put in
	// Redis, and a preview that wrote there would repair the one project being
	// looked at while the cache everywhere else stayed stale.
	facts, err := cache.CompileFacts(ctx, userID, app.ProjectID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	if facts.ProjectID == "" {
		facts.ProjectID = app.ProjectID
	}
	if facts.UserID == "" {
		facts.UserID = user.ID
	}

	// The token's roles are Zitadel's: here, its grants as last observed.
	rawRoles, _, err := ObservedRoles(ctx, userID, app.ProjectID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	facts.Roles = rawRoles
	// And its identity is Zitadel's, as the token's payload user carries it.
	facts = facts.Identify(identityOf(user))

	// The preview is the data plane. Same profile resolution, same shaper, same
	// facts — the only difference is that nothing is signed. Previously this
	// function invented an {iss, sub, aud, source, project} envelope that the
	// Actions v2 path never emitted, so the screen operators used to debug
	// "my app isn't seeing the roles it expects" was showing them a token that
	// did not exist.
	// A token now carries only its own app's claims (ADR-003), so every key is
	// this app's.
	e, err := resolveApp(ctx, appID)
	if err != nil {
		return models.ApplicationSimulation{}, err
	}
	payload := claims.Shape(e.profile, facts)
	owners := []models.ClaimKeyOwner{}
	for _, c := range e.profile.Claims {
		owners = append(owners, models.ClaimKeyOwner{Key: c.Path[0], OwnerLabel: app.Name, ApplicationID: appID, Kind: c.Kind, Source: c.Source})
	}

	return models.ApplicationSimulation{
		Application:  app,
		User:         user,
		RawRoles:     rawRoles,
		CustomClaims: payload,
		OwnedClaims:  topKeys(e.profile),
		ClaimOwners:  owners,
	}, nil
}

func ListProjects(ctx context.Context) ([]models.ProjectSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return listProjectsFromSnapshot(snap)
}

func listProjectsFromSnapshot(snap *accessSnapshot) ([]models.ProjectSummary, error) {
	projects, err := directory.Default.Projects(snap.ctx)
	if err != nil {
		return nil, err
	}
	rules, err := svcGetActiveMappingRules(snap.ctx)
	if err != nil {
		return nil, err
	}
	bundles, err := svcGetAllBundles(snap.ctx)
	if err != nil {
		return nil, err
	}

	// "Roles" here has to mean what it means on the project detail page: roles
	// that exist, not roles someone currently holds — the contradiction this
	// function used to render was "0 roles" beside a detail page listing 3.
	// This is the same union GlobalRoleCatalog builds (directory-declared,
	// Syndra-local, and referenced-only roles), assembled from two bulk
	// queries plus the project list already fetched above — not a second
	// derivation, and not a query per project.
	localRoles, err := svcDbGetAllLocalRoles(snap.ctx)
	if err != nil {
		return nil, err
	}
	referencedRoles, err := svcDbGetAllReferencedRoleKeys(snap.ctx)
	if err != nil {
		return nil, err
	}
	existingRoleKeys := make(map[string]map[string]bool, len(projects))
	addRoleKey := func(projectID, roleKey string) {
		set, ok := existingRoleKeys[projectID]
		if !ok {
			set = make(map[string]bool)
			existingRoleKeys[projectID] = set
		}
		set[roleKey] = true
	}
	for _, project := range projects {
		for _, role := range project.Roles {
			addRoleKey(project.ID, role.Key)
		}
	}
	for _, r := range localRoles {
		addRoleKey(r.ProjectID, r.RoleKey)
	}
	for _, ref := range referencedRoles {
		addRoleKey(ref[0], ref[1])
	}

	// One basis for every project in this response — the observation confirming
	// membership is an org-wide fact, not a per-project one, so it is fetched
	// once and carried on every row rather than re-asked per project.
	basis, err := snap.Basis()
	if err != nil {
		return nil, fmt.Errorf("observation basis: %w", err)
	}

	// What each bundle gives, includes and held versions too: a bundle made
	// only of includes gives a project roles as much as one listing them.
	given, err := svcGetBundleRolesGiven(snap.ctx)
	if err != nil {
		return nil, err
	}

	projectSummaries := make([]models.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		memberCount := 0
		confirmedMemberCount := 0
		observedMemberCount := 0
		sampleMembers := []string{}
		for _, user := range snap.Users() {
			roleMap, _, err := snap.For(user.ID)
			if err != nil {
				return nil, err
			}
			var observed map[string]bool
			if basis.ReadAt != nil {
				if observed, err = snap.Observed(user.ID); err != nil {
					return nil, err
				}
				if hasObservedProject(observed, project.ID) {
					observedMemberCount++
				}
			}
			if hasProjectRole(roleMap, project.ID) {
				memberCount++
				if len(sampleMembers) < 3 {
					sampleMembers = append(sampleMembers, user.Name)
				}
				if observed != nil && hasObservedProjectRole(roleMap, observed, project.ID) {
					confirmedMemberCount++
				}
			}
		}

		bundleCount := 0
		for _, bundle := range bundles {
			for _, role := range given[bundle.ID] {
				if role.ProjectID == project.ID {
					bundleCount++
					break
				}
			}
		}

		ruleInCount := 0
		ruleOutCount := 0
		for _, rule := range rules {
			if rule.TargetProject == project.ID {
				ruleInCount++
			}
			if rule.SourceProject == project.ID {
				ruleOutCount++
			}
		}

		roleKeys := make([]string, 0, len(existingRoleKeys[project.ID]))
		for roleKey := range existingRoleKeys[project.ID] {
			roleKeys = append(roleKeys, roleKey)
		}
		sort.Strings(roleKeys)

		summary := models.ProjectSummary{
			Project:       project,
			MemberCount:   memberCount,
			BundleCount:   bundleCount,
			RuleInCount:   ruleInCount,
			RuleOutCount:  ruleOutCount,
			RoleKeys:      roleKeys,
			SampleMembers: sampleMembers,
			Observation:   basis,
		}
		if basis.ReadAt != nil {
			confirmed, observed := confirmedMemberCount, observedMemberCount
			summary.ConfirmedMemberCount = &confirmed
			summary.ObservedMemberCount = &observed
		}
		projectSummaries = append(projectSummaries, summary)
	}

	textsort.ByName(projectSummaries, func(s models.ProjectSummary) string { return s.Project.Name }, func(s models.ProjectSummary) string { return s.Project.ID })

	return projectSummaries, nil
}

func BundleImpact(ctx context.Context, bundleID string) (models.BundleImpact, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.BundleImpact{}, err
	}
	return bundleImpactFromSnapshot(snap, bundleID)
}

func bundleImpactFromSnapshot(snap *accessSnapshot, bundleID string) (models.BundleImpact, error) {
	// What the bundle gives today: its latest published version, own roles
	// and included ones, the same set the bundle list counts. The working copy
	// alone counted a bundle made of includes as giving nothing.
	_, roles, err := svcLatestVersionRoles(snap.ctx, bundleID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return models.BundleImpact{}, err
	}

	impactedUsers := []models.UserProfile{}
	for _, user := range snap.Users() {
		bundles, err := svcGetBundlesForUser(snap.ctx, user.ID)
		if err != nil {
			return models.BundleImpact{}, err
		}
		for _, bundle := range bundles {
			if bundle.ID == bundleID {
				impactedUsers = append(impactedUsers, user)
				break
			}
		}
	}

	return models.BundleImpact{
		BundleID:  bundleID,
		RoleCount: len(roles),
		Users:     impactedUsers,
	}, nil
}

func UserDirectGrants(ctx context.Context, userID string) ([]models.DirectGrant, error) {
	return svcGetDirectGrantsForUser(ctx, userID, true)
}

// AllDirectGrants returns every active Syndra-direct grant. Active means
// expires_at is NULL or in the future — expired rows are excluded so the
// reconciliation handler compares like-for-like with what Zitadel currently
// surfaces. Backs GET /api/v1/reconciliation/grants.
func AllDirectGrants(ctx context.Context) ([]models.DirectGrant, error) {
	return svcGetAllDirectGrants(ctx, false)
}

func Governance(ctx context.Context) (models.GovernanceSummary, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	return governanceFromSnapshot(snap)
}

func governanceFromSnapshot(snap *accessSnapshot) (models.GovernanceSummary, error) {
	requests, err := svcGetAccessRequests(snap.ctx, "pending")
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	expiring, err := svcGetExpiringDirectGrants(snap.ctx, expiryHorizon)
	if err != nil {
		return models.GovernanceSummary{}, err
	}

	cleanupHints := []string{}
	bundles, err := svcGetAllBundles(snap.ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	// One grouped count for every bundle: only "has any holder" is needed here,
	// so resolving each user's bundles (bundles x users queries) was pure cost.
	holders, err := svcGetBundleHolderCounts(snap.ctx)
	if err != nil {
		return models.GovernanceSummary{}, err
	}
	for _, bundle := range bundles {
		if holders[bundle.ID] == 0 {
			cleanupHints = append(cleanupHints, fmt.Sprintf("Bundle %q is unused and can be reviewed for cleanup.", bundle.Name))
		}
	}
	if len(requests) == 0 {
		cleanupHints = append(cleanupHints, "No pending requests right now, so approvals are caught up.")
	}

	if requests == nil {
		requests = []models.AccessRequest{}
	}
	if expiring == nil {
		expiring = []models.DirectGrant{}
	}

	// Outbox depth + reachability so the UI can render the amber "N changes
	// awaiting Zitadel" callout and gate "Resume now". A count error is
	// non-fatal — the rest of the summary is still useful, so degrade to 0.
	pendingCount, err := svcCountPendingPropagations(snap.ctx)
	if err != nil {
		pendingCount = 0
	}

	// Drift pending-triage count + top-N preview for the dashboard callout. A
	// count/lookup error is non-fatal — the rest of the summary is still useful,
	// so degrade to 0/empty (mirrors the pending-propagation degrade above).
	driftCount, err := svcCountPendingDrift(snap.ctx)
	if err != nil {
		driftCount = 0
	}
	topDrift, err := svcGetTopDrift(snap.ctx, 3)
	if err != nil {
		topDrift = nil
	}
	if topDrift == nil {
		topDrift = []models.DriftItem{}
	}

	// The targets Syndra cannot vouch for. Non-fatal like the two counts above,
	// and for the same reason — but note what degrading to empty means here: it
	// reports "nothing is unreadable" when the read itself failed. That is the
	// same false quiet this field exists to break, so the failure is logged
	// rather than swallowed silently.
	unreconciled, err := svcGetUnreconciledTargets(snap.ctx)
	if err != nil {
		log.Printf("[GOVERNANCE] could not read unreconciled targets: %v (degrading to none)", err)
		unreconciled = nil
	}
	if unreconciled == nil {
		unreconciled = []models.UnreconciledTarget{}
	}

	// Accepted writes no read has since observed, old enough to be a finding.
	// Non-fatal like the drift and merge-finding reads above — one unreadable
	// signal on this page must not blank the rest of it.
	unconfirmedWrites, err := svcGetUnconfirmedWrites(snap.ctx)
	if err != nil {
		log.Printf("[GOVERNANCE] could not read unconfirmed writes: %v (degrading to none)", err)
		unconfirmedWrites = models.UnconfirmedWriteSummary{}
	}
	if unconfirmedWrites.Top == nil {
		unconfirmedWrites.Top = []models.UnconfirmedWrite{}
	}

	return models.GovernanceSummary{
		PendingRequests: requests,
		ExpiringGrants:  expiring,
		CleanupHints:    cleanupHints,
		PendingPropagation: models.PendingPropagationSummary{
			Count:            pendingCount,
			ZitadelReachable: svcZitadelReachable(snap.ctx),
		},
		Drift:               models.DriftSummary{Count: driftCount, Top: topDrift},
		UnreconciledTargets: unreconciled,
		MergeFindings:       svcCountMergeFindings(snap.ctx),
		UnconfirmedWrites:   unconfirmedWrites,
	}, nil
}

func Topology(ctx context.Context) (models.TopologyGraph, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.TopologyGraph{
			Nodes: []models.TopologyNode{},
			Edges: []models.TopologyEdge{},
		}, err
	}
	return topologyFromSnapshot(snap)
}

func topologyFromSnapshot(snap *accessSnapshot) (models.TopologyGraph, error) {
	ctx := snap.ctx
	graph := models.TopologyGraph{
		Nodes: []models.TopologyNode{},
		Edges: []models.TopologyEdge{},
	}
	nodeSeen := make(map[string]bool)
	edgeSeen := make(map[string]bool)
	projectCatalog := make(map[string]models.ProjectCatalog)
	roleCatalog := make(map[string]map[string]models.ProjectRole)

	addNode := func(node models.TopologyNode) {
		if nodeSeen[node.ID] {
			return
		}
		nodeSeen[node.ID] = true
		graph.Nodes = append(graph.Nodes, node)
	}
	addEdge := func(edge models.TopologyEdge) {
		if edgeSeen[edge.ID] {
			return
		}
		edgeSeen[edge.ID] = true
		graph.Edges = append(graph.Edges, edge)
	}
	ensureProjectNode := func(projectID string) {
		projectNodeID := "project:" + projectID
		project, ok := projectCatalog[projectID]
		if ok {
			addNode(models.TopologyNode{
				ID:          projectNodeID,
				Label:       project.Name,
				Kind:        "project",
				ProjectID:   project.ID,
				Description: project.Description,
				Meta: map[string]string{
					"kind": project.Kind,
				},
			})
			return
		}

		addNode(models.TopologyNode{
			ID:          projectNodeID,
			Label:       projectID,
			Kind:        "project",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or bundle grants that do not exist in the seeded demo catalog yet.",
			Meta: map[string]string{
				"kind":   "external",
				"source": "database",
			},
		})
	}
	ensureRoleNode := func(projectID, roleKey string) {
		ensureProjectNode(projectID)

		roleNode := models.TopologyNode{
			ID:          roleNodeID(projectID, roleKey),
			Label:       roleLabel(roleKey),
			Kind:        "role",
			ProjectID:   projectID,
			Description: "Referenced by persisted rules or grants outside the seeded role catalog.",
			Meta: map[string]string{
				"role_key": roleKey,
				"source":   "database",
			},
		}

		if roles, ok := roleCatalog[projectID]; ok {
			if role, ok := roles[roleKey]; ok {
				roleNode.Label = role.Label
				roleNode.Description = role.Description
				roleNode.Meta = map[string]string{
					"role_key": role.Key,
				}
			}
		}

		addNode(roleNode)
		addEdge(models.TopologyEdge{
			ID:     "contains:" + projectID + ":" + roleKey,
			Source: "project:" + projectID,
			Target: roleNode.ID,
			Kind:   "contains",
			Label:  "defines",
		})
	}

	dirProjects, err := directory.Default.Projects(ctx)
	if err != nil {
		return graph, err
	}
	for _, project := range dirProjects {
		projectCatalog[project.ID] = project
		roleCatalog[project.ID] = make(map[string]models.ProjectRole, len(project.Roles))
		ensureProjectNode(project.ID)

		for _, role := range project.Roles {
			roleCatalog[project.ID][role.Key] = role
			ensureRoleNode(project.ID, role.Key)
		}
	}

	appViews, err := listApplicationsFromSnapshot(snap)
	if err != nil {
		return graph, err
	}
	for _, app := range appViews {
		nodeID := "application:" + app.Application.ID
		ensureProjectNode(app.Application.ProjectID)
		addNode(models.TopologyNode{
			ID:          nodeID,
			Label:       app.Application.Name,
			Kind:        "application",
			ProjectID:   app.Application.ProjectID,
			Description: app.Application.Description,
			Meta: map[string]string{
				"claim_name":  app.Application.ClaimName,
				"format_type": app.Application.FormatType,
				"consumer":    app.Application.Consumer,
			},
		})
		addEdge(models.TopologyEdge{
			ID:     "consumes:" + app.Application.ID,
			Source: nodeID,
			Target: "project:" + app.Application.ProjectID,
			Kind:   "application",
			Label:  "consumes",
		})
	}

	bundles, err := svcGetAllBundles(ctx)
	if err != nil {
		return graph, err
	}
	given, err := svcGetBundleRolesGiven(ctx)
	if err != nil {
		return graph, err
	}
	for _, bundle := range bundles {
		bundleID := "bundle:" + bundle.ID
		addNode(models.TopologyNode{
			ID:          bundleID,
			Label:       bundle.Name,
			Kind:        "bundle",
			Description: bundle.Description,
		})

		for _, role := range given[bundle.ID] {
			ensureRoleNode(role.ProjectID, role.RoleKey)
			addEdge(models.TopologyEdge{
				ID:     "bundle-role:" + bundle.ID + ":" + role.ProjectID + ":" + role.RoleKey,
				Source: bundleID,
				Target: roleNodeID(role.ProjectID, role.RoleKey),
				Kind:   "bundle",
				Label:  "grants",
			})
		}
	}

	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return graph, err
	}
	for _, rule := range rules {
		ensureRoleNode(rule.SourceProject, rule.SourceRole)
		ensureRoleNode(rule.TargetProject, rule.TargetRole)
		addEdge(models.TopologyEdge{
			ID:     "rule:" + rule.ID,
			Source: roleNodeID(rule.SourceProject, rule.SourceRole),
			Target: roleNodeID(rule.TargetProject, rule.TargetRole),
			Kind:   "rule",
			Label:  "maps",
		})
	}

	sort.Slice(graph.Nodes, func(i, j int) bool {
		if graph.Nodes[i].Kind == graph.Nodes[j].Kind {
			if graph.Nodes[i].Label == graph.Nodes[j].Label {
				return graph.Nodes[i].ID < graph.Nodes[j].ID
			}
			return graph.Nodes[i].Label < graph.Nodes[j].Label
		}
		return graph.Nodes[i].Kind < graph.Nodes[j].Kind
	})
	sort.Slice(graph.Edges, func(i, j int) bool {
		return graph.Edges[i].ID < graph.Edges[j].ID
	})

	return graph, nil
}

func collectUserRoles(ctx context.Context, userID string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
	roleMap := make(map[roleKey]*models.EffectiveRole)
	// Each base role with when it ends (zero: never), for access.Ends.
	base := map[roleKey]time.Time{}
	hold := func(k roleKey, end time.Time) {
		if cur, ok := base[k]; ok {
			end = access.Later(cur, end)
		}
		base[k] = end
	}

	directGrants, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, nil, err
	}
	for _, grant := range directGrants {
		key := roleKey{projectID: grant.ProjectID, roleKey: grant.RoleKey}
		var end time.Time
		if grant.ExpiresAt != nil {
			end = *grant.ExpiresAt
		}
		hold(key, end)
		description := "Direct Zitadel grant"
		if grant.Reason != "" {
			description = grant.Reason
		}
		upsertRole(ctx, roleMap, key, true, models.RoleReason{
			Kind:        "direct",
			Description: description,
		})
	}

	bundles, err := svcGetBundlesForUser(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// Resolved through each person's pinned version, not the bundle's working
	// copy: somebody left on v2 holds v2's roles, and reading the working copy
	// here would show them access they do not have.
	byBundle, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	// Included bundles are named once per person, not once per role.
	viaNames := map[string]string{}
	viaName := func(id string) string {
		if name, ok := viaNames[id]; ok {
			return name
		}
		name := id
		if b, err := svcGetBundleByID(ctx, id); err == nil {
			name = b.Name
		}
		viaNames[id] = name
		return name
	}
	for _, bundle := range bundles {
		roles := byBundle[bundle.ID]
		for _, role := range roles {
			key := roleKey{projectID: role.ProjectID, roleKey: role.RoleKey}
			hold(key, time.Time{})
			reason := models.RoleReason{
				Kind:        "bundle",
				Description: fmt.Sprintf("Given by bundle %s v%d", bundle.Name, bundle.PinnedVersion),
				BundleID:    bundle.ID,
				BundleName:  bundle.Name,
			}
			if role.ViaBundleID != "" {
				reason.ViaBundleID, reason.ViaBundleName = role.ViaBundleID, viaName(role.ViaBundleID)
				reason.Description = fmt.Sprintf("Via %s, from %s", bundle.Name, reason.ViaBundleName)
			}
			upsertRole(ctx, roleMap, key, true, reason)
		}
	}

	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return nil, nil, err
	}

	// What the chains reach, when each role ends, and which rules fire, is
	// the access core's to say (the one implementation, which the token
	// compiler also calls); this only names, for each role reached, every rule
	// that reaches it.
	dated := access.Ends(base, rules, newRoleKey)
	closure := make(map[roleKey]bool, len(dated))
	for k := range dated {
		closure[k] = true
	}
	for _, rule := range access.Fired(closure, rules, newRoleKey) {
		upsertRole(ctx, roleMap, roleKey{projectID: rule.TargetProject, roleKey: rule.TargetRole}, false, models.RoleReason{
			Kind:           "mapping",
			Description:    fmt.Sprintf("Derived from %s:%s", rule.SourceProject, rule.SourceRole),
			TriggerProject: rule.SourceProject,
			TriggerRole:    rule.SourceRole,
		})
	}
	for k, r := range roleMap {
		if end := dated[k]; !end.IsZero() {
			r.Ends = &end
		}
	}

	return roleMap, bundles, nil
}

// markQueuedDeliveries flags the sources whose grant has not been sent yet.
//
// Every table read above answers "what has this person been given", and none
// of them answers "has it been delivered". Those are different questions
// between the moment a manual-mode change is recorded and the moment somebody
// confirms it under Pending changes — a window of minutes or days — and for
// that window the page claimed the second while knowing only the first.
//
// Matched per SOURCE rather than per role, because a role can be held twice:
// delivered by a direct grant and queued by a bundle, in which case the person
// has it and one of the two reasons is still owed. A per-role flag would have
// to pick one of those to be wrong about.
//
// A rule-derived reason carries no rule id — the derivation names the trigger,
// not the rule row — so those match on source alone. That is exact whenever a
// role is derived by one rule, and the failure mode of the rest is a marker
// that appears while a sibling derivation is queued, which overstates the wait
// rather than the delivery.
func markQueuedDeliveries(ctx context.Context, userID string, roleMap map[roleKey]*models.EffectiveRole, giving func(projectID, roleKey string) bool) error {
	pending, err := svcPendingDeliveries(ctx, userID)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	type deliveryKey struct{ projectID, roleKey, source, sourceRef string }
	// The earliest queue time wins: "Given 2h ago" dates the oldest unsent change.
	queued := make(map[deliveryKey]time.Time, len(pending))
	for _, d := range pending {
		for _, k := range []deliveryKey{
			{d.ProjectID, d.RoleKey, d.Source, d.SourceRef},
			{d.ProjectID, d.RoleKey, d.Source, ""},
		} {
			if at, ok := queued[k]; !ok || d.QueuedAt.Before(at) {
				queued[k] = d.QueuedAt
			}
		}
	}

	for key, role := range roleMap {
		if role == nil || !giving(key.projectID, key.roleKey) {
			continue
		}
		for i, reason := range role.Reasons {
			var source, ref string
			switch reason.Kind {
			case "direct":
				source = "direct"
			case "bundle":
				source, ref = "bundle", reason.BundleID
			case "mapping":
				source = "rule"
			default:
				continue
			}
			if at, ok := queued[deliveryKey{key.projectID, key.roleKey, source, ref}]; ok {
				role.Reasons[i].Queued = true
				if !at.IsZero() {
					at := at
					role.Reasons[i].QueuedAt = &at
				}
			}
		}
	}
	return nil
}

func upsertRole(ctx context.Context, roleMap map[roleKey]*models.EffectiveRole, key roleKey, isSource bool, reason models.RoleReason) bool {
	current := roleMap[key]
	if current == nil {
		// Best-effort project name — never block lineage resolution on a
		// directory lookup failure. Fall back to the raw project ID.
		//
		// directory.Default.ProjectName itself returns the id with a nil
		// error on an unresolved lookup (deliberately, so audit rows and
		// topology placeholders never collapse to empty) — so a real error
		// and a silent miss both surface here the same way: the returned
		// name equals the id we asked about. That equality IS the "did this
		// resolve" signal, since a real project name colliding with its own
		// platform id is not a case this system needs to handle.
		projectName := key.projectID
		if name, err := directory.Default.ProjectName(ctx, key.projectID); err == nil {
			projectName = name
		}
		current = &models.EffectiveRole{
			ProjectID:           key.projectID,
			ProjectName:         projectName,
			ProjectNameResolved: projectName != key.projectID,
			RoleKey:             key.roleKey,
			IsSource:            isSource,
		}
		roleMap[key] = current
	}
	if isSource {
		current.IsSource = true
	}

	for _, existing := range current.Reasons {
		if existing.Kind == reason.Kind &&
			existing.Description == reason.Description &&
			existing.BundleID == reason.BundleID &&
			existing.TriggerProject == reason.TriggerProject &&
			existing.TriggerRole == reason.TriggerRole {
			return false
		}
	}

	current.Reasons = append(current.Reasons, reason)
	return true
}

func hasProjectRole(roleMap map[roleKey]*models.EffectiveRole, projectID string) bool {
	for _, role := range roleMap {
		if role.ProjectID == projectID {
			return true
		}
	}
	return false
}

// hasObservedProjectRole is hasProjectRole's confirmed twin: true only when a
// role this person is recorded as holding in projectID is also present in
// what the observation store confirmed for them.
func hasObservedProjectRole(roleMap map[roleKey]*models.EffectiveRole, observed map[string]bool, projectID string) bool {
	for key, role := range roleMap {
		if role.ProjectID == projectID && observed[key.projectID+":"+key.roleKey] {
			return true
		}
	}
	return false
}

func matchesUser(user models.UserProfile, query string) bool {
	return strings.Contains(strings.ToLower(user.ID), query) ||
		strings.Contains(strings.ToLower(user.Name), query) ||
		strings.Contains(strings.ToLower(user.Email), query) ||
		strings.Contains(strings.ToLower(user.Team), query)
}

func ensureBundles(bundles []models.Bundle) []models.Bundle {
	if bundles == nil {
		return []models.Bundle{}
	}
	return bundles
}

func roleNodeID(projectID, roleKey string) string {
	return "role:" + projectID + ":" + roleKey
}

func roleLabel(roleKey string) string {
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(roleKey))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	if len(words) == 0 {
		return roleKey
	}
	return strings.Join(words, " ")
}
