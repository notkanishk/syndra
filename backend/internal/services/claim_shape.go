package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Claim shaping — the operator's control over what an application receives.
//
// A project has one default profile; an application may override it. Because
// the Zitadel Actions v2 function payload carries no application identifier
// (documented fields: function, userinfo, user, user_metadata, org,
// user_grants), a token issued for a project carries the default AND every
// override key on that project, and each application reads its own. That is
// why claim keys must be unique within a project. A token never spans two
// projects (the data plane refuses cross-project delivery), so keys need not
// be unique across them.

// ResolveClaimProfiles returns the profile set the data plane must apply for a
// project: the default (built-in if the operator never edited it) followed by
// every application override. Deliberately free of display names — the data
// plane pays for nothing it will not emit.
func ResolveClaimProfiles(ctx context.Context, projectID string) ([]claims.Profile, error) {
	row, found, err := svcGetClaimProfile(ctx, projectID)
	if err != nil {
		return nil, err
	}

	def := claims.Profile{
		ProjectID:  projectID,
		ClaimName:  claims.DefaultClaimName,
		FormatType: claims.DefaultFormat,
	}
	if found {
		def = profileFromRow(row)
	}
	out := []claims.Profile{def}

	overrides, err := svcListAppClaimOverrides(ctx)
	if err != nil {
		return nil, err
	}
	for _, o := range overrides {
		if o.ProjectID != projectID {
			continue
		}
		out = append(out, profileFromOverrideRow(o))
	}
	return out, nil
}

// ClaimShape is the operator-facing view of one project's token shape: the
// editable default, every override with the application it belongs to, and the
// full list of keys a token for this project will carry.
type ClaimShape struct {
	ProjectID    string                      `json:"project_id"`
	ProjectName  string                      `json:"project_name"`
	Default      claims.Profile              `json:"default"`
	Overrides    []claims.Profile            `json:"overrides"`
	Applications []models.ApplicationCatalog `json:"applications"`
	EmittedKeys  []models.ClaimKeyOwner      `json:"emitted_keys"`
	Conflicts    []claims.Conflict           `json:"conflicts"`
}

// ProjectClaimShape builds the operator view for one project.
func ProjectClaimShape(ctx context.Context, projectID string) (ClaimShape, error) {
	profiles, err := ResolveClaimProfiles(ctx, projectID)
	if err != nil {
		return ClaimShape{}, err
	}

	apps, err := directory.Default.Applications(ctx)
	if err != nil {
		return ClaimShape{}, err
	}
	appNames := map[string]string{}
	projectApps := []models.ApplicationCatalog{}
	for _, a := range apps {
		appNames[a.ID] = a.Name
		if a.ProjectID == projectID {
			projectApps = append(projectApps, a)
		}
	}

	shape := ClaimShape{
		ProjectID:    projectID,
		ProjectName:  projectID,
		Applications: projectApps,
		Overrides:    []claims.Profile{},
	}
	if name, nerr := directory.Default.ProjectName(ctx, projectID); nerr == nil && name != "" {
		shape.ProjectName = name
	}

	for _, p := range profiles {
		if p.ApplicationID == "" {
			shape.Default = p
			continue
		}
		p.ApplicationName = appNames[p.ApplicationID]
		shape.Overrides = append(shape.Overrides, p)
	}
	sort.Slice(shape.Overrides, func(i, j int) bool {
		return shape.Overrides[i].ApplicationID < shape.Overrides[j].ApplicationID
	})

	shape.EmittedKeys = emittedKeys(append([]claims.Profile{shape.Default}, shape.Overrides...))
	shape.Conflicts = claims.Conflicts(append([]claims.Profile{shape.Default}, shape.Overrides...))
	return shape, nil
}

