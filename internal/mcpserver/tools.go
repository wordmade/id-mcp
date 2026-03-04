package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers groups all MCP tool and resource handler methods.
type handlers struct {
	client *IDClient
}

// ---------------------------------------------------------------------------
// agent_lookup
// ---------------------------------------------------------------------------

func (h *handlers) agentLookup(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	identifier := mcp.ParseString(req, "identifier", "")
	if identifier == "" {
		return mcp.NewToolResultError("identifier is required"), nil
	}

	// Normalize: accept "@@handle", "@handle", or bare "handle".
	// The API expects "@{handle}" for handle lookup.
	if !isUUIDLike(identifier) {
		identifier = "@" + strings.TrimLeft(identifier, "@")
	}

	profile, err := h.client.Lookup(ctx, identifier)
	if err != nil {
		return toolResultFromError("lookup", err), nil
	}

	return toolResultJSON("agent_lookup", profile)
}

// ---------------------------------------------------------------------------
// agent_directory
// ---------------------------------------------------------------------------

func (h *handlers) agentDirectory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := SearchParams{
		Q:          mcp.ParseString(req, "q", ""),
		Capability: mcp.ParseString(req, "capability", ""),
		Skill:      mcp.ParseString(req, "skill", ""),
		Tag:        mcp.ParseString(req, "tag", ""),
		Level:      mcp.ParseString(req, "level", ""),
		CertLevel:  mcp.ParseInt(req, "cert_level", 0),
		MinTrust:   mcp.ParseInt(req, "min_trust", 0),
		Sort:       mcp.ParseString(req, "sort", ""),
		Page:       mcp.ParseInt(req, "page", 0),
		PerPage:    mcp.ParseInt(req, "per_page", 0),
	}

	page, err := h.client.Search(ctx, params)
	if err != nil {
		return toolResultFromError("directory search", err), nil
	}

	return toolResultJSON("agent_directory", page)
}

// ---------------------------------------------------------------------------
// agent_verify
// ---------------------------------------------------------------------------

func (h *handlers) agentVerify(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	token := mcp.ParseString(req, "token", "")
	if token == "" {
		return mcp.NewToolResultError("token is required"), nil
	}
	audience := mcp.ParseString(req, "audience", "")

	result, err := h.client.Verify(ctx, token, audience)
	if err != nil {
		return toolResultFromError("verify", err), nil
	}

	return toolResultJSON("agent_verify", result)
}

// ---------------------------------------------------------------------------
// agent_register
// ---------------------------------------------------------------------------

func (h *handlers) agentRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	certToken := mcp.ParseString(req, "cert_token", "")
	handle := mcp.ParseString(req, "handle", "")
	name := mcp.ParseString(req, "name", "")
	acceptedTerms := mcp.ParseBoolean(req, "accepted_terms", false)

	if certToken == "" {
		return mcp.NewToolResultError("cert_token is required"), nil
	}
	if handle == "" {
		return mcp.NewToolResultError("handle is required"), nil
	}
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if !acceptedTerms {
		return mcp.NewToolResultError("accepted_terms must be true — the agent must accept the terms of service"), nil
	}

	// Parse capabilities from the array argument.
	caps := parseStringArray(req, "capabilities")

	regReq := RegisterRequest{
		CertToken:     certToken,
		Handle:        handle,
		Name:          name,
		BioOneliner:   mcp.ParseString(req, "bio_oneliner", ""),
		Capabilities:  caps,
		AcceptedTerms: true,
		RecoveryEmail: mcp.ParseString(req, "recovery_email", ""),
	}

	resp, err := h.client.Register(ctx, regReq)
	if err != nil {
		return toolResultFromError("register", err), nil
	}

	return toolResultJSON("agent_register", resp)
}

// ---------------------------------------------------------------------------
// agent_token
// ---------------------------------------------------------------------------

func (h *handlers) agentToken(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := mcp.ParseString(req, "api_key", "")
	certToken := mcp.ParseString(req, "cert_token", "")

	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if certToken == "" {
		return mcp.NewToolResultError("cert_token is required"), nil
	}

	handle := mcp.ParseString(req, "handle", "")
	agentUUID := mcp.ParseString(req, "uuid", "")
	if handle == "" && agentUUID == "" {
		return mcp.NewToolResultError("either handle or uuid is required"), nil
	}

	scope := parseStringArray(req, "scope")

	tokenReq := TokenRequest{
		Handle:    handle,
		UUID:      agentUUID,
		APIKey:    apiKey,
		CertToken: certToken,
		Audience:  mcp.ParseString(req, "audience", ""),
		Scope:     scope,
	}

	resp, err := h.client.IssueToken(ctx, tokenReq)
	if err != nil {
		return toolResultFromError("token", err), nil
	}

	return toolResultJSON("agent_token", resp)
}

// ---------------------------------------------------------------------------
// agent_profile (update)
// ---------------------------------------------------------------------------

func (h *handlers) agentProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}

	fields := ProfileUpdateFields{}
	if v := mcp.ParseString(req, "name", ""); v != "" {
		fields.Name = &v
	}
	if v := mcp.ParseString(req, "bio_oneliner", ""); v != "" {
		fields.BioOneliner = &v
	}
	if v := mcp.ParseString(req, "bio", ""); v != "" {
		fields.Bio = &v
	}
	if v := mcp.ParseString(req, "country", ""); v != "" {
		fields.Country = &v
	}
	if v := mcp.ParseString(req, "city", ""); v != "" {
		fields.City = &v
	}
	if v := mcp.ParseString(req, "business", ""); v != "" {
		fields.Business = &v
	}
	if caps := parseStringArray(req, "capabilities"); len(caps) > 0 {
		fields.Caps = caps
	}

	profile, err := h.client.UpdateProfile(ctx, agentUUID, apiKey, fields)
	if err != nil {
		return toolResultFromError("profile update", err), nil
	}

	return toolResultJSON("agent_profile", profile)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isUUIDLike checks if a string looks like a UUID (contains hyphens and hex chars).
func isUUIDLike(s string) bool {
	if len(s) < 32 {
		return false
	}
	for _, c := range s {
		if c == '-' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

// parseStringArray extracts a string array from a tool request argument.
func parseStringArray(req mcp.CallToolRequest, key string) []string {
	raw := mcp.ParseArgument(req, key, nil)
	if raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// toolResultJSON marshals data to indented JSON and returns as text content.
func toolResultJSON(toolName string, data interface{}) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%s: marshal result: %w", toolName, err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

// toolResultFromError converts an error into a user-friendly MCP tool error result.
func toolResultFromError(operation string, err error) *mcp.CallToolResult {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return mcp.NewToolResultError(fmt.Sprintf("%s failed: %s — %s (HTTP %d)",
			operation, apiErr.Code, apiErr.Message, apiErr.StatusCode))
	}
	return mcp.NewToolResultError(fmt.Sprintf("%s failed: %s", operation, err.Error()))
}
