// Package mcpserver implements a Model Context Protocol (MCP) server for
// Wordmade ID, exposing agent identity operations as MCP tools and resources.
//
// The server communicates with the ID REST API via HTTP — it does not access
// the database directly. This keeps the MCP server stateless and deployable
// as a standalone stdio process.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// IDClient is a thin HTTP wrapper for the Wordmade ID REST API.
type IDClient struct {
	apiURL string
	client *http.Client
}

// NewIDClient creates a client pointing at the given API base URL.
func NewIDClient(apiURL string) *IDClient {
	return &IDClient{
		apiURL: apiURL,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

// APIError represents a non-2xx response from the ID API.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"error"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("id-api %d: %s — %s", e.StatusCode, e.Code, e.Message)
}

// ---------------------------------------------------------------------------
// Response types — lightweight, self-contained (no imports from identity pkg)
// ---------------------------------------------------------------------------

// AgentProfile is the public shape returned by GET /v1/agents/{id}.
type AgentProfile struct {
	UUID         string                 `json:"uuid"`
	Handle       string                 `json:"handle"`
	Name         string                 `json:"name"`
	BioOneliner  string                 `json:"bio_oneliner"`
	Bio          string                 `json:"bio"`
	AvatarURL    string                 `json:"avatar_url"`
	Country      string                 `json:"country"`
	City         string                 `json:"city"`
	Business     string                 `json:"business"`
	Caps         []string               `json:"capabilities"`
	Verification map[string]interface{} `json:"verification"`
	Custom       map[string]string      `json:"custom"`
	WorldPres    []WorldPresence        `json:"world_presences"`
	Stats        map[string]interface{} `json:"stats"`
}

// WorldPresence is a structured world_N reference.
type WorldPresence struct {
	Title string `json:"title,omitempty"`
	WMW   string `json:"wmw"`
	URL   string `json:"url"`
}

// DirectoryPage is a paginated list of agents from GET /v1/directory.
type DirectoryPage struct {
	Agents  []json.RawMessage `json:"agents"`
	Total   int               `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Pages   int               `json:"pages"`
}

// DirectoryStats from GET /v1/directory/stats.
type DirectoryStats struct {
	TotalAgents    int            `json:"total_agents"`
	CertifiedToday int            `json:"certified_today"`
	Capabilities   map[string]int `json:"capabilities"`
}

// VerifyResult from POST /v1/verify.
type VerifyResult struct {
	Valid             bool     `json:"valid"`
	UUID              string   `json:"uuid,omitempty"`
	Handle            string   `json:"handle,omitempty"`
	Name              string   `json:"name,omitempty"`
	TrustScore        int      `json:"trust_score,omitempty"`
	VerificationLevel string   `json:"verification_level,omitempty"`
	Capabilities      []string `json:"capabilities,omitempty"`
	CertScore         float64  `json:"cert_score,omitempty"`
	CertLevel         int      `json:"cert_level,omitempty"`
	CertLevelLabel    string   `json:"cert_level_label,omitempty"`
	CertifiedAt       string   `json:"certified_at,omitempty"`
	Audience          string   `json:"audience,omitempty"`
	IssuedAt          string   `json:"issued_at,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	TokenType         string   `json:"token_type,omitempty"`
	Error             string   `json:"error,omitempty"`
}

// RegisterRequest is the payload for POST /v1/agents/register.
type RegisterRequest struct {
	CertToken     string   `json:"cert_token"`
	Handle        string   `json:"handle"`
	Name          string   `json:"name"`
	BioOneliner   string   `json:"bio_oneliner,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	AcceptedTerms bool     `json:"accepted_terms"`
	RecoveryEmail string   `json:"recovery_email,omitempty"`
}

// RegisterResponse from POST /v1/agents/register.
type RegisterResponse struct {
	UUID       string `json:"uuid"`
	Handle     string `json:"handle"`
	APIKey     string `json:"api_key"`
	APIKeyID   string `json:"api_key_id"`
	ProfileURL string `json:"profile_url"`
}

// TokenRequest is the payload for POST /v1/agents/token.
type TokenRequest struct {
	Handle    string   `json:"handle,omitempty"`
	UUID      string   `json:"uuid,omitempty"`
	APIKey    string   `json:"api_key"`
	CertToken string   `json:"cert_token"`
	Audience  string   `json:"audience,omitempty"`
	Scope     []string `json:"scope,omitempty"`
}

// TokenResponse from POST /v1/agents/token.
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	AgentUUID string `json:"agent_uuid"`
	Handle    string `json:"handle"`
}

// ProfileUpdateFields are the mutable fields accepted by PUT /v1/agents/{uuid}.
type ProfileUpdateFields struct {
	Name        *string  `json:"name,omitempty"`
	BioOneliner *string  `json:"bio_oneliner,omitempty"`
	Bio         *string  `json:"bio,omitempty"`
	Country     *string  `json:"country,omitempty"`
	City        *string  `json:"city,omitempty"`
	Business    *string  `json:"business,omitempty"`
	Caps        []string `json:"capabilities,omitempty"`
}

// Skill represents an agent skill.
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// SkillsResponse from GET /v1/agents/{uuid}/skills.
type SkillsResponse struct {
	Skills []Skill `json:"skills"`
	Count  int     `json:"count"`
}

// CustomField is a public custom key/value on an agent profile.
type CustomField struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at,omitempty"`
	WellKnown bool   `json:"well_known,omitempty"`
}

// CustomFieldsResponse from GET /v1/agents/{uuid}/custom.
type CustomFieldsResponse struct {
	Fields []CustomField `json:"fields"`
	Count  int           `json:"count"`
	Quota  int           `json:"quota"`
}

// WellKnownField describes a recognized custom field key.
type WellKnownField struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Rendering   string `json:"rendering,omitempty"`
	Example     string `json:"example,omitempty"`
	Format      string `json:"format,omitempty"`
}

