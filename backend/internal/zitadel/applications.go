package zitadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ZitadelApplication represents an application (OIDC client, API, or SAML SP)
// attached to a Zitadel project. Applications are distinct from projects —
// a project is a grouping boundary, an application is an actual client that
// authenticates end users or services.
type ZitadelApplication struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	// Type is derived from which *Config block the Zitadel response carries.
	// Values: "OIDC", "API", "SAML", or "" when the shape is unrecognized.
	Type string `json:"-"`
	// ClientID is the oidcConfig/apiConfig clientId ("" for SAML or when absent).
	ClientID string `json:"-"`
	// Created is details.creationDate; zero when Zitadel gave none.
	Created time.Time `json:"-"`
}

// HumanizeAppType maps the derived Type onto a short label for the "consumer"
// column on the admin /applications page. The UI already renders this field,
// so we piggyback on it rather than growing the DTO with a dedicated column.
func HumanizeAppType(t string) string {
	switch t {
	case "OIDC":
		return "OIDC Client"
	case "API":
		return "API"
	case "SAML":
		return "SAML SP"
	default:
		return ""
	}
}

func (c *managementClient) ListApplications(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ZitadelApplication], error) {
	path := fmt.Sprintf("/management/v1/projects/%s/apps/_search", projectID)
	body := map[string]any{
		"query": searchQuery(p),
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list applications response: %w", err)
	}

	// Zitadel returns one of oidcConfig / apiConfig / samlConfig per app depending
	// on its type. Type is not carried as a top-level discriminator, so we infer
	// it from which config block is populated.
	var result struct {
		Details searchDetails `json:"details"`
		Result  []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			State   string `json:"state"`
			Details struct {
				CreationDate string `json:"creationDate"`
			} `json:"details"`
			OIDCConfig json.RawMessage `json:"oidcConfig,omitempty"`
			APIConfig  json.RawMessage `json:"apiConfig,omitempty"`
			SAMLConfig json.RawMessage `json:"samlConfig,omitempty"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list applications response: %w", err)
	}

	apps := make([]ZitadelApplication, len(result.Result))
	for i, r := range result.Result {
		apps[i] = ZitadelApplication{
			ID:       r.ID,
			Name:     r.Name,
			State:    r.State,
			Type:     deriveAppType(r.OIDCConfig, r.APIConfig, r.SAMLConfig),
			ClientID: clientIDOf(r.OIDCConfig, r.APIConfig),
			Created:  parseCreationDate(r.Details.CreationDate),
		}
	}
	return &SearchResult[ZitadelApplication]{Items: apps, Total: result.Details.totalInt()}, nil
}

// clientIDOf reads clientId from whichever of the OIDC or API config blocks is
// present. A block that does not parse yields "", never an error: the client id
// only helps attribute a sign-in to a project.
func clientIDOf(blocks ...json.RawMessage) string {
	for _, b := range blocks {
		var c struct {
			ClientID string `json:"clientId"`
		}
		if len(b) > 0 && json.Unmarshal(b, &c) == nil && c.ClientID != "" {
			return c.ClientID
		}
	}
	return ""
}

func deriveAppType(oidc, api, saml json.RawMessage) string {
	switch {
	case len(oidc) > 0:
		return "OIDC"
	case len(api) > 0:
		return "API"
	case len(saml) > 0:
		return "SAML"
	default:
		return ""
	}
}

// TokenSettings are the Zitadel settings that decide whether an app's claims
// reach it, and where: check 1 of the claims explain step. Read-only.
type TokenSettings struct {
	// AccessTokenType is "JWT" or "BEARER" ("" for an app with no OIDC config).
	// Only a JWT access token carries claims.
	AccessTokenType string `json:"access_token_type"`
	// IDTokenUserinfoAssertion is "User Info inside ID Token": without it, Pre
	// Userinfo claims reach userinfo but not the ID token.
	IDTokenUserinfoAssertion bool `json:"id_token_userinfo_assertion"`
	// ProjectRoleAssertion is the project's "Assert roles on authentication".
	ProjectRoleAssertion bool `json:"project_role_assertion"`
	// ProjectRoleCheck is the project's "Check authorization on authentication":
	// only people holding a role may sign in.
	ProjectRoleCheck bool `json:"project_role_check"`
}

// TokenSettings reads one app's OIDC config and its project's role settings.
func (c *managementClient) TokenSettings(ctx context.Context, projectID, appID string) (*TokenSettings, error) {
	var app struct {
		App struct {
			OIDCConfig *struct {
				AccessTokenType          string `json:"accessTokenType"`
				IDTokenUserinfoAssertion bool   `json:"idTokenUserinfoAssertion"`
			} `json:"oidcConfig"`
		} `json:"app"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("/management/v1/projects/%s/apps/%s", projectID, appID), &app); err != nil {
		return nil, fmt.Errorf("get app %s: %w", appID, err)
	}
	var project struct {
		Project struct {
			ProjectRoleAssertion bool `json:"projectRoleAssertion"`
			ProjectRoleCheck     bool `json:"projectRoleCheck"`
		} `json:"project"`
	}
	if err := c.getJSON(ctx, "/management/v1/projects/"+projectID, &project); err != nil {
		return nil, fmt.Errorf("get project %s: %w", projectID, err)
	}
	out := &TokenSettings{
		ProjectRoleAssertion: project.Project.ProjectRoleAssertion,
		ProjectRoleCheck:     project.Project.ProjectRoleCheck,
	}
	if oidc := app.App.OIDCConfig; oidc != nil {
		out.IDTokenUserinfoAssertion = oidc.IDTokenUserinfoAssertion
		// Zitadel omits the enum's zero value, which is Bearer.
		out.AccessTokenType = "BEARER"
		if oidc.AccessTokenType == "OIDC_TOKEN_TYPE_JWT" {
			out.AccessTokenType = "JWT"
		}
	}
	return out, nil
}

func (c *managementClient) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
