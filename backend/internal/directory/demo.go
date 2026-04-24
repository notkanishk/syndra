package directory

import (
	"context"

	"mkauth/internal/demo"
	"mkauth/internal/models"
)

// demoSource is a thin delegate over the demo.* helpers. Used for local-dev
// when no Zitadel client is initialized.
type demoSource struct{}

// NewDemoSource returns a Source backed by the hardcoded demo catalog.
func NewDemoSource() Source { return &demoSource{} }

func (d *demoSource) Users(ctx context.Context) ([]models.UserProfile, error) {
	return demo.Users(), nil
}

func (d *demoSource) FindUser(ctx context.Context, userID string) (models.UserProfile, bool, error) {
	u, ok := demo.FindUser(userID)
	return u, ok, nil
}

func (d *demoSource) Projects(ctx context.Context) ([]models.ProjectCatalog, error) {
	return demo.Projects(), nil
}

func (d *demoSource) FindProject(ctx context.Context, projectID string) (models.ProjectCatalog, bool, error) {
	p, ok := demo.FindProject(projectID)
	return p, ok, nil
}

func (d *demoSource) Applications(ctx context.Context) ([]models.ApplicationCatalog, error) {
	return demo.Applications(), nil
}

func (d *demoSource) FindApplication(ctx context.Context, appID string) (models.ApplicationCatalog, bool, error) {
	a, ok := demo.FindApplication(appID)
	return a, ok, nil
}

func (d *demoSource) RoleKeysForProject(ctx context.Context, projectID string) ([]string, error) {
	return demo.RoleKeysForProject(projectID), nil
}

func (d *demoSource) ProjectName(ctx context.Context, projectID string) (string, error) {
	return demo.ProjectName(projectID), nil
}

func (d *demoSource) Tag() string { return "demo" }

// Invalidate* are no-ops — the demo catalog is in-memory and has no TTL.

func (d *demoSource) InvalidateAll()             {}
func (d *demoSource) InvalidateProject(_ string) {}
func (d *demoSource) InvalidateUsers()           {}
