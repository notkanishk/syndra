package zitadel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			ID         string          `json:"id"`
			Name       string          `json:"name"`
			State      string          `json:"state"`
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
			ID:    r.ID,
			Name:  r.Name,
			State: r.State,
			Type:  deriveAppType(r.OIDCConfig, r.APIConfig, r.SAMLConfig),
		}
	}
	return &SearchResult[ZitadelApplication]{Items: apps, Total: result.Details.totalInt()}, nil
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
