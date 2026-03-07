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
// agent_skills (list)
// ---------------------------------------------------------------------------

func (h *handlers) agentListSkills(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}

	resp, err := h.client.ListSkills(ctx, agentUUID)
	if err != nil {
		return toolResultFromError("list skills", err), nil
	}

	return toolResultJSON("agent_skills", resp)
}

// ---------------------------------------------------------------------------
// agent_add_skill
// ---------------------------------------------------------------------------

func (h *handlers) agentAddSkill(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	skillID := mcp.ParseString(req, "id", "")
	name := mcp.ParseString(req, "name", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if skillID == "" {
		return mcp.NewToolResultError("id is required"), nil
	}
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	skill := Skill{
		ID:          skillID,
		Name:        name,
		Description: mcp.ParseString(req, "description", ""),
		Tags:        parseStringArray(req, "tags"),
		Examples:    parseStringArray(req, "examples"),
	}

	result, err := h.client.AddSkill(ctx, agentUUID, apiKey, skill)
	if err != nil {
		return toolResultFromError("add skill", err), nil
	}

	return toolResultJSON("agent_add_skill", result)
}

// ---------------------------------------------------------------------------
// agent_delete_skill
// ---------------------------------------------------------------------------

func (h *handlers) agentDeleteSkill(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	skillID := mcp.ParseString(req, "skill_id", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if skillID == "" {
		return mcp.NewToolResultError("skill_id is required"), nil
	}

	if err := h.client.DeleteSkill(ctx, agentUUID, apiKey, skillID); err != nil {
		return toolResultFromError("delete skill", err), nil
	}

	return mcp.NewToolResultText("skill deleted"), nil
}

// ---------------------------------------------------------------------------
// agent_custom_fields (list)
// ---------------------------------------------------------------------------

func (h *handlers) agentListCustomFields(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}

	resp, err := h.client.ListCustomFields(ctx, agentUUID, apiKey)
	if err != nil {
		return toolResultFromError("list custom fields", err), nil
	}

	return toolResultJSON("agent_custom_fields", resp)
}

// ---------------------------------------------------------------------------
// agent_set_custom_field
// ---------------------------------------------------------------------------

func (h *handlers) agentSetCustomField(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	key := mcp.ParseString(req, "key", "")
	value := mcp.ParseString(req, "value", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}

	if err := h.client.SetCustomField(ctx, agentUUID, apiKey, key, value); err != nil {
		return toolResultFromError("set custom field", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("custom field %q set", key)), nil
}

// ---------------------------------------------------------------------------
// agent_delete_custom_field
// ---------------------------------------------------------------------------

func (h *handlers) agentDeleteCustomField(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	key := mcp.ParseString(req, "key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}

	if err := h.client.DeleteCustomField(ctx, agentUUID, apiKey, key); err != nil {
		return toolResultFromError("delete custom field", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("custom field %q deleted", key)), nil
}

// ---------------------------------------------------------------------------
// well_known_fields (list recognized keys)
// ---------------------------------------------------------------------------

func (h *handlers) agentWellKnownFields(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp, err := h.client.ListWellKnownFields(ctx)
	if err != nil {
		return toolResultFromError("list well-known fields", err), nil
	}

	return toolResultJSON("well_known_fields", resp)
}

// ---------------------------------------------------------------------------
// agent_private_metadata (list)
// ---------------------------------------------------------------------------

func (h *handlers) agentListPrivateMetadata(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}

	resp, err := h.client.ListPrivateMetadata(ctx, agentUUID, apiKey)
	if err != nil {
		return toolResultFromError("list private metadata", err), nil
	}

	return toolResultJSON("agent_private_metadata", resp)
}

// ---------------------------------------------------------------------------
// agent_get_private
// ---------------------------------------------------------------------------

func (h *handlers) agentGetPrivate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	key := mcp.ParseString(req, "key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}

	entry, err := h.client.GetPrivateMetadata(ctx, agentUUID, apiKey, key)
	if err != nil {
		return toolResultFromError("get private metadata", err), nil
	}

	return toolResultJSON("agent_get_private", entry)
}

// ---------------------------------------------------------------------------
// agent_set_private
// ---------------------------------------------------------------------------

func (h *handlers) agentSetPrivate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	key := mcp.ParseString(req, "key", "")
	value := mcp.ParseString(req, "value", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}

	if err := h.client.SetPrivateMetadata(ctx, agentUUID, apiKey, key, value); err != nil {
		return toolResultFromError("set private metadata", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("private metadata %q set", key)), nil
}

// ---------------------------------------------------------------------------
// agent_delete_private
// ---------------------------------------------------------------------------

func (h *handlers) agentDeletePrivate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")
	key := mcp.ParseString(req, "key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}
	if key == "" {
		return mcp.NewToolResultError("key is required"), nil
	}

	if err := h.client.DeletePrivateMetadata(ctx, agentUUID, apiKey, key); err != nil {
		return toolResultFromError("delete private metadata", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("private metadata %q deleted", key)), nil
}

// ---------------------------------------------------------------------------
// agent_create_session
// ---------------------------------------------------------------------------

func (h *handlers) agentCreateSession(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := mcp.ParseString(req, "api_key", "")

	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required (must be iak_ key, not session token)"), nil
	}

	resp, err := h.client.CreateSession(ctx, apiKey)
	if err != nil {
		return toolResultFromError("create session", err), nil
	}

	return toolResultJSON("agent_create_session", resp)
}

// ---------------------------------------------------------------------------
// agent_revoke_session
// ---------------------------------------------------------------------------

func (h *handlers) agentRevokeSession(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiKey := mcp.ParseString(req, "api_key", "")

	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required"), nil
	}

	if err := h.client.RevokeSession(ctx, apiKey); err != nil {
		return toolResultFromError("revoke session", err), nil
	}

	return mcp.NewToolResultText("session revoked"), nil
}

// ---------------------------------------------------------------------------
// agent_rotate_key
// ---------------------------------------------------------------------------

func (h *handlers) agentRotateKey(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentUUID := mcp.ParseString(req, "uuid", "")
	apiKey := mcp.ParseString(req, "api_key", "")

	if agentUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}
	if apiKey == "" {
		return mcp.NewToolResultError("api_key is required (must be iak_ key)"), nil
	}

	resp, err := h.client.RotateKey(ctx, agentUUID, apiKey)
	if err != nil {
		return toolResultFromError("rotate key", err), nil
	}

	return toolResultJSON("agent_rotate_key", resp)
}

// ---------------------------------------------------------------------------
// agent_recover
// ---------------------------------------------------------------------------

func (h *handlers) agentRecover(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	certToken := mcp.ParseString(req, "cert_token", "")
	handle := mcp.ParseString(req, "handle", "")
	agentUUID := mcp.ParseString(req, "uuid", "")

	if certToken == "" {
		return mcp.NewToolResultError("cert_token is required"), nil
	}
	if handle == "" && agentUUID == "" {
		return mcp.NewToolResultError("either handle or uuid is required"), nil
	}

	recoverReq := RecoverReq{
		CertToken: certToken,
		Handle:    handle,
		UUID:      agentUUID,
	}

	if err := h.client.Recover(ctx, recoverReq); err != nil {
		return toolResultFromError("recover", err), nil
	}

	return mcp.NewToolResultText("recovery email sent — check the recovery email for a token"), nil
}

// ---------------------------------------------------------------------------
// agent_recover_confirm
// ---------------------------------------------------------------------------

func (h *handlers) agentRecoverConfirm(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	recoveryToken := mcp.ParseString(req, "recovery_token", "")
	certToken := mcp.ParseString(req, "cert_token", "")

	if recoveryToken == "" {
		return mcp.NewToolResultError("recovery_token is required"), nil
	}
	if certToken == "" {
		return mcp.NewToolResultError("cert_token is required"), nil
	}

	resp, err := h.client.RecoverConfirm(ctx, RecoverConfirmReq{
		RecoveryToken: recoveryToken,
		CertToken:     certToken,
	})
	if err != nil {
		return toolResultFromError("recover confirm", err), nil
	}

	return toolResultJSON("agent_recover_confirm", resp)
}

// ---------------------------------------------------------------------------
// agent_registry
// ---------------------------------------------------------------------------

func (h *handlers) agentRegistry(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := RegistryParams{
		Q:        mcp.ParseString(req, "q", ""),
		Skill:    mcp.ParseString(req, "skill", ""),
		Tag:      mcp.ParseString(req, "tag", ""),
		Level:    mcp.ParseString(req, "level", ""),
		MinTrust: mcp.ParseInt(req, "min_trust", 0),
		Sort:     mcp.ParseString(req, "sort", ""),
		Page:     mcp.ParseInt(req, "page", 0),
		PerPage:  mcp.ParseInt(req, "per_page", 0),
	}

	page, err := h.client.GetRegistry(ctx, params)
	if err != nil {
		return toolResultFromError("registry", err), nil
	}

	return toolResultJSON("agent_registry", page)
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