// WellKnownFieldsResponse from GET /v1/custom-fields.
type WellKnownFieldsResponse struct {
	Fields []WellKnownField `json:"fields"`
	Count  int              `json:"count"`
	Note   string           `json:"note,omitempty"`
}

// MetadataEntry is a private metadata key/value pair.
type MetadataEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// MetadataListResponse from GET /v1/agents/{uuid}/private.
type MetadataListResponse struct {
	Keys  []MetadataEntry `json:"keys"`
	Count int             `json:"count"`
	Quota int             `json:"quota"`
}

// SessionResponse from POST /v1/agents/session.
type SessionResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	AgentUUID string `json:"agent_uuid"`
}

// RotateKeyResponse from POST /v1/agents/{uuid}/keys/rotate.
type RotateKeyResponse struct {
	APIKey      string `json:"api_key"`
	APIKeyID    string `json:"api_key_id"`
	Message     string `json:"message"`
	RevokedKeys int    `json:"revoked_keys"`
	ProfileURL  string `json:"profile_url"`
}

// RecoverReq is the payload for POST /v1/agents/recover.
type RecoverReq struct {
	CertToken string `json:"cert_token"`
	Handle    string `json:"handle,omitempty"`
	UUID      string `json:"uuid,omitempty"`
}

// RecoverConfirmReq is the payload for POST /v1/agents/recover/confirm.
type RecoverConfirmReq struct {
	RecoveryToken string `json:"recovery_token"`
	CertToken     string `json:"cert_token"`
}

// RecoverConfirmResponse from POST /v1/agents/recover/confirm.
type RecoverConfirmResponse struct {
	UUID     string `json:"uuid"`
	Handle   string `json:"handle"`
	APIKey   string `json:"api_key"`
	APIKeyID string `json:"api_key_id"`
}

