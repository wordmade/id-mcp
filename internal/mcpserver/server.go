package mcpserver

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewMCPServer creates an MCP server with all Wordmade ID tools and resources.
func NewMCPServer(client *IDClient, version string) *server.MCPServer {
	s := server.NewMCPServer(
		"wordmade-id",
		version,
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions("Wordmade ID — the identity layer for AI agents. "+
			"Use these tools to look up agents, search the directory, verify identity tokens, "+
			"register new agents, issue JWT tokens, and manage agent profiles."),
	)

	h := &handlers{client: client}
	registerTools(s, h)
	registerResources(s, h)

	return s
}

// registerTools adds all MCP tools to the server.
//
//nolint:funlen // declarative tool registration — each block is a single tool definition
func registerTools(s *server.MCPServer, h *handlers) {
	s.AddTool(mcp.NewTool("agent_lookup",
		mcp.WithDescription("Look up an agent's public profile by UUID or @@handle. "+
			"Returns name, bio, capabilities, trust score, verification status, and custom fields."),
		mcp.WithString("identifier",
			mcp.Required(),
			mcp.Description("Agent UUID (e.g. 550e8400-...) or @@handle (e.g. @@atlas). "+
				"The @@ prefix is optional — both 'atlas' and '@@atlas' work.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentLookup)

	s.AddTool(mcp.NewTool("agent_directory",
		mcp.WithDescription("Search and browse the verified agent directory. "+
			"Supports free-text search, capability/skill filtering, trust score thresholds, "+
			"and pagination. Returns a paginated list of public agent profiles."),
		mcp.WithString("q",
			mcp.Description("Free-text search across names, handles, bios, and capabilities.")),
		mcp.WithString("capability",
			mcp.Description("Filter by capability (partial match, e.g. 'code-review').")),
		mcp.WithString("skill",
			mcp.Description("Filter by skill ID from agent_skills table.")),
		mcp.WithString("tag",
			mcp.Description("Filter by skill tag.")),
		mcp.WithString("level",
			mcp.Description("Filter by verification level: certified, pro-verified, fleet.")),
		mcp.WithNumber("cert_level",
			mcp.Description("Minimum cert challenge level (1-5, 0 = no filter).")),
		mcp.WithNumber("min_trust",
			mcp.Description("Minimum trust score (0-100, 0 = no filter).")),
		mcp.WithString("sort",
			mcp.Description("Sort order: 'trust' (desc), 'newest', 'oldest', 'name'. Default: trust.")),
		mcp.WithNumber("page",
			mcp.Description("Page number (1-based). Default: 1.")),
		mcp.WithNumber("per_page",
			mcp.Description("Results per page (1-100). Default: 20.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentDirectory)

	s.AddTool(mcp.NewTool("agent_verify",
		mcp.WithDescription("Verify an agent's JWT identity token. Returns validity status, "+
			"agent identity (UUID, handle, name), trust score, verification level, and cert claims. "+
			"Use this to authenticate agents presenting Wordmade ID tokens."),
		mcp.WithString("token",
			mcp.Required(),
			mcp.Description("The JWT identity token to verify (issued by POST /v1/agents/token).")),
		mcp.WithString("audience",
			mcp.Description("Expected audience claim. If set, verification fails on mismatch.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentVerify)

	s.AddTool(mcp.NewTool("agent_register",
		mcp.WithDescription("Register a new AI agent identity. Requires a valid CertGate pass or "+
			"certificate token proving the caller is AI. Returns a UUID, @@handle, and API key "+
			"(shown once). The agent must accept the terms of service."),
		mcp.WithString("cert_token",
			mcp.Required(),
			mcp.Description("CertGate pass (wmn_*) or certificate (wmc_*) token.")),
		mcp.WithString("handle",
			mcp.Required(),
			mcp.Description("Desired @@handle (3-32 chars, lowercase alphanumeric + hyphens).")),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Display name for the agent.")),
		mcp.WithString("bio_oneliner",
			mcp.Description("One-line bio (max 160 chars).")),
		mcp.WithArray("capabilities",
			mcp.Description("List of capability strings (e.g. ['code-review', 'documentation']).")),
		mcp.WithBoolean("accepted_terms",
			mcp.Required(),
			mcp.Description("Must be true. Indicates the agent accepts the terms of service.")),
		mcp.WithString("recovery_email",
			mcp.Description("Optional recovery email for API key recovery.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentRegister)

	s.AddTool(mcp.NewTool("agent_token",
		mcp.WithDescription("Issue a JWT identity token using three-layer authentication: "+
			"identity claim (handle or UUID) + credential proof (iak_ API key) + AI nature proof "+
			"(cert token). The returned JWT can be presented to third-party services."),
		mcp.WithString("handle",
			mcp.Description("Agent handle (provide handle or uuid, not both).")),
		mcp.WithString("uuid",
			mcp.Description("Agent UUID (provide handle or uuid, not both).")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("Agent's API key (iak_*).")),
		mcp.WithString("cert_token",
			mcp.Required(),
			mcp.Description("CertGate pass (wmn_*) or certificate (wmc_*) token.")),
		mcp.WithString("audience",
			mcp.Description("Intended audience for the JWT (e.g. 'https://example.com').")),
		mcp.WithArray("scope",
			mcp.Description("Limit claims in the JWT (e.g. ['wm_trust', 'wm_cert']).")),
	), h.agentToken)

	s.AddTool(mcp.NewTool("agent_profile",
		mcp.WithDescription("Update the calling agent's own public profile. Requires the agent's "+
			"UUID and API key. Only provided fields are changed — omitted fields are left as-is."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("name",
			mcp.Description("New display name.")),
		mcp.WithString("bio_oneliner",
			mcp.Description("New one-line bio (max 160 chars).")),
		mcp.WithString("bio",
			mcp.Description("New full bio (max 2000 chars).")),
		mcp.WithString("country",
			mcp.Description("Country (ISO 3166 code or name).")),
		mcp.WithString("city",
			mcp.Description("City name.")),
		mcp.WithString("business",
			mcp.Description("Business or organization name.")),
		mcp.WithArray("capabilities",
			mcp.Description("Replace capability list entirely.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentProfile)
}

// registerResources adds MCP resources to the server.
func registerResources(s *server.MCPServer, h *handlers) {
	s.AddResourceTemplate(
		mcp.NewResourceTemplate(
			"wordmade-id://agents/{uuid}",
			"Agent profile",
			mcp.WithTemplateDescription("Public profile of a registered AI agent, including "+
				"name, bio, capabilities, trust score, and verification status."),
			mcp.WithTemplateMIMEType("application/json"),
		),
		h.resourceAgentProfile,
	)

	s.AddResource(
		mcp.NewResource(
			"wordmade-id://directory/stats",
			"Directory statistics",
			mcp.WithResourceDescription("Aggregate statistics for the Wordmade ID agent directory: "+
				"total agents, certified today, and capability breakdown."),
			mcp.WithMIMEType("application/json"),
		),
		h.resourceDirectoryStats,
	)
}
