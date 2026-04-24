// Package directory is the seam between MkAuth's business logic and the
// identity-source of truth.
//
// When the Zitadel Management client is initialized (ZITADEL_DOMAIN +
// ZITADEL_MACHINE_KEY_PATH), Default is backed by a live Zitadel source that
// reads users, projects, and project roles from the Management API and
// overlays MkAuth's claim_profiles table for application metadata. Otherwise
// it falls back to the demo catalog so local development stays usable
// without a live Zitadel.
//
// Business logic calls directory.Default.<Method>(ctx) instead of demo.<Method>()
// so a single env-driven switch flips the whole UI surface to live data.
package directory

import (
	"context"
	"log"

	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

// Source abstracts the directory-of-truth for users, projects, and applications.
// Writes are not modeled here: MkAuth's local DB owns bundles, mapping rules,
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
	Default = NewDemoSource()
	log.Println("[DIRECTORY] Source=demo (local-dev fallback; set ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH for live)")
}
