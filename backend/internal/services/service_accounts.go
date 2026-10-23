package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"syndra/internal/db"
	"syndra/internal/models"
)

// svcListServiceAccountRows is Syndra's own record of service accounts: the
// operator's mark on a human-type account, and the kind, purpose and owner of
// any account. A seam so no test needs the pool.
var svcListServiceAccountRows = db.ListServiceAccounts

// serviceAccountMarks is the stored rows by user id.
func serviceAccountMarks(ctx context.Context) (map[string]db.ServiceAccountRow, error) {
	rows, err := svcListServiceAccountRows(ctx)
	if err != nil {
		return nil, err
	}
	marks := make(map[string]db.ServiceAccountRow, len(rows))
	for _, r := range rows {
		marks[r.UserID] = r
	}
	return marks, nil
}

// isServiceAccountIn is the one rule: Zitadel says it is a machine, or the
// operator marked it. Every flow that leaves people for service accounts asks
// this, so none of them can disagree about who is one.
func isServiceAccountIn(marks map[string]db.ServiceAccountRow, u models.UserProfile) bool {
	if u.IsMachine() {
		return true
	}
	_, marked := marks[u.ID]
	return marked
}

// flagServiceAccounts returns the users with the effective answer set, for the
// surfaces that serve users to a screen. A copy: the directory's slice is
// shared and cached, and one request's marks must not leak into another's.
func flagServiceAccounts(users []models.UserProfile, marks map[string]db.ServiceAccountRow) []models.UserProfile {
	out := make([]models.UserProfile, len(users))
	for i, u := range users {
		u.ServiceAccount = isServiceAccountIn(marks, u)
		out[i] = u
	}
	return out
}

// IsServiceAccount answers the rule for one user. A failed read of the marks is
// an error, never "a person": callers that hand out access must not guess.
func IsServiceAccount(ctx context.Context, u models.UserProfile) (bool, error) {
	if u.IsMachine() {
		return true, nil
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return false, err
	}
	return isServiceAccountIn(marks, u), nil
}

// The four kinds an account made for a purpose can be.
var serviceAccountKinds = map[string]bool{"integration": true, "device": true, "agent": true, "shared_login": true}

const maxServicePurpose = 200

var (
	svcUpsertServiceAccount = db.UpsertServiceAccount
	svcDeleteServiceAccount = db.DeleteServiceAccount
	// svcSnapshotOf is the access snapshot over exactly these users, for the
	// one reading of what they hold that the People list also uses.
	svcSnapshotOf = func(ctx context.Context, users []models.UserProfile) *accessSnapshot {
		return &accessSnapshot{
			ctx: ctx, users: users,
			roles:    make(map[string]userRoles, len(users)),
			observed: make(map[string]map[string]bool, len(users)),
		}
	}
)

var (
	ErrServiceAccountNotFound = errors.New("not a service account")
	ErrMachineAccount         = errors.New("Zitadel says it's a machine account")
)

// ServiceAccountInvalid is a refused field: the request names something a
// service account cannot be.
type ServiceAccountInvalid struct{ Field, Message string }

func (e *ServiceAccountInvalid) Error() string { return e.Field + ": " + e.Message }

// ServiceAccountInput is what an operator sets on an account.
type ServiceAccountInput struct{ Kind, Purpose, OwnerID string }

