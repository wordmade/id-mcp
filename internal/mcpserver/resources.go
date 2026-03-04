package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// wordmade-id://agents/{uuid} — agent profile resource
// ---------------------------------------------------------------------------

func (h *handlers) resourceAgentProfile(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI

	// Extract UUID from "wordmade-id://agents/{uuid}"
	agentUUID := strings.TrimPrefix(uri, "wordmade-id://agents/")
	if agentUUID == "" || agentUUID == uri {
		return nil, fmt.Errorf("invalid resource URI: %s", uri)
	}

	profile, err := h.client.Lookup(ctx, agentUUID)
	if err != nil {
		return nil, fmt.Errorf("lookup agent %s: %w", agentUUID, err)
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal agent profile: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

// ---------------------------------------------------------------------------
// wordmade-id://directory/stats — directory statistics resource
// ---------------------------------------------------------------------------

func (h *handlers) resourceDirectoryStats(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats, err := h.client.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get directory stats: %w", err)
	}

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal directory stats: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      req.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}
