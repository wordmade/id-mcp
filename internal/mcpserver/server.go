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
			"register new agents, issue JWT tokens, manage profiles, skills, custom fields, "+
			"private metadata, sessions, key rotation, recovery, and browse the A2A registry."),
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
			mcp.Description("Filter by verification level (always 'certified').")),
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

	// -----------------------------------------------------------------
	// Skills
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_skills",
		mcp.WithDescription("List all skills for an agent. Public — no authentication required."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentListSkills)

	s.AddTool(mcp.NewTool("agent_add_skill",
		mcp.WithDescription("Add a skill to the agent's profile. Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Skill ID (lowercase, hyphens, e.g. 'code-review').")),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Human-readable skill name.")),
		mcp.WithString("description",
			mcp.Description("What this skill does.")),
		mcp.WithArray("tags",
			mcp.Description("Tags for discoverability (e.g. ['go', 'python']).")),
		mcp.WithArray("examples",
			mcp.Description("Example prompts or use cases.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentAddSkill)

	s.AddTool(mcp.NewTool("agent_delete_skill",
		mcp.WithDescription("Remove a skill from the agent's profile. Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("skill_id",
			mcp.Required(),
			mcp.Description("ID of the skill to remove.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentDeleteSkill)

	// -----------------------------------------------------------------
	// Custom Fields
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_custom_fields",
		mcp.WithDescription("List all custom fields on the agent's public profile. "+
			"Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentListCustomFields)

	s.AddTool(mcp.NewTool("agent_set_custom_field",
		mcp.WithDescription("Set a custom field on the agent's public profile. "+
			"Use well-known keys (website, twitter, github, etc.) for best visibility. "+
			"Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Field key (1-64 chars, lowercase alphanumeric + hyphens/underscores).")),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Field value (max 512 chars).")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentSetCustomField)

	s.AddTool(mcp.NewTool("agent_delete_custom_field",
		mcp.WithDescription("Remove a custom field from the agent's public profile. "+
			"Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Key of the custom field to remove.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentDeleteCustomField)

	s.AddTool(mcp.NewTool("well_known_fields",
		mcp.WithDescription("List all recognized custom field keys with their categories "+
			"and rendering hints. Public — no authentication required. Use these keys "+
			"when setting custom fields for best discoverability."),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentWellKnownFields)

	// -----------------------------------------------------------------
	// Private Metadata
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_private_metadata",
		mcp.WithDescription("List all private metadata keys and values. Private metadata is "+
			"encrypted at rest (AES-256-GCM) and only accessible by the agent itself. "+
			"Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentListPrivateMetadata)

	s.AddTool(mcp.NewTool("agent_get_private",
		mcp.WithDescription("Get a single private metadata value. Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The metadata key to retrieve.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentGetPrivate)

	s.AddTool(mcp.NewTool("agent_set_private",
		mcp.WithDescription("Set a private metadata value. Stored encrypted at rest. "+
			"Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("Metadata key (1-64 chars, lowercase alphanumeric + hyphens/underscores).")),
		mcp.WithString("value",
			mcp.Required(),
			mcp.Description("Metadata value (max 4096 chars).")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentSetPrivate)

	s.AddTool(mcp.NewTool("agent_delete_private",
		mcp.WithDescription("Delete a private metadata key. Requires the agent's API key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithString("key",
			mcp.Required(),
			mcp.Description("The metadata key to delete.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentDeletePrivate)

	// -----------------------------------------------------------------
	// Sessions
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_create_session",
		mcp.WithDescription("Create a short-lived session token (ias_, 30 min TTL). "+
			"Requires an iak_ API key — session tokens cannot create sessions. "+
			"The returned ias_ token can be used for all agent operations."),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's permanent API key (iak_* only).")),
	), h.agentCreateSession)

	s.AddTool(mcp.NewTool("agent_revoke_session",
		mcp.WithDescription("Revoke the current session (logout). Accepts iak_ or ias_ keys."),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's API key (iak_* or ias_*).")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentRevokeSession)

	// -----------------------------------------------------------------
	// Key Rotation
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_rotate_key",
		mcp.WithDescription("Rotate the agent's API key. Generates a new iak_ key and revokes "+
			"ALL existing keys and sessions. The new key is shown ONCE — store it securely. "+
			"Requires the current iak_ key."),
		mcp.WithString("uuid",
			mcp.Required(),
			mcp.Description("The agent's UUID.")),
		mcp.WithString("api_key",
			mcp.Required(),
			mcp.Description("The agent's current API key (iak_* only).")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentRotateKey)

	// -----------------------------------------------------------------
	// Recovery
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_recover",
		mcp.WithDescription("Initiate API key recovery. Sends a recovery token to the agent's "+
			"registered recovery email. Requires a fresh CertGate pass as proof of AI nature."),
		mcp.WithString("cert_token",
			mcp.Required(),
			mcp.Description("CertGate pass (wmn_*) or certificate (wmc_*) token.")),
		mcp.WithString("handle",
			mcp.Description("Agent handle (provide handle or uuid).")),
		mcp.WithString("uuid",
			mcp.Description("Agent UUID (provide handle or uuid).")),
	), h.agentRecover)

	s.AddTool(mcp.NewTool("agent_recover_confirm",
		mcp.WithDescription("Complete API key recovery. Provide the recovery token from email "+
			"and a fresh CertGate pass. Returns new API key (shown ONCE — store it securely). "+
			"All previous keys are revoked."),
		mcp.WithString("recovery_token",
			mcp.Required(),
			mcp.Description("The recovery token received by email.")),
		mcp.WithString("cert_token",
			mcp.Required(),
			mcp.Description("Fresh CertGate pass (wmn_*) or certificate (wmc_*) token.")),
		mcp.WithDestructiveHintAnnotation(true),
	), h.agentRecoverConfirm)

	// -----------------------------------------------------------------
	// Registry
	// -----------------------------------------------------------------

	s.AddTool(mcp.NewTool("agent_registry",
		mcp.WithDescription("Browse the A2A agent card registry. Returns paginated agent cards "+
			"with skills, contact info, and world presences. Supports filtering and search."),
		mcp.WithString("q",
			mcp.Description("Free-text search across names, handles, bios.")),
		mcp.WithString("skill",
			mcp.Description("Filter by skill ID.")),
		mcp.WithString("tag",
			mcp.Description("Filter by skill tag.")),
		mcp.WithString("level",
			mcp.Description("Filter by verification level (always 'certified').")),
		mcp.WithNumber("min_trust",
			mcp.Description("Minimum trust score (0-100).")),
		mcp.WithString("sort",
			mcp.Description("Sort: 'trust_score' (default), 'joined', 'name'.")),
		mcp.WithNumber("page",
			mcp.Description("Page number (1-based). Default: 1.")),
		mcp.WithNumber("per_page",
			mcp.Description("Results per page (1-100). Default: 20.")),
		mcp.WithReadOnlyHintAnnotation(true),
	), h.agentRegistry)
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
