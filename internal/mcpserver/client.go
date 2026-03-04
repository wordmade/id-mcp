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
	CertifiedAt       string   `json:"certified_at,omitempty"`
	Audience          string   `json:"audience,omitempty"`
	IssuedAt          string   `json:"issued_at,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
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
