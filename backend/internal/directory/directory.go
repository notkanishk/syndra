// Package directory is the seam between Syndra's business logic and the
// identity-source of truth.
//
// When the Zitadel Management client is initialized (ZITADEL_DOMAIN +
// ZITADEL_MACHINE_KEY_PATH), Default is backed by a live Zitadel source that
// reads users, projects, and project roles from the Management API and
// overlays Syndra's claim_profiles table for application metadata. While
// Zitadel doesn't answer it serves the last good read, marked not current
// (Freshness). The demo catalog is used only when Zitadel is not configured
// at all (local development): Syndra never shows sample data in place of
// Zitadel (N23).
//
// Business logic calls directory.Default.<Method>(ctx) instead of demo.<Method>()
// so a single env-driven switch flips the whole UI surface to live data.
package directory

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// Source abstracts the directory-of-truth for users, projects, and applications.
// Writes are not modeled here: Syndra's local DB owns bundles, mapping rules,
// direct grants, and audit logs, and the Zitadel Management client handles
// grant/role mutations directly.
type Source interface {
	Users(ctx context.Context) ([]models.UserProfile, error)
	FindUser(ctx context.Context, userID string) (models.UserProfile, bool, error)

	Projects(ctx context.Context) ([]models.ProjectCatalog, error)
	FindProject(ctx context.Context, projectID string) (models.ProjectCatalog, bool, error)

	Applications(ctx context.Context) ([]models.ApplicationCatalog, error)
	FindApplication(ctx context.Context, appID string) (models.ApplicationCatalog, bool, error)

	RoleKeysForProject(ctx context.Context, projectID string) ([]string, error)
	ProjectName(ctx context.Context, projectID string) (string, error)

	// Tag identifies the source for audit/display surfaces. "demo" for the
	// local-dev fallback, "zitadel" for the live Management API backing.
	Tag() string

	// InvalidateAll drops every cached entry. Called by bulk/unknown-scope writes.
	InvalidateAll()
	// InvalidateProject drops the given project's cached roles + any list-level
	// entries that embed project data (projects list, applications list).
	InvalidateProject(projectID string)
	// InvalidateUsers drops the users list cache. Called after user-facing
	// mutations in the /zitadel admin surface.
	InvalidateUsers()
}

// ErrUnreachable is a read Zitadel didn't answer, with no last good read to
// serve in its place. Handlers answer it 503 DIRECTORY_UNREACHABLE, no rows.
var ErrUnreachable = errors.New("directory unreachable")

// FreshnessOf says when the source last answered (zero when it never has) and
// whether it is answering now. A source that can't go stale (demo) is always
// current.
func FreshnessOf(s Source) (lastRead time.Time, current bool) {
	if f, ok := s.(interface{ Freshness() (time.Time, bool) }); ok {
		return f.Freshness()
	}
	return time.Time{}, true
}

// Default is the active directory source. Callers use it as
// directory.Default.Users(ctx) etc. Tests can override it.
//
// Until Init() runs, Default falls back to the demo source so that package-init
// code paths and tests work without explicit setup.
var Default Source = NewDemoSource()

// Init picks the live or demo source based on whether the Zitadel Management
// client was initialized. Must be called once at startup after
// zitadel.InitClient().
func Init() {
	if zitadel.MgmtClient != nil {
		Default = NewZitadelSource(zitadel.MgmtClient)
		log.Println("[DIRECTORY] Source=zitadel (live Zitadel Management API backing users/projects/roles)")
		return
	}
	if os.Getenv("ZITADEL_DOMAIN") != "" {
		// Configured, but the client couldn't be built (a key that won't
		// load). Nothing has been read, so nothing is shown: never the demo.
		Default = unreachableSource{}
		log.Println("[DIRECTORY] Source=unreachable (Zitadel is configured but its client could not be initialized; no directory reads until it can)")
		return
	}
	Default = NewDemoSource()
	log.Println("[DIRECTORY] Source=demo (local-dev fallback; set ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH for live)")
}

// unreachableSource answers every read with ErrUnreachable.
type unreachableSource struct{}

func (unreachableSource) Users(context.Context) ([]models.UserProfile, error) {
	return nil, ErrUnreachable
}
func (unreachableSource) FindUser(context.Context, string) (models.UserProfile, bool, error) {
	return models.UserProfile{}, false, ErrUnreachable
}
func (unreachableSource) Projects(context.Context) ([]models.ProjectCatalog, error) {
	return nil, ErrUnreachable
}
func (unreachableSource) FindProject(context.Context, string) (models.ProjectCatalog, bool, error) {
	return models.ProjectCatalog{}, false, ErrUnreachable
}
func (unreachableSource) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return nil, ErrUnreachable
}
func (unreachableSource) FindApplication(context.Context, string) (models.ApplicationCatalog, bool, error) {
	return models.ApplicationCatalog{}, false, ErrUnreachable
}
func (unreachableSource) RoleKeysForProject(context.Context, string) ([]string, error) {
	return nil, ErrUnreachable
}
func (unreachableSource) ProjectName(context.Context, string) (string, error) {
	return "", ErrUnreachable
}
func (unreachableSource) Tag() string                  { return "unreachable" }
func (unreachableSource) Freshness() (time.Time, bool) { return time.Time{}, false }
func (unreachableSource) InvalidateAll()               {}
func (unreachableSource) InvalidateProject(string)     {}
func (unreachableSource) InvalidateUsers()             {}