// serviceAccountViews builds the list from one snapshot: every machine user and
// every marked human, by name. What each holds is the People list's own
// reading, taken from the same code, so the two can never give two numbers.
func serviceAccountViews(snap *accessSnapshot, marks map[string]db.ServiceAccountRow) ([]models.ServiceAccount, error) {
	var accounts []models.UserProfile
	for _, u := range snap.Users() {
		if isServiceAccountIn(marks, u) {
			accounts = append(accounts, u)
		}
	}
	sub := *snap
	sub.users = accounts
	items, err := listUsersFromSnapshot(&sub, "", attentionIndex{})
	if err != nil {
		return nil, err
	}
	out := make([]models.ServiceAccount, 0, len(items))
	for _, it := range items {
		v := models.ServiceAccount{
			ID: it.User.ID, Name: it.User.Name, Kind: "integration", ZitadelType: models.UserKindHuman,
			RoleCount: it.EffectiveRoleCount,
		}
		if it.User.IsMachine() {
			v.ZitadelType = models.UserKindMachine
		}
		if it.ObservedRoleCount != nil {
			v.RoleCount, v.RoleCountObserved = *it.ObservedRoleCount, true
		}
		if row, ok := marks[it.User.ID]; ok {
			v.Kind, v.Purpose, v.OwnerID, v.MarkedBy = row.Kind, row.Purpose, row.OwnerID, row.MarkedBy
			at := row.MarkedAt
			v.MarkedAt = &at
		}
		out = append(out, v)
	}
	// By name as a person reads it: "netbird" does not sort after "Zed".
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ListServiceAccounts is every service account, by name.
func ListServiceAccounts(ctx context.Context) ([]models.ServiceAccount, error) {
	users, err := svcDirectoryUsers(ctx)
	if err != nil {
		return nil, err
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return nil, err
	}
	return serviceAccountViews(svcSnapshotOf(ctx, users), marks)
}

// viewOf builds the one account's view, or ErrServiceAccountNotFound when the
// user is neither a machine nor marked.
func viewOf(ctx context.Context, u models.UserProfile, marks map[string]db.ServiceAccountRow) (models.ServiceAccount, error) {
	if !isServiceAccountIn(marks, u) {
		return models.ServiceAccount{}, ErrServiceAccountNotFound
	}
	views, err := serviceAccountViews(svcSnapshotOf(ctx, []models.UserProfile{u}), marks)
	if err != nil || len(views) != 1 {
		return models.ServiceAccount{}, fmt.Errorf("service account %s: %w", u.ID, err)
	}
	return views[0], nil
}

// GetServiceAccount is one account; ErrServiceAccountNotFound when the id is
// unknown or belongs to a person.
func GetServiceAccount(ctx context.Context, id string) (models.ServiceAccount, error) {
	u, found, err := svcFindUser(ctx, id)
	if err != nil {
		return models.ServiceAccount{}, err
	}
	if !found {
		return models.ServiceAccount{}, ErrServiceAccountNotFound
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return models.ServiceAccount{}, err
	}
	return viewOf(ctx, u, marks)
}

// SaveServiceAccount marks a human-type account, or changes the kind, purpose
// or owner of any service account. Writes only Syndra's own table.
func SaveServiceAccount(ctx context.Context, id string, in ServiceAccountInput, actor string) (models.ServiceAccount, error) {
	u, found, err := svcFindUser(ctx, id)
	if err != nil {
		return models.ServiceAccount{}, err
	}
	if !found {
		return models.ServiceAccount{}, ErrServiceAccountNotFound
	}
	if !serviceAccountKinds[in.Kind] {
		return models.ServiceAccount{}, &ServiceAccountInvalid{"kind", "Choose what it is: an integration, a device, an agent or a shared login."}
	}
	purpose := strings.TrimSpace(in.Purpose)
	if utf8.RuneCountInString(purpose) > maxServicePurpose {
		return models.ServiceAccount{}, &ServiceAccountInvalid{"purpose", "Keep the purpose to 200 characters."}
	}
	marks, err := serviceAccountMarks(ctx)
	if err != nil {
		return models.ServiceAccount{}, err
	}
	if in.OwnerID != "" {
		if err := checkServiceOwner(ctx, id, in.OwnerID, marks); err != nil {
			return models.ServiceAccount{}, err
		}
	}
	row, err := svcUpsertServiceAccount(ctx, id, in.Kind, purpose, in.OwnerID, actor)
	if err != nil {
		return models.ServiceAccount{}, err
	}
	marks[id] = row
	return viewOf(ctx, u, marks)
}

// checkServiceOwner: the person who answers for an account is a known person,
// not the account itself and not another service account.
func checkServiceOwner(ctx context.Context, id, ownerID string, marks map[string]db.ServiceAccountRow) error {
	notPerson := &ServiceAccountInvalid{"owner_id", "Choose a person to answer for it."}
	if ownerID == id {
		return notPerson
	}
	owner, found, err := svcFindUser(ctx, ownerID)
	if err != nil {
		return err
	}
	if !found || isServiceAccountIn(marks, owner) {
		return notPerson
	}
	return nil
}

// UnmarkServiceAccount is "Mark as a person": it removes the operator's mark.
// A machine user stays a service account because Zitadel says so, so that is
// ErrMachineAccount; a user with no mark is ErrServiceAccountNotFound.
func UnmarkServiceAccount(ctx context.Context, id string) error {
	u, found, err := svcFindUser(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return ErrServiceAccountNotFound
	}
	if u.IsMachine() {
		return ErrMachineAccount
	}
	removed, err := svcDeleteServiceAccount(ctx, id)
	if err != nil {
		return err
	}
	if !removed {
		return ErrServiceAccountNotFound
	}
	return nil
}
