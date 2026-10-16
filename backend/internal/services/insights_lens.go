package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Insights lenses: the same observed holders, looked at through one question —
// whose access is about to end, or what does this group hold. Read-only.

const (
	lensDefaultDays = 30
	lensMaxDays     = 365
	lensMaxTeamLen  = 100
)

// ErrInvalidLens is a lens question that is not well formed. The text is plain
// language, meant to be shown as it is.
var ErrInvalidLens = errors.New("invalid lens")

func lensInvalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalidLens, msg) }

// LensGroupKind is which group a group lens looks at.
type LensGroupKind string

const (
	LensLeft    LensGroupKind = "left"
	LensNewTerm LensGroupKind = "new_term"
	LensTeam    LensGroupKind = "team"
)

// LensGroup is a parsed ?group= value. Team is set only for LensTeam.
type LensGroup struct {
	Kind LensGroupKind
	Team string
}

// String is the value the group was asked for by.
func (g LensGroup) String() string {
	if g.Kind == LensTeam {
		return "team:" + g.Team
	}
	return string(g.Kind)
}

// ParseLensDays defaults to 30 and clamps to 1..365; only something that is not
// a whole number is an error.
func ParseLensDays(s string) (int, error) {
	if s == "" {
		return lensDefaultDays, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, lensInvalid("days must be a whole number")
	}
	return min(max(n, 1), lensMaxDays), nil
}

// ParseLensGroup accepts left, new_term or team:<name>. A team name is trimmed,
// non-empty, short and free of control characters.
func ParseLensGroup(s string) (LensGroup, error) {
	switch s {
	case "left":
		return LensGroup{Kind: LensLeft}, nil
	case "new_term":
		return LensGroup{Kind: LensNewTerm}, nil
	}
	if name, ok := strings.CutPrefix(s, "team:"); ok {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > lensMaxTeamLen || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return LensGroup{}, lensInvalid("name a team after \"team:\", in 100 characters or fewer")
		}
		return LensGroup{Kind: LensTeam, Team: name}, nil
	}
	return LensGroup{}, lensInvalid("group must be left, new_term or team:<name>")
}

// lensPerson is one person as a lens sees them. Observed is what Zitadel shows
// them holding ("project:role"); Indirect is the subset of those Syndra can
// also explain by something other than a direct grant, so ending a direct
// grant would not end the access.
type lensPerson struct {
	ID, Status, Team string
	// Joined is when Zitadel created the account; nil when it gave no date.
	Joined   *time.Time
	Observed map[string]bool
	Indirect map[string]bool
}

// lensWorld is the one read a lens works from. Observed is false when nothing
// has ever been read from Zitadel, in which case no person carries holdings.
type lensWorld struct {
	People   []lensPerson
	Observed bool
}

// The reads, as seams, so the lenses need no database.
var (
	lensNow       = time.Now
	lensGrants    = db.GrantsWithExpiry
	lensFirstSeen = db.UsersFirstSeen
	lensProjects  = func(ctx context.Context) ([]models.ProjectCatalog, error) { return directory.Default.Projects(ctx) }
	lensRead      = readLensWorld
)

func readLensWorld(ctx context.Context, withRoutes bool) (lensWorld, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return lensWorld{}, err
	}
	basis, err := snap.Basis()
	if err != nil {
		return lensWorld{}, err
	}
	w := lensWorld{Observed: basis.ReadAt != nil}
	for _, u := range snap.Users() {
		p := lensPerson{ID: u.ID, Status: u.Status, Team: u.Team, Joined: u.Joined}
		if w.Observed {
			if p.Observed, err = snap.Observed(u.ID); err != nil {
				return lensWorld{}, err
			}
			if withRoutes {
				roleMap, _, err := snap.For(u.ID)
				if err != nil {
					return lensWorld{}, err
				}
				p.Indirect = map[string]bool{}
				for k, role := range roleMap {
					for _, r := range role.Reasons {
						if r.Kind != "direct" {
							p.Indirect[k.projectID+":"+k.roleKey] = true
						}
					}
				}
			}
		}
		w.People = append(w.People, p)
	}
	return w, nil
}

// lensNamer turns "project:role" into names. A project or role the directory
// does not list falls back to its key, as the trends read does.
func lensNamer(projects []models.ProjectCatalog) func(project, role string) (string, string) {
	pn := map[string]string{}
	rl := map[string]string{}
	for _, p := range projects {
		pn[p.ID] = p.Name
		for _, r := range p.Roles {
			rl[p.ID+":"+r.Key] = r.Label
		}
	}
	return func(project, role string) (string, string) {
		pName, rLabel := pn[project], rl[project+":"+role]
		if pName == "" {
			pName = project
		}
		if rLabel == "" {
			rLabel = role
		}
		return pName, rLabel
	}
}

func sortLensRoles(rs []models.LensRole) {
	sort.Slice(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		switch {
		case a.People != b.People:
			return a.People > b.People
		case a.ProjectName != b.ProjectName:
			return a.ProjectName < b.ProjectName
		case a.RoleLabel != b.RoleLabel:
			return a.RoleLabel < b.RoleLabel
		case a.ProjectID != b.ProjectID:
			return a.ProjectID < b.ProjectID
		}
		return a.RoleKey < b.RoleKey
	})
}

