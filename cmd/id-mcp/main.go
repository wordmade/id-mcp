// id-mcp is the MCP (Model Context Protocol) server for Wordmade ID.
//
// It exposes agent identity operations — lookup, directory search, verification,
// registration, token issuance, and profile management — as MCP tools over stdio.
// All operations proxy through the ID REST API; the MCP server itself is stateless.
//
// Configuration:
//
//	WORDMADE_ID_API_URL  Base URL of the ID API (default: https://api.id.wordmade.world)
//
// Usage:
//
//	id-mcp              Start MCP server on stdio (JSON-RPC over stdin/stdout)
//	id-mcp --version    Print version and exit
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/wordmade/id-mcp/internal/mcpserver"
)

const defaultAPIURL = "https://api.id.wordmade.world"

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// Handle --version flag manually (no flag package needed for a single flag).
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("id-mcp %s\n", version)
		os.Exit(0)
	}

	apiURL := os.Getenv("WORDMADE_ID_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	client := mcpserver.NewIDClient(apiURL)
	mcpSrv := mcpserver.NewMCPServer(client, version)

	// Stderr logger for MCP transport errors (stdout is reserved for JSON-RPC).
	errLog := log.New(os.Stderr, "id-mcp: ", log.LstdFlags)

	if err := server.ServeStdio(mcpSrv, server.WithErrorLogger(errLog)); err != nil {
		errLog.Fatalf("stdio server error: %v", err)
	}
}
