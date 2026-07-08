package zitadel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// UserMetadata is a key/value pair an admin has attached to a Zitadel user.
// Zitadel stores arbitrary metadata per user as an opaque K/V store; MkAuth
// reads well-known keys (title/team) to populate UserProfile fields
// that Zitadel's first-class schema doesn't model.
type UserMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (c *managementClient) ListUserMetadata(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserMetadata], error) {
	path := fmt.Sprintf("/management/v1/users/%s/metadata/_search", userID)
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
		return nil, fmt.Errorf("read list user metadata response: %w", err)
	}

	// Zitadel returns metadata values as base64-encoded strings to preserve
	// arbitrary bytes. Decode them here so callers see the plaintext value.
	var result struct {
		Details searchDetails `json:"details"`
		Result  []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list user metadata response: %w", err)
	}

	out := make([]UserMetadata, len(result.Result))
	for i, r := range result.Result {
		out[i] = UserMetadata{Key: r.Key, Value: decodeMetadataValue(r.Value)}
	}
	return &SearchResult[UserMetadata]{Items: out, Total: result.Details.totalInt()}, nil
}

// decodeMetadataValue returns the plaintext metadata value. Zitadel returns
// base64 in practice; fall back to the raw string so non-base64 test fixtures
// or future API changes still render something sensible instead of blank.
func decodeMetadataValue(encoded string) string {
	if encoded == "" {
		return ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return string(decoded)
	}
	return encoded
}