// RegistryPage from GET /v1/registry.
type RegistryPage struct {
	Cards   []json.RawMessage `json:"cards"`
	Total   int               `json:"total"`
	Page    int               `json:"page"`
	PerPage int               `json:"per_page"`
	Pages   int               `json:"pages"`
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func (c *IDClient) doJSON(ctx context.Context, method, path string, body interface{}, headers map[string]string) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if json.Unmarshal(respBody, apiErr) != nil {
			apiErr.Code = "unknown"
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	return respBody, nil
}

// ---------------------------------------------------------------------------
// Public methods
// ---------------------------------------------------------------------------

// Lookup fetches a public agent profile by UUID or @@handle.
func (c *IDClient) Lookup(ctx context.Context, identifier string) (*AgentProfile, error) {
	path := "/v1/agents/" + url.PathEscape(identifier)
	data, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var p AgentProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode agent profile: %w", err)
	}
	return &p, nil
}

// SearchParams holds query parameters for the directory search.
type SearchParams struct {
	Q          string
	Capability string
	Skill      string
	Tag        string
	Level      string
	CertLevel  int
	MinTrust   int
	Sort       string
	Page       int
	PerPage    int
}

// Search queries the agent directory.
func (c *IDClient) Search(ctx context.Context, params SearchParams) (*DirectoryPage, error) {
	q := url.Values{}
	if params.Q != "" {
		q.Set("q", params.Q)
	}
	if params.Capability != "" {
		q.Set("capability", params.Capability)
	}
	if params.Skill != "" {
		q.Set("skill", params.Skill)
	}
	if params.Tag != "" {
		q.Set("tag", params.Tag)
	}
	if params.Level != "" {
		q.Set("level", params.Level)
	}
	if params.CertLevel > 0 {
		q.Set("cert_level", strconv.Itoa(params.CertLevel))
	}
	if params.MinTrust > 0 {
		q.Set("min_trust", strconv.Itoa(params.MinTrust))
	}
	if params.Sort != "" {
		q.Set("sort", params.Sort)
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	}

	path := "/v1/directory"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var page DirectoryPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("decode directory page: %w", err)
	}
	return &page, nil
}

// Verify calls POST /v1/verify with a JWT token.
func (c *IDClient) Verify(ctx context.Context, token, audience string) (*VerifyResult, error) {
	body := map[string]string{"token": token}
	if audience != "" {
		body["audience"] = audience
	}
	data, err := c.doJSON(ctx, http.MethodPost, "/v1/verify", body, nil)
	if err != nil {
		return nil, err
	}
	var result VerifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode verify result: %w", err)
	}
	return &result, nil
}

// Register calls POST /v1/agents/register.
func (c *IDClient) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	data, err := c.doJSON(ctx, http.MethodPost, "/v1/agents/register", req, nil)
	if err != nil {
		return nil, err
	}
	var resp RegisterResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	return &resp, nil
}

// IssueToken calls POST /v1/agents/token.
func (c *IDClient) IssueToken(ctx context.Context, req TokenRequest) (*TokenResponse, error) {
	data, err := c.doJSON(ctx, http.MethodPost, "/v1/agents/token", req, nil)
	if err != nil {
		return nil, err
	}
	var resp TokenResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &resp, nil
}

// UpdateProfile calls PUT /v1/agents/{uuid} with the agent's API key.
func (c *IDClient) UpdateProfile(ctx context.Context, agentUUID, apiKey string, fields ProfileUpdateFields) (*AgentProfile, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	data, err := c.doJSON(ctx, http.MethodPut, "/v1/agents/"+url.PathEscape(agentUUID), fields, headers)
	if err != nil {
		return nil, err
	}
	var p AgentProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decode updated profile: %w", err)
	}
	return &p, nil
}

// GetStats calls GET /v1/directory/stats.
func (c *IDClient) GetStats(ctx context.Context) (*DirectoryStats, error) {
	data, err := c.doJSON(ctx, http.MethodGet, "/v1/directory/stats", nil, nil)
	if err != nil {
		return nil, err
	}
	var stats DirectoryStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("decode directory stats: %w", err)
	}
	return &stats, nil
}

// ---------------------------------------------------------------------------
// Skills
// ---------------------------------------------------------------------------

