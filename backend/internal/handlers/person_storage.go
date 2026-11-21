package handlers

import (
	"net/http"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/handlers/kinds"
	"syndra/internal/services"
)

// A person's network storage, for operators (A-10 Part 1): one read-only view
// per registered system. It is the member's own view of the same system
// (describeMyTarget, the same code), plus what only an operator reads: how the
// account came to be theirs, the home folder limit and where it comes from,
// the holds in force, why a system isn't ready yet, and what an account still
// to be made will open. Syndra keeps no copy of a read (design §13.3): with the
// add-on down the system's read is none, and the page says "not read".
//
// Nothing here writes. Every value set elsewhere is set on the system's page.

type personTargetView struct {
	myTargetView
	// Adopted is an account made by hand on the target before Syndra managed
	// it, assigned to this person by an operator. Nil for an account Syndra
	// made.
	Adopted *personAdoption `json:"adopted,omitempty"`
	// Home is the /targets/{t}/homes row for this account, with usage from the
	// target's own answer about it.
	Home *accountHome `json:"home,omitempty"`
	// Holds is every deny in force for this person on this system.
	Holds []personHold `json:"holds"`
	// Setup says why the system can't reach anybody yet: "no_project" or
	// "no_mapping". Empty once a mapped role can reach it.
	Setup string `json:"setup,omitempty"`
	// Preview is what an entitled person with no account yet will open, from
	// the groups their roles give, as the target last read its folders.
	Preview []addons.Place `json:"preview,omitempty"`
}

type personAdoption struct {
	ActorID string    `json:"actor_id"`
	At      time.Time `json:"at"`
}

type personHold struct {
	Field      string     `json:"field"`
	Value      string     `json:"value"`
	Reason     string     `json:"reason"`
	ActorID    string     `json:"actor_id"`
	Since      time.Time  `json:"since"`
	ReviewDate *time.Time `json:"review_date,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// adoptionWindow is how close an account.adopt must sit before the binding it
// wrote for it to be that binding's origin. A binding made later (released,
// then made by a drain) is not the adopted one.
const adoptionWindow = 10 * time.Minute

// handleUserTargets is GET /api/v1/users/{id}/targets (operators only).
func handleUserTargets(w http.ResponseWriter, r *http.Request) {
	subject := r.PathValue("id")
	if !trimmedNonEmpty(subject) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	projects, err := dbTargetProjects(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}
	registrations := addonsRegistered()
	views := make([]personTargetView, 0, len(registrations))
	for _, reg := range registrations {
		view, err := describePersonTarget(r, reg.Target, subject, projects[reg.Target] != "")
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
			return
		}
		views = append(views, view)
	}
	jsonResponse(w, http.StatusOK, map[string]any{"targets": views})
}

func describePersonTarget(r *http.Request, target, subject string, hasProject bool) (personTargetView, error) {
	mine, set, err := describeTargetFor(r, target, subject)
	if err != nil {
		return personTargetView{}, err
	}
	mine.Usage = personalUsage(r.Context(), target, subject, false, mine.Storage)
	// The member's own line about a password change nobody confirmed is theirs
	// to settle, by setting it again; passwords never reach an operator (A-13 §5).
	mine.PasswordChange = nil
	view := personTargetView{myTargetView: mine, Holds: []personHold{}}
	now := time.Now()

	if !hasProject {
		view.Setup = "no_project"
	} else if mappings, err := dbListRoleMappings(r.Context(), target); err == nil && len(mappings) == 0 {
		view.Setup = "no_mapping"
	}

	if inForce, err := dbAllowancesInForce(r.Context(), subject, target); err == nil {
		for _, a := range inForce {
			if a.Direction == db.AllowanceDeny {
				view.Holds = append(view.Holds, personHold{Field: a.Field, Value: a.Value, Reason: a.Reason,
					ActorID: a.ActorID, Since: a.CreatedAt, ReviewDate: a.ReviewDate, ExpiresAt: a.ExpiresAt})
			}
		}
	}

	// The home folder's limit field, by role and unit (kinds.HomeLimit), from
	// the schema the view already carries: the saved one (acceptedShape).
	limit, _ := kinds.HomeLimit(view.EntitlementSchema)
	if view.Account != nil {
		home := accountHome{SubjectID: subject, Username: view.Account.Username}
		roleLimits(&home, set, limit)
		home.Own = ownHomeLimit(r, subject, target, limit.Name, now)
		if storage := view.Storage; storage != nil {
			for _, p := range storage.Places {
				if p.Home && p.Usage != nil {
					used, quota := p.Usage.UsedBytes, p.Usage.QuotaBytes
					home.Home, home.UsedBytes, home.QuotaBytes = p.Path, &used, &quota
				}
			}
		}
		ownEndedIfOver(r, &home, target, limit.Name, now)
		view.Home = &home
		view.Adopted = adoptionOf(r, target, subject, view.Account.BoundAt)
	} else if view.Entitled {
		// What they'll get, for the preview: the limit their roles give and
		// the folders their groups open. No use: there is no folder yet.
		home := accountHome{SubjectID: subject}
		roleLimits(&home, set, limit)
		view.Home = &home
		view.Preview = previewPlaces(r, target, set)
	}
	return view, nil
}

// adoptionOf is the account.adopt that bound this account, when it was one.
func adoptionOf(r *http.Request, target, subject string, boundAt *time.Time) *personAdoption {
	if boundAt == nil {
		return nil
	}
	op, found, err := dbLastSucceededAddonOperation(r.Context(), target, subject, "account.adopt")
	if err != nil || !found {
		return nil
	}
	at := op.CreatedAt
	if op.SettledAt != nil {
		at = *op.SettledAt
	}
	if boundAt.Sub(op.CreatedAt) > adoptionWindow || op.CreatedAt.After(boundAt.Add(adoptionWindow)) {
		return nil
	}
	return &personAdoption{ActorID: op.ActorID, At: at}
}

// previewPlaces is what the groups this person's roles give open on the target,
// with the roles behind each, as the target last read its folders. Nil when
// that hasn't been read.
func previewPlaces(r *http.Request, target string, set services.EntitlementSet) []addons.Place {
	reach := addonsReadReach(r.Context(), target, wantsFresh(r))
	if reach.Read.None() {
		return nil
	}
	byPath := map[string]int{}
	var out []addons.Place
	add := func(p addons.Place, roles []services.RoleSource) {
		i, seen := byPath[p.Path]
		if !seen {
			i = len(out)
			byPath[p.Path] = i
			p.Roles = nil
			out = append(out, p)
		} else if p.Access == "change" {
			out[i].Access = "change"
		}
		for _, role := range roles {
			pr := addons.PlaceRole{ProjectID: role.ProjectID, RoleKey: role.RoleKey}
			dup := false
			for _, have := range out[i].Roles {
				dup = dup || have == pr
			}
			if !dup {
				out[i].Roles = append(out[i].Roles, pr)
			}
		}
	}
	for _, p := range reach.Everyone {
		add(p, nil)
	}
	membership := kinds.Storage(set.Roles).Membership
	for _, g := range set.Fields[membership] {
		for _, p := range reach.Groups[g] {
			add(p, set.Given[membership+"\x00"+g])
		}
	}
	// The home folder is the Home folder row's, never a folder here.
	kept := out[:0]
	for _, p := range out {
		if !p.Home && strings.TrimSpace(p.Path) != "" {
			kept = append(kept, p)
		}
	}
	return kept
}