// tallyEnding counts, per role, the people whose direct grant ends after `now`
// and no later than `until` (both edges: a grant ending exactly at `until` is
// in; one that has already ended, or never ends, is out), who are observed
// holding the role and would not still hold it some other way.
func tallyEnding(grants []db.ExpiringGrant, people []lensPerson, now, until time.Time, name func(string, string) (string, string)) []models.LensRole {
	by := make(map[string]lensPerson, len(people))
	for _, p := range people {
		by[p.ID] = p
	}
	type key struct{ project, role string }
	type tally struct {
		people map[string]bool
		first  time.Time
	}
	tallies := map[key]*tally{}
	for _, g := range grants {
		if !g.ExpiresAt.After(now) || g.ExpiresAt.After(until) {
			continue
		}
		k := g.ProjectID + ":" + g.RoleKey
		p, ok := by[g.UserID]
		if !ok || !p.Observed[k] || p.Indirect[k] {
			continue
		}
		t := tallies[key{g.ProjectID, g.RoleKey}]
		if t == nil {
			t = &tally{people: map[string]bool{}, first: g.ExpiresAt}
			tallies[key{g.ProjectID, g.RoleKey}] = t
		}
		t.people[g.UserID] = true
		if g.ExpiresAt.Before(t.first) {
			t.first = g.ExpiresAt
		}
	}
	out := []models.LensRole{}
	for k, t := range tallies {
		pn, rl := name(k.project, k.role)
		first := t.first
		out = append(out, models.LensRole{ProjectID: k.project, ProjectName: pn, RoleKey: k.role, RoleLabel: rl, People: len(t.people), EarliestEnd: &first})
	}
	sortLensRoles(out)
	return out
}

// tallyGroup counts, per role, how many of the group are observed holding it.
func tallyGroup(group []lensPerson, name func(string, string) (string, string)) []models.LensRole {
	counts := map[string]int{}
	for _, p := range group {
		for k := range p.Observed {
			counts[k]++
		}
	}
	out := []models.LensRole{}
	for k, n := range counts {
		project, role, ok := strings.Cut(k, ":")
		if !ok {
			continue
		}
		pn, rl := name(project, role)
		out = append(out, models.LensRole{ProjectID: project, ProjectName: pn, RoleKey: role, RoleLabel: rl, People: n})
	}
	sortLensRoles(out)
	return out
}

// pickGroup selects the people a group means. Joined, firstSeen and termStart
// matter only to new_term: somebody with no Zitadel creation date and nothing
// ever recorded has no first time, so is not new. A person's status is the directory's own word.
//
// Statuses that exist (directory.normalizeUserState): active, inactive,
// initial, locked, deleted, unspecified. "left" is leftStatuses: inactive,
// locked, deleted, plus "departed", the word the drift queue uses.
func pickGroup(g LensGroup, people []lensPerson, firstSeen map[string]time.Time, termStart time.Time) []lensPerson {
	var out []lensPerson
	for _, p := range people {
		var in bool
		switch g.Kind {
		case LensLeft:
			in = leftStatuses[strings.ToLower(p.Status)]
		case LensNewTerm:
			// Zitadel's creation date wins: holding and people are observed
			// from Zitadel, records only explain. First seen is a fallback for
			// an account with no date, and a poor one: rows backfilled at
			// deploy or first sync make everyone look new.
			at, ok := firstSeen[p.ID]
			if p.Joined != nil {
				at, ok = *p.Joined, true
			}
			in = ok && !at.Before(termStart)
		case LensTeam:
			in = strings.EqualFold(strings.TrimSpace(p.Team), g.Team)
		}
		if in {
			out = append(out, p)
		}
	}
	return out
}

// InsightsLensEnding answers kind=ending for the next `days` days from now.
func InsightsLensEnding(ctx context.Context, days int) (models.InsightsLensEnding, error) {
	days = min(max(days, 1), lensMaxDays)
	now := lensNow()
	grants, err := lensGrants(ctx, now)
	if err != nil {
		return models.InsightsLensEnding{}, err
	}
	projects, err := lensProjects(ctx)
	if err != nil {
		return models.InsightsLensEnding{}, err
	}
	world, err := lensRead(ctx, true)
	if err != nil {
		return models.InsightsLensEnding{}, err
	}
	out := models.InsightsLensEnding{Kind: "ending", WindowDays: days, HoldersObserved: world.Observed, Roles: []models.LensRole{}}
	if world.Observed {
		out.Roles = tallyEnding(grants, world.People, now, now.AddDate(0, 0, days), lensNamer(projects))
	}
	return out, nil
}

// InsightsLensGroup answers kind=group. For "left" the roles are the access
// that should probably be revoked.
func InsightsLensGroup(ctx context.Context, g LensGroup) (models.InsightsLensGroup, error) {
	now := lensNow()
	projects, err := lensProjects(ctx)
	if err != nil {
		return models.InsightsLensGroup{}, err
	}
	world, err := lensRead(ctx, false)
	if err != nil {
		return models.InsightsLensGroup{}, err
	}
	var seen map[string]time.Time
	if g.Kind == LensNewTerm {
		if seen, err = lensFirstSeen(ctx); err != nil {
			return models.InsightsLensGroup{}, err
		}
	}
	members := pickGroup(g, world.People, seen, windowFor(TrendTerm, now).From)
	return models.InsightsLensGroup{
		Kind: "group", Group: g.String(), Size: len(members), HoldersObserved: world.Observed,
		Roles: tallyGroup(members, lensNamer(projects)),
	}, nil
}