// ListSkills calls GET /v1/agents/{uuid}/skills (public, no auth).
func (c *IDClient) ListSkills(ctx context.Context, agentUUID string) (*SkillsResponse, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/skills"
	data, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var resp SkillsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode skills response: %w", err)
	}
	return &resp, nil
}

// AddSkill calls POST /v1/agents/{uuid}/skills (requires agent_key).
func (c *IDClient) AddSkill(ctx context.Context, agentUUID, apiKey string, skill Skill) (*Skill, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/skills"
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodPost, path, skill, headers)
	if err != nil {
		return nil, err
	}
	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode skill: %w", err)
	}
	return &s, nil
}

// ReplaceSkills calls PUT /v1/agents/{uuid}/skills (requires agent_key).
func (c *IDClient) ReplaceSkills(ctx context.Context, agentUUID, apiKey string, skills []Skill) (*SkillsResponse, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/skills"
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	body := map[string]interface{}{"skills": skills}
	data, err := c.doJSON(ctx, http.MethodPut, path, body, headers)
	if err != nil {
		return nil, err
	}
	var resp SkillsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode skills response: %w", err)
	}
	return &resp, nil
}

// DeleteSkill calls DELETE /v1/agents/{uuid}/skills/{id} (requires agent_key).
func (c *IDClient) DeleteSkill(ctx context.Context, agentUUID, apiKey, skillID string) error {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/skills/" + url.PathEscape(skillID)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, headers)
	return err
}

// ---------------------------------------------------------------------------
// Custom Fields
// ---------------------------------------------------------------------------

// ListCustomFields calls GET /v1/agents/{uuid}/custom (requires agent_key).
func (c *IDClient) ListCustomFields(ctx context.Context, agentUUID, apiKey string) (*CustomFieldsResponse, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/custom"
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodGet, path, nil, headers)
	if err != nil {
		return nil, err
	}
	var resp CustomFieldsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode custom fields: %w", err)
	}
	return &resp, nil
}

// SetCustomField calls PUT /v1/agents/{uuid}/custom/{key} (requires agent_key).
func (c *IDClient) SetCustomField(ctx context.Context, agentUUID, apiKey, key, value string) error {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/custom/" + url.PathEscape(key)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodPut, path, map[string]string{"value": value}, headers)
	return err
}

// DeleteCustomField calls DELETE /v1/agents/{uuid}/custom/{key} (requires agent_key).
func (c *IDClient) DeleteCustomField(ctx context.Context, agentUUID, apiKey, key string) error {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/custom/" + url.PathEscape(key)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, headers)
	return err
}

// ListWellKnownFields calls GET /v1/custom-fields (public, no auth).
func (c *IDClient) ListWellKnownFields(ctx context.Context) (*WellKnownFieldsResponse, error) {
	data, err := c.doJSON(ctx, http.MethodGet, "/v1/custom-fields", nil, nil)
	if err != nil {
		return nil, err
	}
	var resp WellKnownFieldsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode well-known fields: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Private Metadata
// ---------------------------------------------------------------------------

// ListPrivateMetadata calls GET /v1/agents/{uuid}/private (requires agent_key).
func (c *IDClient) ListPrivateMetadata(ctx context.Context, agentUUID, apiKey string) (*MetadataListResponse, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/private"
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodGet, path, nil, headers)
	if err != nil {
		return nil, err
	}
	var resp MetadataListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode private metadata: %w", err)
	}
	return &resp, nil
}

// GetPrivateMetadata calls GET /v1/agents/{uuid}/private/{key} (requires agent_key).
func (c *IDClient) GetPrivateMetadata(ctx context.Context, agentUUID, apiKey, key string) (*MetadataEntry, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/private/" + url.PathEscape(key)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodGet, path, nil, headers)
	if err != nil {
		return nil, err
	}
	var entry MetadataEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("decode private metadata entry: %w", err)
	}
	return &entry, nil
}