// emittedKeys flattens a profile set into the token's key inventory.
func emittedKeys(profiles []claims.Profile) []models.ClaimKeyOwner {
	out := []models.ClaimKeyOwner{}
	for _, p := range profiles {
		label := "Project default"
		if p.ApplicationID != "" {
			label = p.ApplicationName
			if label == "" {
				label = p.ApplicationID
			}
		}
		claimName := strings.TrimSpace(p.ClaimName)
		if claimName == "" {
			claimName = claims.DefaultClaimName
		}
		out = append(out, models.ClaimKeyOwner{
			Key: claimName, OwnerLabel: label, ApplicationID: p.ApplicationID, Kind: "roles",
		})
		for key, source := range p.AttributeClaims {
			out = append(out, models.ClaimKeyOwner{
				Key: key, OwnerLabel: label, ApplicationID: p.ApplicationID, Kind: "attribute", Source: source,
			})
		}
		for key := range p.StaticClaims {
			out = append(out, models.ClaimKeyOwner{
				Key: key, OwnerLabel: label, ApplicationID: p.ApplicationID, Kind: "static",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// SaveProjectClaimProfile validates and persists a project's default shape.
func SaveProjectClaimProfile(ctx context.Context, projectID string, p claims.Profile) error {
	p.ProjectID = projectID
	p.ApplicationID = ""
	if err := validateAgainstProject(ctx, p); err != nil {
		return err
	}
	return svcUpsertClaimProfile(ctx, db.ClaimProfileRow{
		ProjectID:       projectID,
		ClaimName:       strings.TrimSpace(p.ClaimName),
		FormatType:      p.FormatType,
		AttributeClaims: p.AttributeClaims,
		StaticClaims:    p.StaticClaims,
	})
}

// SaveAppClaimOverride validates and persists one application's override. The
// project is derived from the application itself rather than trusted from the
// request, so an override can never be filed against a project the application
// does not belong to.
func SaveAppClaimOverride(ctx context.Context, applicationID string, p claims.Profile) error {
	app, ok, err := directory.Default.FindApplication(ctx, applicationID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("application %q not found", applicationID)
	}

	p.ApplicationID = applicationID
	p.ApplicationName = app.Name
	p.ProjectID = app.ProjectID
	if err := validateAgainstProject(ctx, p); err != nil {
		return err
	}
	return svcUpsertAppClaimOverride(ctx, db.AppClaimOverrideRow{
		ApplicationID:   applicationID,
		ProjectID:       app.ProjectID,
		ClaimName:       strings.TrimSpace(p.ClaimName),
		FormatType:      p.FormatType,
		AttributeClaims: p.AttributeClaims,
		StaticClaims:    p.StaticClaims,
	})
}

// DeleteAppClaimOverride returns an application to its project's default shape.
func DeleteAppClaimOverride(ctx context.Context, applicationID string) error {
	return svcDeleteAppClaimOverride(ctx, applicationID)
}

// validateAgainstProject runs the standalone profile checks, then rejects any
// claim key another profile on the SAME project already emits, the built-in
// default included: a token carries its project's default and every override,
// so siblings share one flat namespace. Other projects never share a token
// (the data plane refuses cross-project delivery), so two projects may both
// use "roles".
func validateAgainstProject(ctx context.Context, p claims.Profile) error {
	if err := claims.ValidateProfile(p); err != nil {
		return err
	}

	emitted, err := ResolveClaimProfiles(ctx, p.ProjectID)
	if err != nil {
		return err
	}
	others := make([]claims.Profile, 0, len(emitted))
	for _, e := range emitted {
		if e.ApplicationID != p.ApplicationID {
			others = append(others, e) // skip the row being replaced
		}
	}

	if conflicts := claims.Conflicts(append(others, p)); len(conflicts) > 0 {
		return conflicts[0]
	}
	return nil
}

func profileFromRow(r db.ClaimProfileRow) claims.Profile {
	name := r.ClaimName
	if strings.TrimSpace(name) == "" {
		name = claims.DefaultClaimName
	}
	format := r.FormatType
	if format == "" {
		format = claims.DefaultFormat
	}
	return claims.Profile{
		ProjectID:       r.ProjectID,
		ClaimName:       name,
		FormatType:      format,
		AttributeClaims: r.AttributeClaims,
		StaticClaims:    r.StaticClaims,
	}
}

func profileFromOverrideRow(r db.AppClaimOverrideRow) claims.Profile {
	format := r.FormatType
	if format == "" {
		format = claims.DefaultFormat
	}
	return claims.Profile{
		ProjectID:       r.ProjectID,
		ApplicationID:   r.ApplicationID,
		ClaimName:       r.ClaimName,
		FormatType:      format,
		AttributeClaims: r.AttributeClaims,
		StaticClaims:    r.StaticClaims,
	}
}