// SetPrivateMetadata calls PUT /v1/agents/{uuid}/private/{key} (requires agent_key).
func (c *IDClient) SetPrivateMetadata(ctx context.Context, agentUUID, apiKey, key, value string) error {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/private/" + url.PathEscape(key)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodPut, path, map[string]string{"value": value}, headers)
	return err
}

// DeletePrivateMetadata calls DELETE /v1/agents/{uuid}/private/{key} (requires agent_key).
func (c *IDClient) DeletePrivateMetadata(ctx context.Context, agentUUID, apiKey, key string) error {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/private/" + url.PathEscape(key)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodDelete, path, nil, headers)
	return err
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

// CreateSession calls POST /v1/agents/session (requires iak_ key only).
func (c *IDClient) CreateSession(ctx context.Context, apiKey string) (*SessionResponse, error) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodPost, "/v1/agents/session", nil, headers)
	if err != nil {
		return nil, err
	}
	var resp SessionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode session response: %w", err)
	}
	return &resp, nil
}

// RevokeSession calls DELETE /v1/agents/session (requires iak_ or ias_ key).
func (c *IDClient) RevokeSession(ctx context.Context, apiKey string) error {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	_, err := c.doJSON(ctx, http.MethodDelete, "/v1/agents/session", nil, headers)
	return err
}

// ---------------------------------------------------------------------------
// Key Rotation
// ---------------------------------------------------------------------------

// RotateKey calls POST /v1/agents/{uuid}/keys/rotate (requires iak_ key only).
func (c *IDClient) RotateKey(ctx context.Context, agentUUID, apiKey string) (*RotateKeyResponse, error) {
	path := "/v1/agents/" + url.PathEscape(agentUUID) + "/keys/rotate"
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	data, err := c.doJSON(ctx, http.MethodPost, path, nil, headers)
	if err != nil {
		return nil, err
	}
	var resp RotateKeyResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode rotate key response: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Recovery
// ---------------------------------------------------------------------------

// Recover calls POST /v1/agents/recover (no auth — cert_token in body).
func (c *IDClient) Recover(ctx context.Context, req RecoverReq) error {
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/agents/recover", req, nil)
	return err
}

// RecoverConfirm calls POST /v1/agents/recover/confirm (no auth).
func (c *IDClient) RecoverConfirm(ctx context.Context, req RecoverConfirmReq) (*RecoverConfirmResponse, error) {
	data, err := c.doJSON(ctx, http.MethodPost, "/v1/agents/recover/confirm", req, nil)
	if err != nil {
		return nil, err
	}
	var resp RecoverConfirmResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode recover confirm response: %w", err)
	}
	return &resp, nil
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// RegistryParams holds query parameters for the registry search.
type RegistryParams struct {
	Q             string
	Skill         string
	Tag           string
	Level         string
	MinTrust      int
	HasContact    *bool
	WorldPresence *bool
	Sort          string
	Page          int
	PerPage       int
}

// GetRegistry calls GET /v1/registry (public, no auth).
func (c *IDClient) GetRegistry(ctx context.Context, params RegistryParams) (*RegistryPage, error) {
	q := url.Values{}
	if params.Q != "" {
		q.Set("q", params.Q)
	}
	if params.Skill != "" {
		q.Set("skill", params.Skill)
	}
	if params.Tag != "" {
		q.Set("tag", params.Tag)
	}
	if params.Level != "" {
		q.Set("level", params.Level)
	}
	if params.MinTrust > 0 {
		q.Set("min_trust", strconv.Itoa(params.MinTrust))
	}
	if params.HasContact != nil {
		q.Set("has_contact", strconv.FormatBool(*params.HasContact))
	}
	if params.WorldPresence != nil {
		q.Set("world_presence", strconv.FormatBool(*params.WorldPresence))
	}
	if params.Sort != "" {
		q.Set("sort", params.Sort)
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		q.Set("per_page", strconv.Itoa(params.PerPage))
	}

	path := "/v1/registry"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	var page RegistryPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("decode registry page: %w", err)
	}
	return &page, nil
}
